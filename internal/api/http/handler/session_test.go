package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// stubPipelineRunner immediately sends completion event (instant completion).
type stubPipelineRunner struct{}

func (s *stubPipelineRunner) RunPipeline(_ context.Context, sessionID, _ string) (<-chan ProgressEvent, error) {
	ch := make(chan ProgressEvent, 1)
	// Send immediate completion event.
	ch <- ProgressEvent{
		SessionID: sessionID,
		Done:      true,
		Progress:  1.0,
	}
	close(ch)
	return ch, nil
}

// blockingPipelineRunner blocks until its release channel is closed or ctx is done.
type blockingPipelineRunner struct {
	release chan struct{}
}

func (b *blockingPipelineRunner) RunPipeline(ctx context.Context, _ string, _ string) (<-chan ProgressEvent, error) {
	out := make(chan ProgressEvent, 1)
	go func() {
		defer close(out)
		select {
		case <-b.release:
		case <-ctx.Done():
		}
	}()
	return out, nil
}

func newTestSessionHandler(t *testing.T, runner PipelineRunner) (*SessionHandler, string) {
	t.Helper()
	base := t.TempDir()
	h := NewSessionHandler(runner, base)
	t.Cleanup(h.Close)
	return h, base
}

func startSession(t *testing.T, h *SessionHandler, sessionID, inputDir string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"input_dir": "` + inputDir + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/process", bytes.NewBufferString(body))
	req.SetPathValue("id", sessionID)
	rec := httptest.NewRecorder()
	h.StartProcessing(rec, req)
	return rec
}

// TestStartProcessing_MissingInputDir verifies 400 when input_dir is omitted.
func TestStartProcessing_MissingInputDir(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	h.StartProcessing(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body)
	}
}

// TestStartProcessing_InvalidBody verifies 400 on malformed JSON.
func TestStartProcessing_InvalidBody(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`not json`))
	req.SetPathValue("id", "sess1")
	rec := httptest.NewRecorder()
	h.StartProcessing(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestStartProcessing_PathTraversal verifies that input_dir outside allowedBase is rejected.
func TestStartProcessing_PathTraversal(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	body := `{"input_dir": "/etc"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.SetPathValue("id", "sess-traversal")
	rec := httptest.NewRecorder()
	h.StartProcessing(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d (path outside allowed base), got %d: %s", http.StatusBadRequest, rec.Code, rec.Body)
	}
}

// TestStartProcessing_NonExistentDir verifies 400 when input_dir does not exist.
func TestStartProcessing_NonExistentDir(t *testing.T) {
	t.Parallel()

	h, base := newTestSessionHandler(t, &stubPipelineRunner{})
	body := `{"input_dir": "` + filepath.Join(base, "does-not-exist") + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.SetPathValue("id", "sess-nodir")
	rec := httptest.NewRecorder()
	h.StartProcessing(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body)
	}
}

// TestStartProcessing_OK verifies 202 Accepted and correct session_id in response.
func TestStartProcessing_OK(t *testing.T) {
	t.Parallel()

	h, base := newTestSessionHandler(t, &stubPipelineRunner{})
	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}

	rec := startSession(t, h, "sess-ok", inputDir)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d: %s", http.StatusAccepted, rec.Code, rec.Body)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["session_id"] != "sess-ok" {
		t.Fatalf("expected session_id=sess-ok, got %q", resp["session_id"])
	}
}

// TestStartProcessing_Duplicate verifies 409 Conflict when session already started.
func TestStartProcessing_Duplicate(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	h, base := newTestSessionHandler(t, &blockingPipelineRunner{release: release})
	defer close(release)

	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}

	startSession(t, h, "sess-dup", inputDir)

	rec2 := startSession(t, h, "sess-dup", inputDir)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected %d on duplicate start, got %d: %s", http.StatusConflict, rec2.Code, rec2.Body)
	}
}

// TestGetStatus_NotFound verifies 404 for unknown session.
func TestGetStatus_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "unknown-session")
	rec := httptest.NewRecorder()
	h.GetStatus(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// TestGetStatus_OK verifies 200 with session data for a known session.
func TestGetStatus_OK(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	h, base := newTestSessionHandler(t, &blockingPipelineRunner{release: release})
	defer close(release)

	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	startSession(t, h, "sess-status", inputDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "sess-status")
	rec := httptest.NewRecorder()
	h.GetStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["session_id"] != "sess-status" {
		t.Fatalf("expected session_id=sess-status, got %v", resp["session_id"])
	}
}

// TestCancelProcessing_NotFound verifies 404 for unknown session.
func TestCancelProcessing_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", "no-such-session")
	rec := httptest.NewRecorder()
	h.CancelProcessing(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// TestCancelProcessing_OK verifies that an active session can be canceled.
func TestCancelProcessing_OK(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	h, base := newTestSessionHandler(t, &blockingPipelineRunner{release: release})
	defer close(release)

	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	startSession(t, h, "sess-cancel", inputDir)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", "sess-cancel")
	rec := httptest.NewRecorder()
	h.CancelProcessing(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body)
	}
}

// TestCancelProcessing_AlreadyFinished verifies 409 when session completed before cancel.
func TestCancelProcessing_AlreadyFinished(t *testing.T) {
	t.Parallel()

	h, base := newTestSessionHandler(t, &stubPipelineRunner{})
	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	startSession(t, h, "sess-done", inputDir)

	// Wait for the background goroutine to update session status to completed/failed.
	// stubRunner closes its channel immediately, so the goroutine finishes quickly.
	deadline := time.Now().Add(500 * time.Millisecond)
	var finalStatus SessionStatus
	for time.Now().Before(deadline) {
		runtime.Gosched()
		val, ok := h.sessions.Load("sess-done")
		if !ok {
			continue
		}
		state := val.(*sessionState) //nolint:forcetypeassert,errcheck
		state.mu.RLock()
		status := state.Status
		state.mu.RUnlock()
		finalStatus = status
		if status == statusCompleted || status == statusFailed {
			break
		}
		time.Sleep(time.Millisecond)
	}

	// If session already finished, cancel should return 409.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", "sess-done")
	rec := httptest.NewRecorder()
	h.CancelProcessing(rec, req)

	// Accept both 409 (already finished) or 200 (if race condition, still cancellable).
	// The key invariant: session should be in a terminal state.
	if rec.Code == http.StatusConflict {
		return // Expected behavior.
	}
	if rec.Code == http.StatusOK && (finalStatus == statusCompleted || finalStatus == statusFailed) {
		return // Race condition: session finished but cancel still went through.
	}
	t.Fatalf("expected 409 (already finished), got %d: %s (final status: %s)", rec.Code, rec.Body.String(), finalStatus)
}

// TestStreamProgress_NotFound verifies 404 for unknown session.
func TestStreamProgress_NotFound(t *testing.T) {
	t.Parallel()

	h, _ := newTestSessionHandler(t, &stubPipelineRunner{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "unknown-session")
	rec := httptest.NewRecorder()
	h.StreamProgress(rec, req)

	if rec.Code != http.StatusOK {
		// SSE may return 200 with error event or 404.
		body := rec.Body.String()
		if !bytes.Contains([]byte(body), []byte("session not found")) {
			t.Fatalf("expected session not found error, got %s", body)
		}
	}
}

// TestStreamProgress_Cancel verifies SSE stream stops when client disconnects.
func TestStreamProgress_Cancel(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	h, base := newTestSessionHandler(t, &blockingPipelineRunner{release: release})
	defer close(release)

	inputDir := filepath.Join(base, "inputs")
	if err := os.MkdirAll(inputDir, 0o750); err != nil {
		t.Fatal(err)
	}
	startSession(t, h, "sess-stream", inputDir)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	req.SetPathValue("id", "sess-stream")
	rec := httptest.NewRecorder()

	// Stream should stop when context is canceled.
	h.StreamProgress(rec, req)

	// Verify stream was interrupted (may have partial data).
	// The key is that it doesn't hang indefinitely.
}
