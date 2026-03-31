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

// TestPersonRename_NoDB verifies 503 when no database is configured.
func TestPersonRename_NoDB(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/persons/123", strings.NewReader(`{"name":"Alice"}`))
	req.SetPathValue("id", "123")
	rec := httptest.NewRecorder()
	h.Rename(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d: %s", http.StatusServiceUnavailable, rec.Code, rec.Body)
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

// TestPersonRename_InvalidUUID verifies 400 when UUID in path is malformed (DB path).
// Without a real DB this hits the nil-DB guard → 503, so this tests the nil path.
func TestPersonRename_InvalidUUID_NoDB(t *testing.T) {
	t.Parallel()

	h := NewPersonHandler(t.TempDir(), nil)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/persons/not-a-uuid", strings.NewReader(`{"name":"Alice"}`))
	req.SetPathValue("id", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.Rename(rec, req)

	// Without DB: immediately 503 before UUID parsing.
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}
