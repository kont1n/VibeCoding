package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/kont1n/face-grouper/internal/api/http/handler"
)

type stubPipelineRunner struct{}

func (s *stubPipelineRunner) RunPipeline(_ context.Context, _ string, _ string) (<-chan api.ProgressEvent, error) {
	ch := make(chan api.ProgressEvent)
	close(ch)
	return ch, nil
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := ServerConfig{
		Port:      0,
		OutputDir: t.TempDir(),
		UploadDir: t.TempDir(),
	}

	return NewServer(cfg, &stubPipelineRunner{})
}

func TestHealthEndpoints_OK(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	srv.healthHandler.HealthCheck(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}
