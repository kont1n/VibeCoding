package extractor

import (
	"fmt"
	"hash/fnv"
	"image"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kont1n/face-grouper/internal/imageutil"
	"github.com/kont1n/face-grouper/internal/inference"
	"github.com/kont1n/face-grouper/internal/model"
)

// Config holds extractor settings.
type Config struct {
	ModelsDir      string
	Workers        int
	GPU            bool
	GPUDetSessions int
	GPURecSessions int
	EmbedBatchSize int
	EmbedFlushMS   int
	ThumbDir       string
	MaxDim         int
	DetThresh      float32
	NMSThresh      float32
}

// Result aggregates extraction output and error statistics.
type Result struct {
	Faces      []model.Face
	ErrorCount int
	FileErrors map[string]string
}

// Extract runs face detection and embedding extraction on all files using
// native Go ONNX inference (no Python dependency).
func Extract(files []string, cfg Config, w io.Writer) (*Result, error) {
	res := &Result{FileErrors: make(map[string]string)}

	detPath := filepath.Join(cfg.ModelsDir, "det_10g.onnx")
	recPath := filepath.Join(cfg.ModelsDir, "w600k_r50.onnx")

	if _, err := os.Stat(detPath); err != nil {
		return nil, fmt.Errorf("detection model not found at %s: %w", detPath, err)
	}
	if _, err := os.Stat(recPath); err != nil {
		return nil, fmt.Errorf("recognition model not found at %s: %w", recPath, err)
	}

	type fileResult struct {
		path  string
		faces []model.Face
		err   error
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	detSessions := workers
	recSessions := workers
	if cfg.GPU {
		detSessions = cfg.GPUDetSessions
		recSessions = cfg.GPURecSessions
		if detSessions <= 0 {
			detSessions = min(workers, 2)
			if detSessions <= 0 {
				detSessions = 1
			}
		}
		if recSessions <= 0 {
			recSessions = min(workers, 2)
			if recSessions <= 0 {
				recSessions = 1
			}
		}
	}

	detectors := make([]*inference.Detector, 0, detSessions)
	recognizers := make([]*inference.Recognizer, 0, recSessions)
	detPool := make(chan *inference.Detector, detSessions)
	recPool := make(chan *inference.Recognizer, recSessions)

	closeResources := func() {
		for _, d := range detectors {
			if d != nil {
				d.Close()
			}
		}
		for _, r := range recognizers {
			if r != nil {
				r.Close()
			}
		}
	}

	for i := 0; i < detSessions; i++ {
		det, err := inference.NewDetector(inference.DetectorConfig{
			ModelPath: detPath,
			GPU:       cfg.GPU,
			DetThresh: cfg.DetThresh,
			NMSThresh: cfg.NMSThresh,
		})
		if err != nil {
			closeResources()
			return nil, fmt.Errorf("init detector session %d/%d: %w", i+1, detSessions, err)
		}
		detectors = append(detectors, det)
		detPool <- det
	}

	var recSize int
	for i := 0; i < recSessions; i++ {
		rec, err := inference.NewRecognizer(inference.RecognizerConfig{
			ModelPath: recPath,
			GPU:       cfg.GPU,
		})
		if err != nil {
			closeResources()
			return nil, fmt.Errorf("init recognizer session %d/%d: %w", i+1, recSessions, err)
		}
		if recSize == 0 {
			recSize = rec.InputSize()
		}
		recognizers = append(recognizers, rec)
		recPool <- rec
	}
	defer closeResources()

	embedBatchSize := cfg.EmbedBatchSize
	if embedBatchSize <= 0 {
		embedBatchSize = 64
	}
	embedFlush := time.Duration(cfg.EmbedFlushMS) * time.Millisecond
	if embedFlush <= 0 {
		embedFlush = 10 * time.Millisecond
	}
	recBatcher := newRecognizerBatcher(recPool, recSessions, embedBatchSize, embedFlush)
	defer recBatcher.Close()

	jobs := make(chan string, len(files))
	results := make(chan fileResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				faces, err := processImage(path, detPool, recBatcher, recSize, cfg)
				results <- fileResult{path: path, faces: faces, err: err}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	processed := 0
	total := len(files)
	for r := range results {
		processed++
		if r.err != nil {
			fmt.Fprintf(w, "[%d/%d] ERROR %s: %v\n", processed, total, r.path, r.err)
			res.FileErrors[r.path] = r.err.Error()
			res.ErrorCount++
			continue
		}
		fmt.Fprintf(w, "[%d/%d] %s — found %d face(s)\n", processed, total, r.path, len(r.faces))
		res.Faces = append(res.Faces, r.faces...)
	}

	return res, nil
}

// processImage loads an image, detects faces, aligns them, extracts embeddings,
// and optionally saves thumbnails.
func processImage(
	imagePath string,
	detPool chan *inference.Detector,
	recBatcher *recognizerBatcher,
	recSize int,
	cfg Config,
) ([]model.Face, error) {
	img, err := imageutil.LoadImage(imagePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read image: %s: %w", imagePath, err)
	}
	if img.Empty() {
		return nil, fmt.Errorf("empty image: %s", imagePath)
	}
	defer img.Close()

	if cfg.MaxDim > 0 {
		maxSide := img.Height
		if img.Width > maxSide {
			maxSide = img.Width
		}
		if maxSide > cfg.MaxDim {
			scale := float64(cfg.MaxDim) / float64(maxSide)
			newW := int(float64(img.Width) * scale)
			newH := int(float64(img.Height) * scale)
			resized := imageutil.Resize(img, newW, newH)
			img.Close()
			img = resized
		}
	}

	dets, err := detectWithPool(detPool, img)
	if err != nil {
		return nil, fmt.Errorf("detection: %w", err)
	}
	if len(dets) == 0 {
		return nil, nil
	}

	// Align faces
	aligned := make([]*imageutil.Image, len(dets))
	for i, d := range dets {
		aligned[i] = inference.NormCrop(img, d.Kps, recSize)
	}
	defer func() {
		for _, a := range aligned {
			if a != nil {
				a.Close()
			}
		}
	}()

	embeddings, err := recBatcher.Infer(aligned)
	if err != nil {
		return nil, fmt.Errorf("recognition: %w", err)
	}

	faces := make([]model.Face, len(dets))
	for i, d := range dets {
		thumb := ""
		if cfg.ThumbDir != "" {
			thumb = saveThumbnail(img, d, imagePath, i, cfg.ThumbDir)
		}

		var keypoints [5][2]float64
		for k := 0; k < 5; k++ {
			keypoints[k][0] = float64(d.Kps[k][0])
			keypoints[k][1] = float64(d.Kps[k][1])
		}

		faces[i] = model.Face{
			BBox:      [4]float64{float64(d.X1), float64(d.Y1), float64(d.X2), float64(d.Y2)},
			Keypoints: keypoints,
			Embedding: embeddings[i],
			DetScore:  float64(d.Score),
			Thumbnail: thumb,
			FilePath:  imagePath,
		}
	}

	return faces, nil
}

// saveThumbnail crops a face region with 25% padding, resizes to 160x160,
// and saves as JPEG.
func saveThumbnail(img *imageutil.Image, det inference.Detection, imagePath string, faceIdx int, thumbDir string) string {
	h := img.Height
	w := img.Width

	x1 := int(det.X1)
	y1 := int(det.Y1)
	x2 := int(det.X2)
	y2 := int(det.Y2)

	padX := int(float64(x2-x1) * 0.25)
	padY := int(float64(y2-y1) * 0.25)

	cx1 := max(0, x1-padX)
	cy1 := max(0, y1-padY)
	cx2 := min(w, x2+padX)
	cy2 := min(h, y2+padY)

	if cx2 <= cx1 || cy2 <= cy1 {
		return ""
	}

	crop := img.Region(image.Rect(cx1, cy1, cx2, cy2))
	if crop == nil {
		return ""
	}
	defer crop.Close()

	resized := imageutil.Resize(crop, 160, 160)
	defer resized.Close()

	base := filepath.Base(imagePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	thumbName := fmt.Sprintf("%s_%s_face_%d.jpg", name, shortPathHash(imagePath), faceIdx)
	thumbPath := filepath.Join(thumbDir, thumbName)

	if err := imageutil.SaveJPEG(resized, thumbPath, 90); err != nil {
		return ""
	}
	return thumbPath
}

func detectWithPool(detPool chan *inference.Detector, img *imageutil.Image) ([]inference.Detection, error) {
	det := <-detPool
	defer func() { detPool <- det }()
	return det.Detect(img)
}

type recognizeRequest struct {
	done       chan struct{}
	embeddings [][]float64
	remaining  int
	err        error
	mu         sync.Mutex
}

func (r *recognizeRequest) resolve(idx int, embedding []float64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err != nil && r.err == nil {
		r.err = err
	}
	if err == nil {
		r.embeddings[idx] = embedding
	}
	r.remaining--
	if r.remaining == 0 {
		close(r.done)
	}
}

type recognizeItem struct {
	img *imageutil.Image
	req *recognizeRequest
	idx int
}

type recognizerBatcher struct {
	items        chan recognizeItem
	recPool      chan *inference.Recognizer
	batchSize    int
	flushTimeout time.Duration
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func newRecognizerBatcher(recPool chan *inference.Recognizer, workers, batchSize int, flushTimeout time.Duration) *recognizerBatcher {
	if workers <= 0 {
		workers = 1
	}
	if batchSize <= 0 {
		batchSize = 64
	}
	if flushTimeout <= 0 {
		flushTimeout = 10 * time.Millisecond
	}
	b := &recognizerBatcher{
		items:        make(chan recognizeItem, batchSize*workers*2),
		recPool:      recPool,
		batchSize:    batchSize,
		flushTimeout: flushTimeout,
	}
	for i := 0; i < workers; i++ {
		b.wg.Add(1)
		go b.runWorker()
	}
	return b
}

func (b *recognizerBatcher) Infer(imgs []*imageutil.Image) ([][]float64, error) {
	if len(imgs) == 0 {
		return nil, nil
	}
	req := &recognizeRequest{
		done:       make(chan struct{}),
		embeddings: make([][]float64, len(imgs)),
		remaining:  len(imgs),
	}
	for i, img := range imgs {
		b.items <- recognizeItem{img: img, req: req, idx: i}
	}
	<-req.done
	if req.err != nil {
		return nil, req.err
	}
	return req.embeddings, nil
}

func (b *recognizerBatcher) Close() {
	b.closeOnce.Do(func() {
		close(b.items)
		b.wg.Wait()
	})
}

func (b *recognizerBatcher) runWorker() {
	defer b.wg.Done()

	for {
		first, ok := <-b.items
		if !ok {
			return
		}

		batch := []recognizeItem{first}
		deadline := time.After(b.flushTimeout)
		channelClosed := false

	collect:
		for len(batch) < b.batchSize {
			select {
			case item, ok := <-b.items:
				if !ok {
					channelClosed = true
					break collect
				}
				batch = append(batch, item)
			case <-deadline:
				break collect
			}
		}

		imgs := make([]*imageutil.Image, len(batch))
		for i, item := range batch {
			imgs[i] = item.img
		}

		rec, ok := <-b.recPool
		if !ok {
			for _, item := range batch {
				item.req.resolve(item.idx, nil, fmt.Errorf("recognizer pool closed"))
			}
			if channelClosed {
				return
			}
			continue
		}

		embeddings, err := rec.GetEmbeddings(imgs)
		b.recPool <- rec

		if err != nil {
			for _, item := range batch {
				item.req.resolve(item.idx, nil, err)
			}
		} else {
			for i, item := range batch {
				if i >= len(embeddings) {
					item.req.resolve(item.idx, nil, fmt.Errorf("recognizer returned %d embedding(s) for batch of %d", len(embeddings), len(batch)))
					continue
				}
				item.req.resolve(item.idx, embeddings[i], nil)
			}
		}

		if channelClosed {
			return
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func shortPathHash(path string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("%016x", h.Sum64())[:10]
}
