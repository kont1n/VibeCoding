package app

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/config/env"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/extraction"
)

type testScanService struct {
	files []string
}

func (s *testScanService) Scan(_ context.Context, _ string) ([]string, error) {
	return s.files, nil
}

type testExtractionService struct{}

func (s *testExtractionService) Extract(
	_ context.Context,
	files []string,
	_ string,
	_ io.Writer,
	onProgress extraction.ProgressCallback,
) (*extraction.ExtractionResult, error) {
	for i, f := range files {
		if onProgress != nil {
			onProgress(i+1, len(files), f)
		}
	}
	return &extraction.ExtractionResult{
		Faces:      []model.Face{},
		FileErrors: map[string]string{},
	}, nil
}

func TestPipelineRunPipeline_ExtractProgressUpdates(t *testing.T) {
	t.Parallel()

	oldCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = oldCfg })

	outDir := t.TempDir()
	config.AppConfig = &config.Config{
		App: env.AppConfig{
			InputDir:  outDir,
			OutputDir: outDir,
		},
		Cluster: env.ClusterConfig{
			Threshold: 0.65,
		},
		Organizer: env.OrganizerConfig{
			AvatarUpdateThreshold: 0.5,
		},
	}

	files := []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg"}
	di := &DiContainer{
		scanService:       &testScanService{files: files},
		extractionService: &testExtractionService{},
	}
	p := NewPipeline(di)

	ch, err := p.RunPipeline(context.Background(), "session-1", outDir)
	if err != nil {
		t.Fatalf("RunPipeline returned error: %v", err)
	}

	var (
		extractProgressValues []float64
		hasExtractCurrentFile bool
		doneSeen              bool
		doneErr               string
	)

	timeout := time.After(3 * time.Second)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				if !doneSeen {
					t.Fatal("expected done event, got channel close")
				}
				if doneErr == "" {
					t.Fatal("expected failure with error, got empty error")
				}
				if len(extractProgressValues) < 2 {
					t.Fatalf("expected multiple extract progress updates, got %d", len(extractProgressValues))
				}
				if extractProgressValues[len(extractProgressValues)-1] <= extractProgressValues[0] {
					t.Fatalf("expected extract progress to increase, got %v", extractProgressValues)
				}
				if !hasExtractCurrentFile {
					t.Fatal("expected extract event with current file")
				}
				return
			}

			if ev.Stage == "extract" {
				extractProgressValues = append(extractProgressValues, ev.Progress)
				if ev.CurrentFile != "" {
					hasExtractCurrentFile = true
				}
			}
			if ev.Done {
				doneSeen = true
				doneErr = ev.Error
			}
		case <-timeout:
			t.Fatal("timed out waiting for pipeline events")
		}
	}
}
