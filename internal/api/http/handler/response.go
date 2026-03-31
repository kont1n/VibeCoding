package handler

import (
	"encoding/json"
	"net/http"
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


