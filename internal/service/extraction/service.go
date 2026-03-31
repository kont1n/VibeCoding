// Package extraction provides face detection and embedding extraction services.
package extraction

import (
	"context"
	"fmt"
	"hash/fnv"
	"image"
	"io"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/kont1n/face-grouper/internal/config/env"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/imageutil"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// ExtractionResult aggregates face extraction results.
type ExtractionResult struct {
	Faces      []model.Face
	Clusters   []model.Cluster
	ErrorCount int
	FileErrors map[string]string
}

// ExtractionService определяет интерфейс сервиса экстракции.
type ExtractionService interface {
	Extract(ctx context.Context, files []string, thumbDir string, w io.Writer) (*ExtractionResult, error)
}

type extractionService struct {
	cfg          env.ExtractConfig
	detectorPool []ml.DetectorGateway
	recPool      []ml.RecognizerGateway
}

// NewExtractionService создаёт новый экземпляр сервиса экстракции.
func NewExtractionService(
	cfg env.ExtractConfig,
	detectorPool []ml.DetectorGateway,
	recPool []ml.RecognizerGateway,
) ExtractionService {
	return &extractionService{
		cfg:          cfg,
		detectorPool: detectorPool,
		recPool:      recPool,
	}
}

// Extract performs face detection and embedding extraction.
func (s *extractionService) Extract(ctx context.Context, files []string, thumbDir string, w io.Writer) (*ExtractionResult, error) {
	res := &ExtractionResult{FileErrors: make(map[string]string)}

	// If models/ONNX Runtime were not initialized (e.g., in -view mode without ORT),
	// this would cause a deadlock waiting on pools.
	if len(s.detectorPool) == 0 || len(s.recPool) == 0 {
		return nil, fmt.Errorf(
			"extraction runtime not initialized: detectors=%d recognizers=%d",
			len(s.detectorPool),
			len(s.recPool),
		)
	}

	logger.Info(ctx, "starting face extraction",
		zap.Int("files", len(files)),
		zap.Bool("gpu", s.cfg.GPU),
		zap.Int("workers", s.cfg.Workers),
	)

	workers := s.cfg.Workers
	if workers <= 0 {
		workers = 1
	}

	// Create detector pool.
	detPool := make(chan ml.DetectorGateway, len(s.detectorPool))
	for _, det := range s.detectorPool {
		detPool <- det
	}

	// Create recognizer pool.
	recPool := make(chan ml.RecognizerGateway, len(s.recPool))
	for _, rec := range s.recPool {
		recPool <- rec
	}

	// Get recognizer model input size.
	var recSize int
	if len(s.recPool) > 0 {
		recSize = s.recPool[0].InputSize()
	}

	// Batcher for recognition.
	embedBatchSize := s.cfg.EmbedBatchSize
	if embedBatchSize <= 0 {
		embedBatchSize = 64
	}
	embedFlush := time.Duration(s.cfg.EmbedFlushMS) * time.Millisecond
	if embedFlush <= 0 {
		embedFlush = 10 * time.Millisecond
	}
	recBatcher := newRecognizerBatcher(recPool, workers, embedBatchSize, embedFlush)
	defer recBatcher.Close()

	// Semaphore for concurrency limiting.
	sem := make(chan struct{}, workers)

	// Use errgroup for error handling and cancellation.
	g, ctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	processed := 0
	total := len(files)

	for _, f := range files {
		g.Go(func() error {
			// Acquire semaphore.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			defer func() { <-sem }()

			faces, err := s.processImage(ctx, f, detPool, recBatcher, recSize, thumbDir)

			mu.Lock()
			processed++
			if err != nil {
				_, _ = fmt.Fprintf(w, "[%d/%d] ERROR %s: %v\n", processed, total, f, err)
				logger.Error(ctx, "file processing error",
					zap.String("path", f),
					zap.Error(err),
				)
				res.FileErrors[f] = err.Error()
				res.ErrorCount++
			} else {
				_, _ = fmt.Fprintf(w, "[%d/%d] %s — found %d face(s)\n", processed, total, f, len(faces))
				res.Faces = append(res.Faces, faces...)
			}
			mu.Unlock()

			return nil // Don't stop processing on single file error.
		})
	}

	// Wait for all goroutines to complete.
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("extraction: %w", err)
	}

	logger.Info(ctx, "extraction completed",
		zap.Int("total_faces", len(res.Faces)),
		zap.Int("errors", res.ErrorCount),
	)

	return res, nil
}

func (s *extractionService) processImage(
	ctx context.Context,
	imagePath string,
	detPool chan ml.DetectorGateway,
	recBatcher *recognizerBatcher,
	recSize int,
	thumbDir string,
) ([]model.Face, error) {
	// Check for cancellation before expensive I/O.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	img, err := imageutil.LoadImage(imagePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read image: %s: %w", imagePath, err)
	}
	defer img.Close()

	if img.Empty() {
		return nil, fmt.Errorf("empty image: %s", imagePath)
	}

	// Optional downscale.
	if s.cfg.MaxDim > 0 {
		maxSide := img.Height
		if img.Width > maxSide {
			maxSide = img.Width
		}
		if maxSide > s.cfg.MaxDim {
			scale := float64(s.cfg.MaxDim) / float64(maxSide)
			newW := int(float64(img.Width) * scale)
			newH := int(float64(img.Height) * scale)
			resized := imageutil.Resize(img, newW, newH)
			img.Close()
			img = resized
		}
	}

	// Check for cancellation before detection.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	// Detection.
	det := <-detPool
	defer func() { detPool <- det }()

	dets, err := det.Detect(img)
	if err != nil {
		return nil, fmt.Errorf("detection: %w", err)
	}
	if len(dets) == 0 {
		return nil, nil
	}

	// Face alignment.
	aligned := make([]*imageutil.Image, len(dets))
	for i, d := range dets {
		aligned[i] = ml.NormCrop(img, d.Kps, recSize)
	}
	defer func() {
		for _, a := range aligned {
			if a != nil {
				a.Close()
			}
		}
	}()

	// Recognition.
	embeddings, err := recBatcher.Infer(aligned)
	if err != nil {
		return nil, fmt.Errorf("recognition: %w", err)
	}

	// Build results.
	faces := make([]model.Face, len(dets))
	for i, d := range dets {
		thumb := ""
		if thumbDir != "" {
			thumb = s.saveThumbnail(img, d, imagePath, i, thumbDir)
		}

		var keypoints [5][2]float64
		for k := 0; k < 5; k++ {
			keypoints[k][0] = float64(d.Kps[k][0])
			keypoints[k][1] = float64(d.Kps[k][1])
		}

		faces[i] = model.Face{
			BBox: model.BBox{
				X1: float32(d.X1),
				Y1: float32(d.Y1),
				X2: float32(d.X2),
				Y2: float32(d.Y2),
			},
			Keypoints:     keypoints,
			Embedding:     embeddings[i],
			DetScore:      float32(d.Score),
			Thumbnail:     thumb,
			FilePath:      imagePath,
			ThumbnailPath: thumb,
		}
	}

	return faces, nil
}

func (s *extractionService) saveThumbnail(img *imageutil.Image, det ml.Detection, imagePath string, faceIdx int, thumbDir string) string {
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

func shortPathHash(path string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	return fmt.Sprintf("%016x", h.Sum64())[:10]
}

// recognizerBatcher — batcher для распознавания лиц.
type recognizeRequest struct {
	done       chan struct{}
	embeddings [][]float32
	remaining  int
	err        error
	mu         sync.Mutex
}

func (r *recognizeRequest) resolve(idx int, embedding []float32, err error) {
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
	recPool      chan ml.RecognizerGateway
	batchSize    int
	flushTimeout time.Duration
	closeOnce    sync.Once
	closed       atomic.Bool
	wg           sync.WaitGroup
}

func newRecognizerBatcher(recPool chan ml.RecognizerGateway, workers, batchSize int, flushTimeout time.Duration) *recognizerBatcher {
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

func (b *recognizerBatcher) Infer(imgs []*imageutil.Image) ([][]float32, error) {
	if len(imgs) == 0 {
		return nil, nil
	}
	if b.closed.Load() {
		return nil, fmt.Errorf("recognizer batcher is closed")
	}
	req := &recognizeRequest{
		done:       make(chan struct{}),
		embeddings: make([][]float32, len(imgs)),
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
		b.closed.Store(true)
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
			case item, recvOK := <-b.items:
				if !recvOK {
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
