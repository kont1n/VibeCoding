package handler

import (
	"encoding/json"
	"net/http"

	pkgerrors "github.com/kont1n/face-grouper/internal/pkg/errors"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSON is kept for backward compatibility within the handler package.
func writeJSON(w http.ResponseWriter, status int, v any) {
	WriteJSON(w, status, v)
}

// WriteError writes a structured AppError response.
func WriteError(w http.ResponseWriter, appErr *pkgerrors.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   appErr.Message,
		"code":    string(appErr.Code),
		"message": appErr.Message,
		"details": appErr.Details,
	})
}
