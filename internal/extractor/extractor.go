package extractor

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sync"

	"gocv.io/x/gocv"

	"github.com/kont1n/face-grouper/internal/inference"
	"github.com/kont1n/face-grouper/internal/models"
)

// Config holds extractor settings.
type Config struct {
	ModelsDir string
	Workers   int
	GPU       bool
	ThumbDir  string
	MaxDim    int
	DetThresh float32
	NMSThresh float32
}

// Result aggregates extraction output and error statistics.
type Result struct {
	Faces      []models.Face
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
		faces []models.Face
		err   error
	}
	type workerRuntime struct {
		det     *inference.Detector
		rec     *inference.Recognizer
		recSize int
		inferMu *sync.Mutex
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	runtimes := make([]workerRuntime, workers)

	if cfg.GPU {
		det, err := inference.NewDetector(inference.DetectorConfig{
			ModelPath: detPath,
			GPU:       true,
			DetThresh: cfg.DetThresh,
			NMSThresh: cfg.NMSThresh,
		})
		if err != nil {
			return nil, fmt.Errorf("init shared detector: %w", err)
		}

		rec, err := inference.NewRecognizer(inference.RecognizerConfig{
			ModelPath: recPath,
			GPU:       true,
		})
		if err != nil {
			det.Close()
			return nil, fmt.Errorf("init shared recognizer: %w", err)
		}

		recSize := rec.InputSize()
		inferMu := &sync.Mutex{}
		for i := range runtimes {
			runtimes[i] = workerRuntime{
				det:     det,
				rec:     rec,
				recSize: recSize,
				inferMu: inferMu,
			}
		}
		defer det.Close()
		defer rec.Close()
	} else {
		closeInitialized := func(count int) {
			for i := 0; i < count; i++ {
				if runtimes[i].det != nil {
					runtimes[i].det.Close()
				}
				if runtimes[i].rec != nil {
					runtimes[i].rec.Close()
				}
			}
		}

		for i := range runtimes {
			det, err := inference.NewDetector(inference.DetectorConfig{
				ModelPath: detPath,
				GPU:       false,
				DetThresh: cfg.DetThresh,
				NMSThresh: cfg.NMSThresh,
			})
			if err != nil {
				closeInitialized(i)
				return nil, fmt.Errorf("init detector for worker %d: %w", i+1, err)
			}

			rec, err := inference.NewRecognizer(inference.RecognizerConfig{
				ModelPath: recPath,
				GPU:       false,
			})
			if err != nil {
				det.Close()
				closeInitialized(i)
				return nil, fmt.Errorf("init recognizer for worker %d: %w", i+1, err)
			}

			runtimes[i] = workerRuntime{
				det:     det,
				rec:     rec,
				recSize: rec.InputSize(),
			}
		}
		defer func() {
			for _, rt := range runtimes {
				if rt.det != nil {
					rt.det.Close()
				}
				if rt.rec != nil {
					rt.rec.Close()
				}
			}
		}()
	}

	jobs := make(chan string, len(files))
	results := make(chan fileResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		rt := runtimes[i]
		wg.Add(1)
		go func(rt workerRuntime) {
			defer wg.Done()
			for path := range jobs {
				faces, err := processImage(path, rt.det, rt.rec, rt.recSize, cfg, rt.inferMu)
				results <- fileResult{path: path, faces: faces, err: err}
			}
		}(rt)
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
	det *inference.Detector,
	rec *inference.Recognizer,
	recSize int,
	cfg Config,
	inferMu *sync.Mutex,
) ([]models.Face, error) {
	img := gocv.IMRead(imagePath, gocv.IMReadColor)
	if img.Empty() {
		return nil, fmt.Errorf("cannot read image: %s", imagePath)
	}

	if cfg.MaxDim > 0 {
		h := img.Rows()
		w := img.Cols()
		maxSide := h
		if w > maxSide {
			maxSide = w
		}
		if maxSide > cfg.MaxDim {
			scale := float64(cfg.MaxDim) / float64(maxSide)
			newW := int(float64(w) * scale)
			newH := int(float64(h) * scale)
			resized := gocv.NewMat()
			gocv.Resize(img, &resized, image.Point{X: newW, Y: newH}, 0, 0, gocv.InterpolationArea)
			img.Close()
			img = resized
		}
	}
	defer img.Close()

	dets, err := detectWithLock(det, img, inferMu)
	if err != nil {
		return nil, fmt.Errorf("detection: %w", err)
	}
	if len(dets) == 0 {
		return nil, nil
	}

	aligned := make([]gocv.Mat, len(dets))
	for i, d := range dets {
		aligned[i] = inference.NormCrop(img, d.Kps, recSize)
	}
	defer func() {
		for _, a := range aligned {
			a.Close()
		}
	}()

	embeddings, err := recognizeWithLock(rec, aligned, inferMu)
	if err != nil {
		return nil, fmt.Errorf("recognition: %w", err)
	}

	faces := make([]models.Face, len(dets))
	for i, d := range dets {
		thumb := ""
		if cfg.ThumbDir != "" {
			thumb = saveThumbnail(img, d, imagePath, i, cfg.ThumbDir)
		}

		faces[i] = models.Face{
			BBox:      [4]float64{float64(d.X1), float64(d.Y1), float64(d.X2), float64(d.Y2)},
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
func saveThumbnail(img gocv.Mat, det inference.Detection, imagePath string, faceIdx int, thumbDir string) string {
	h := img.Rows()
	w := img.Cols()

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
	defer crop.Close()

	resized := gocv.NewMat()
	defer resized.Close()
	gocv.Resize(crop, &resized, image.Point{X: 160, Y: 160}, 0, 0, gocv.InterpolationLinear)

	base := filepath.Base(imagePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	thumbName := fmt.Sprintf("%s_face_%d.jpg", name, faceIdx)
	thumbPath := filepath.Join(thumbDir, thumbName)

	if ok := gocv.IMWriteWithParams(thumbPath, resized, []int{gocv.IMWriteJpegQuality, 90}); !ok {
		return ""
	}
	return thumbPath
}

func detectWithLock(det *inference.Detector, img gocv.Mat, inferMu *sync.Mutex) ([]inference.Detection, error) {
	if inferMu != nil {
		inferMu.Lock()
		defer inferMu.Unlock()
	}
	return det.Detect(img)
}

func recognizeWithLock(rec *inference.Recognizer, faces []gocv.Mat, inferMu *sync.Mutex) ([][]float64, error) {
	if inferMu != nil {
		inferMu.Lock()
		defer inferMu.Unlock()
	}
	return rec.GetEmbeddings(faces)
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
