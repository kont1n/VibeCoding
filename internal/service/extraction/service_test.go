package extraction

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/imageutil"
)

func TestThumbnailPath_Deterministic(t *testing.T) {
	t.Parallel()

	svc := &extractionService{}
	thumbDir := t.TempDir()
	imagePath := "/tmp/Album/IMG_001.jpeg"

	p1 := svc.thumbnailPath(imagePath, 3, thumbDir)
	p2 := svc.thumbnailPath(imagePath, 3, thumbDir)

	if p1 != p2 {
		t.Fatalf("thumbnail path must be deterministic: %q != %q", p1, p2)
	}
	if filepath.Dir(p1) != thumbDir {
		t.Fatalf("thumbnail path dir mismatch: got %q, want %q", filepath.Dir(p1), thumbDir)
	}
	if filepath.Ext(p1) != ".jpg" {
		t.Fatalf("thumbnail path extension mismatch: got %q", filepath.Ext(p1))
	}
}

func TestPrepareThumbnailImage_ReturnsResizedImage(t *testing.T) {
	t.Parallel()

	svc := &extractionService{}
	img := imageutil.NewImage(800, 600)
	defer img.Close()

	det := ml.Detection{
		X1: 100, Y1: 120,
		X2: 300, Y2: 360,
	}

	thumb := svc.prepareThumbnailImage(img, det)
	if thumb == nil {
		t.Fatal("expected thumbnail image, got nil")
	}
	defer thumb.Close()

	if thumb.Width != 160 || thumb.Height != 160 {
		t.Fatalf("unexpected thumbnail size: got %dx%d, want 160x160", thumb.Width, thumb.Height)
	}
}

func TestThumbnailWriter_WritesFiles(t *testing.T) {
	t.Parallel()

	writer := newThumbnailWriter(2)

	outputDir := t.TempDir()
	total := 4
	for i := 0; i < total; i++ {
		img := imageutil.NewImage(160, 160)
		path := filepath.Join(outputDir, fmt.Sprintf("thumb-%d.jpg", i))
		writer.Submit(img, path)
	}

	writer.Close()

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != total {
		t.Fatalf("unexpected number of thumbnails: got %d, want %d", len(entries), total)
	}
}

func BenchmarkPrepareThumbnailImage(b *testing.B) {
	svc := &extractionService{}
	img := imageutil.NewImage(1920, 1080)
	defer img.Close()
	det := ml.Detection{
		X1: 420, Y1: 180,
		X2: 1120, Y2: 980,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		thumb := svc.prepareThumbnailImage(img, det)
		if thumb == nil {
			b.Fatal("prepareThumbnailImage returned nil")
		}
		thumb.Close()
	}
}

func BenchmarkThumbnailWriter_SubmitAndFlush(b *testing.B) {
	outputDir := b.TempDir()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		writer := newThumbnailWriter(4)
		for j := 0; j < 32; j++ {
			img := imageutil.NewImage(160, 160)
			path := filepath.Join(outputDir, "bench-thumb.jpg")
			writer.Submit(img, path)
		}
		writer.Close()
	}
}

func BenchmarkExtractionThroughputSynthetic(b *testing.B) {
	const (
		totalFiles = 682
		workers    = 10
	)

	faceSample := model.Face{
		DetScore: 0.9,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		start := time.Now()

		res := &ExtractionResult{
			FileErrors: make(map[string]string),
		}
		var (
			mu        sync.Mutex
			processed atomic.Int64
		)

		sem := make(chan struct{}, workers)
		g := errgroup.Group{}

		for fileIdx := 0; fileIdx < totalFiles; fileIdx++ {
			idx := fileIdx
			g.Go(func() error {
				sem <- struct{}{}
				defer func() { <-sem }()

				current := int(processed.Add(1))

				// Emulate periodic progress/log check from the hot path.
				if current%10 == 0 || current == totalFiles {
					_ = current
				}

				// Emulate a small error rate.
				if idx%37 == 0 {
					mu.Lock()
					res.FileErrors[fmt.Sprintf("file-%d.jpg", idx)] = "synthetic error"
					res.ErrorCount++
					mu.Unlock()
					return nil
				}

				mu.Lock()
				res.Faces = append(res.Faces, faceSample)
				mu.Unlock()
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			b.Fatalf("unexpected errgroup error: %v", err)
		}

		elapsed := time.Since(start)
		b.ReportMetric(float64(totalFiles)/elapsed.Seconds(), "files/s")
	}
}
