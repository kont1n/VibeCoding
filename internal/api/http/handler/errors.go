package handler

import (
	"net/http"

	"github.com/kont1n/face-grouper/internal/database"
	"github.com/kont1n/face-grouper/internal/report"
)

// ErrorHandler handles error-related API endpoints.
type ErrorHandler struct {
	outputDir string
	db        *database.DB
}

// NewErrorHandler creates a new ErrorHandler.
func NewErrorHandler(outputDir string, db *database.DB) *ErrorHandler {
	return &ErrorHandler{
		outputDir: outputDir,
		db:        db,
	}
}

// FileError represents a processing error for a single file.
type FileError struct {
	File      string `json:"file"`
	Error     string `json:"error"`
	ErrorType string `json:"error_type"` // no_face, processing_error, unsupported_format
}

// GetSessionErrors handles GET /api/v1/sessions/{id}/errors.
func (h *ErrorHandler) GetSessionErrors(w http.ResponseWriter, r *http.Request) {
	// Try database first.
	if h.db != nil && h.db.Sessions != nil {
		// Session errors from DB would require parsing the session by ID.
		// For now, fall through to report.json.
	}

	// Fallback: load from report.json.
	rpt, err := report.Load(h.outputDir)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no data available"})
		return
	}

	var errors []FileError
	for file, errMsg := range rpt.FileErrors {
		fe := FileError{
			File:      file,
			Error:     errMsg,
			ErrorType: classifyError(errMsg),
		}
		errors = append(errors, fe)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"errors": errors,
		"total":  len(errors),
	})
}

func classifyError(errMsg string) string {
	switch {
	case contains(errMsg, "no face", "face not found", "0 faces"):
		return "no_face"
	case contains(errMsg, "unsupported", "format", "decode"):
		return "unsupported_format"
	default:
		return "processing_error"
	}
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
