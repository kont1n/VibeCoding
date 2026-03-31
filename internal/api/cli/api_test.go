package cli

import (
	"context"
	"io"
	"testing"

	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/extraction"
)

type stubExtractionService struct {
	receivedCallback bool
}

func (s *stubExtractionService) Extract(
	_ context.Context,
	_ []string,
	_ string,
	_ io.Writer,
	onProgress extraction.ProgressCallback,
) (*extraction.ExtractionResult, error) {
	if onProgress != nil {
		s.receivedCallback = true
		onProgress(1, 2, "file.jpg")
	}
	return &extraction.ExtractionResult{
		Faces:      []model.Face{},
		FileErrors: map[string]string{},
	}, nil
}

func TestAPIExtract_ForwardsProgressCallback(t *testing.T) {
	t.Parallel()

	extractSvc := &stubExtractionService{}
	api := NewAPI(nil, extractSvc, nil, nil)

	callbackCalls := 0
	_, err := api.Extract(context.Background(), []string{"f1.jpg"}, "", io.Discard, func(processed, total int, filePath string) {
		callbackCalls++
		if processed != 1 || total != 2 || filePath != "file.jpg" {
			t.Fatalf("unexpected callback payload: processed=%d total=%d file=%q", processed, total, filePath)
		}
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !extractSvc.receivedCallback {
		t.Fatal("expected callback to be forwarded to extraction service")
	}
	if callbackCalls != 1 {
		t.Fatalf("expected callback to be called once, got %d", callbackCalls)
	}
}
