package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPersonList_NoDB_NoReport verifies 404 when no DB and no report.json.
func TestPersonList_NoDB_NoReport(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/persons", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body)
	}
}

// TestPersonGet_NoDB_NoReport verifies 404 when no DB and no report.json.
func TestPersonGet_NoDB_NoReport(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/42", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body)
	}
}

// TestPersonRename_NoDB_NoReport verifies 404 when no database and no report.json.
func TestPersonRename_NoDB_NoReport(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/persons/123", strings.NewReader(`{"name":"Alice"}`))
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()
	h.Rename(rec, req)

	// Without DB, falls back to report.json which doesn't exist → 404.
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body)
	}
}

// TestPersonPhotos_NoDB_NoReport verifies 404 when no DB and no report.json.
func TestPersonPhotos_NoDB_NoReport(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/42/photos", nil)
	req.SetPathValue("id", "42")
	rec := httptest.NewRecorder()
	h.Photos(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body)
	}
}

// TestPersonRelations_NoDB verifies 503 when no database is configured.
func TestPersonRelations_NoDB(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/00000000-0000-0000-0000-000000000001/relations", nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000001")
	rec := httptest.NewRecorder()
	h.Relations(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d: %s", http.StatusServiceUnavailable, rec.Code, rec.Body)
	}
}

// TestPersonRename_InvalidID_NoDB verifies 400 when ID is not a valid integer (no DB path).
func TestPersonRename_InvalidID_NoDB(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/persons/not-a-number", strings.NewReader(`{"name":"Alice"}`))
	req.SetPathValue("id", "not-a-number")
	rec := httptest.NewRecorder()
	h.Rename(rec, req)

	// Without DB: falls through to integer parsing which fails → 400.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
