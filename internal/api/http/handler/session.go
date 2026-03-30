package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const statusFailed = "failed"

// PipelineRunner is the interface for running the face processing pipeline.
type PipelineRunner interface {
	// RunPipeline starts async processing for the given input directory.
	// Returns a channel that receives progress updates.
	RunPipeline(ctx context.Context, sessionID string, inputDir string) (<-chan ProgressEvent, error)
}

// ProgressEvent represents a pipeline progress update.
type ProgressEvent struct {
	SessionID      string  `json:"session_id"`
	Stage          string  `json:"stage"`       // scan, extract, cluster, organize.
	StageLabel     string  `json:"stage_label"` // Human-readable label.
	Progress       float64 `json:"progress"`    // 0.0 - 1.0
	ProcessedItems int     `json:"processed_items"`
	TotalItems     int     `json:"total_items"`
	CurrentFile    string  `json:"current_file,omitempty"`
	Error          string  `json:"error,omitempty"`
	Done           bool    `json:"done"`
	ElapsedMs      int64   `json:"elapsed_ms"`
}

// SessionHandler handles processing session endpoints.
type SessionHandler struct {
	runner       PipelineRunner
	sessions     sync.Map // sessionID -> sessionState.
	allowedBase  string   // Base directory for input path validation.
}

type sessionState struct {
	ID        string
	InputDir  string
	Status    string // pending, processing, completed, failed.
	Stage     string
	Progress  float64
	StartedAt time.Time
	Error     string
	mu        sync.RWMutex
}

// NewSessionHandler creates a new SessionHandler.
// allowedBase restricts input_dir to paths under this directory (path traversal protection).
func NewSessionHandler(runner PipelineRunner, allowedBase string) *SessionHandler {
	return &SessionHandler{
		runner:      runner,
		allowedBase: filepath.Clean(allowedBase),
	}
}

// StartProcessing handles POST /api/v1/sessions/{id}/process.
func (h *SessionHandler) StartProcessing(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session ID required"})
		return
	}

	var req struct {
		InputDir string `json:"input_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.InputDir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_dir is required"})
		return
	}

	// Path traversal protection: input_dir must be within the allowed base directory.
	cleanDir := filepath.Clean(req.InputDir)
	if !strings.HasPrefix(cleanDir, h.allowedBase+string(os.PathSeparator)) && cleanDir != h.allowedBase {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_dir must be within the allowed upload directory"})
		return
	}
	if info, err := os.Stat(cleanDir); err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "input_dir does not exist or is not a directory"})
		return
	}
	req.InputDir = cleanDir

	// Check if session is already processing.
	if _, loaded := h.sessions.Load(sessionID); loaded {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session is already processing"})
		return
	}

	state := &sessionState{
		ID:        sessionID,
		InputDir:  req.InputDir,
		Status:    "processing",
		Stage:     "starting",
		StartedAt: time.Now(),
	}
	h.sessions.Store(sessionID, state)

	// Start pipeline asynchronously.
	// Don't tie long-running pipeline to the HTTP request context.
	// If the client drops/aborts the connection, r.Context() will be canceled
	// and extraction will stop with "context canceled".
	pipelineCtx := context.WithoutCancel(r.Context())
	progressCh, err := h.runner.RunPipeline(pipelineCtx, sessionID, req.InputDir)
	if err != nil {
		state.mu.Lock()
		state.Status = statusFailed
		state.Error = err.Error()
		state.mu.Unlock()

		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Background goroutine to update session state from progress channel.
	go func() {
		for event := range progressCh {
			state.mu.Lock()
			state.Stage = event.Stage
			state.Progress = event.Progress
			if event.Done {
				if event.Error != "" {
					state.Status = statusFailed
					state.Error = event.Error
				} else {
					state.Status = "completed"
				}
			}
			state.mu.Unlock()
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"session_id": sessionID,
		"status":     "processing",
	})
}

// GetStatus handles GET /api/v1/sessions/{id}/status.
func (h *SessionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	val, ok := h.sessions.Load(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	state, ok := val.(*sessionState)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal state error"})
		return
	}
	state.mu.RLock()
	defer state.mu.RUnlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": state.ID,
		"status":     state.Status,
		"stage":      state.Stage,
		"progress":   state.Progress,
		"elapsed_ms": time.Since(state.StartedAt).Milliseconds(),
		"error":      state.Error,
	})
}

// StreamProgress handles GET /api/v1/sessions/{id}/stream (SSE).
func (h *SessionHandler) StreamProgress(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Poll session state and send SSE events.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			val, ok := h.sessions.Load(sessionID)
			if !ok {
				_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"session not found\"}\n\n")
				flusher.Flush()
				return
			}

			state, ok := val.(*sessionState)
			if !ok {
				_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":\"internal state error\"}\n\n")
				flusher.Flush()
				return
			}
			state.mu.RLock()
			event := ProgressEvent{
				SessionID: state.ID,
				Stage:     state.Stage,
				Progress:  state.Progress,
				Done:      state.Status == "completed" || state.Status == statusFailed,
				Error:     state.Error,
				ElapsedMs: time.Since(state.StartedAt).Milliseconds(),
			}
			state.mu.RUnlock()

			data, err := json.Marshal(event)
			if err != nil {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			if event.Done {
				return
			}
		}
	}
}
