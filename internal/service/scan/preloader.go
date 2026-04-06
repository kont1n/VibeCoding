// Package scan provides parallel image preloading for efficient GPU utilization.
package scan

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/service/imageutil"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// PreloadedImage represents an image loaded from disk with its original path.
type PreloadedImage struct {
	Path string
	Img  *imageutil.Image
	Err  error
}

// ImagePreloader preloads images in parallel to keep GPU fed with data.
// While GPU processes batch N, CPU loads batch N+1 from disk.
type ImagePreloader struct {
	workers int
	queue   chan string         // Input: paths to preload.
	output  chan PreloadedImage // Output: loaded images.
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewImagePreloader creates a new parallel image preloader.
// workers: number of parallel preload workers (recommend: 4-6 for fast SSD).
// bufferSize: queue buffer size (recommend: workers*4).
func NewImagePreloader(workers, bufferSize int) *ImagePreloader {
	return NewImagePreloaderWithContext(context.Background(), workers, bufferSize)
}

// NewImagePreloaderWithContext creates a new parallel image preloader with context.
// workers: number of parallel preload workers (recommend: 4-6 for fast SSD).
// bufferSize: queue buffer size (recommend: workers*4).
func NewImagePreloaderWithContext(ctx context.Context, workers, bufferSize int) *ImagePreloader {
	if workers <= 0 {
		workers = 4
	}
	if bufferSize <= 0 {
		bufferSize = workers * 4
	}

	ctx, cancel := context.WithCancel(ctx)

	p := &ImagePreloader{
		workers: workers,
		queue:   make(chan string, bufferSize),
		output:  make(chan PreloadedImage, bufferSize),
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start worker pool.
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	logger.Info(ctx, "image preloader started",
		zap.Int("workers", workers),
		zap.Int("buffer", bufferSize),
	)

	return p
}

// Submit adds an image path to the preload queue.
// Returns false if the preloader has been stopped.
func (p *ImagePreloader) Submit(path string) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.queue <- path:
		return true
	}
}

// Output returns the channel of preloaded images.
func (p *ImagePreloader) Output() <-chan PreloadedImage {
	return p.output
}

// InputQueue returns the input channel for submitting paths (for advanced use).
func (p *ImagePreloader) InputQueue() chan<- string {
	return p.queue
}

// Close stops the preloader and waits for all workers to finish.
func (p *ImagePreloader) Close() {
	p.cancel()
	close(p.queue)
	p.wg.Wait()
	close(p.output)
}

// worker processes preload requests.
func (p *ImagePreloader) worker() {
	defer p.wg.Done()

	for path := range p.queue {
		// Check context cancellation.
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		// Load image with smart resize.
		img, err := imageutil.LoadImage(path)
		if err == nil && img != nil {
			// Apply smart resize immediately after loading.
			resized := imageutil.SmartResize(img)
			if resized != img {
				img.Close()
				img = resized
			}
		}

		result := PreloadedImage{
			Path: path,
			Img:  img,
			Err:  err,
		}

		select {
		case <-p.ctx.Done():
			if img != nil {
				img.Close()
			}
			return
		case p.output <- result:
		}
	}
}

// PreloadAll preloads all images from the given paths and returns them as a slice.
// This is a convenience function for smaller datasets.
func PreloadAll(ctx context.Context, paths []string, workers int) []PreloadedImage {
	if workers <= 0 {
		workers = 4
	}

	preloader := NewImagePreloaderWithContext(ctx, workers, len(paths))
	defer preloader.Close()

	// Submit all paths.
	for _, path := range paths {
		if !preloader.Submit(path) {
			break
		}
	}

	// Collect results.
	results := make([]PreloadedImage, 0, len(paths))
	for i := 0; i < len(paths); i++ {
		select {
		case <-ctx.Done():
			return results
		case result := <-preloader.Output():
			results = append(results, result)
		}
	}

	return results
}
