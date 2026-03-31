package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jpegMagicBytes returns a 512-byte buffer starting with valid JPEG magic bytes.
func jpegMagicBytes() []byte {
	b := make([]byte, 512)
	copy(b, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return b
}

// createFormFileWithType creates a multipart form file part with a specific Content-Type.
func createFormFileWithType(w *multipart.Writer, fieldname, filename, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldname, filename))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}

// buildMultipartRequest builds a POST multipart request where each file is submitted with
// the correct image Content-Type derived from its extension.
func buildMultipartRequest(t *testing.T, files map[string][]byte) (*http.Request, string) {
	t.Helper()

	extToMIME := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		".zip":  "application/zip",
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, content := range files {
		ext := strings.ToLower(filepath.Ext(name))
		mime, ok := extToMIME[ext]
		if !ok {
			mime = "application/octet-stream"
		}
		fw, err := createFormFileWithType(mw, "files", name, mime)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write(content)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req, mw.FormDataContentType()
}

func TestUpload_NoFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	req, _ := buildMultipartRequest(t, map[string][]byte{})
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body)
	}
}

func TestUpload_ValidJPEG(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	req, _ := buildMultipartRequest(t, map[string][]byte{
		"photo.jpg": jpegMagicBytes(),
	})
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body)
	}

	var resp UploadResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.FileCount != 1 {
		t.Fatalf("expected 1 file, got %d", resp.FileCount)
	}
	if resp.SessionID == "" {
		t.Fatal("expected non-empty session_id")
	}
}

func TestUpload_UnsupportedExtension(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	req, _ := buildMultipartRequest(t, map[string][]byte{
		"document.txt": []byte("not an image"),
	})
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpload_InvalidMagicBytes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// .jpg extension but random content — no valid JPEG magic bytes.
	req, _ := buildMultipartRequest(t, map[string][]byte{
		"fake.jpg": []byte("this is not a jpeg at all"),
	})
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpload_PathTraversalInFilename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Filename tries to escape the session directory.
	req, _ := buildMultipartRequest(t, map[string][]byte{
		"../evil.jpg": jpegMagicBytes(),
	})
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	// Go 1.22+ sanitizes "../evil.jpg" → "evil.jpg" in the multipart parser,
	// so the file is saved safely inside the session dir (200).
	// If rejected outright (400), that's also acceptable.
	if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body)
	}

	// Security invariant: no file must have escaped to the parent directory.
	escaped := filepath.Join(dir, "..", "evil.jpg")
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatal("path traversal: file was written outside upload dir")
	}
}

func TestUpload_ZipWithValidJPEG(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, err := zw.Create("photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(jpegMagicBytes())
	_ = zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	ff, err := mw.CreateFormFile("files", "archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = ff.Write(zipBuf.Bytes())
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body)
	}

	var resp UploadResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.FileCount != 1 {
		t.Fatalf("expected 1 file from zip, got %d", resp.FileCount)
	}
}

func TestUpload_ZipWithInvalidContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, _ := zw.Create("fake.jpg")
	_, _ = fw.Write([]byte("not a jpeg"))
	_ = zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	ff, _ := mw.CreateFormFile("files", "archive.zip")
	_, _ = ff.Write(zipBuf.Bytes())
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpload_ZipSlipPrevention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	// Filename in zip tries to escape session dir.
	fw, _ := zw.Create("../../evil.jpg")
	_, _ = fw.Write(jpegMagicBytes())
	_ = zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	ff, _ := mw.CreateFormFile("files", "archive.zip")
	_, _ = ff.Write(zipBuf.Bytes())
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	NewUploadHandler(dir, 2<<30).Upload(rec, req)

	// Zip slip path is sanitized to Base(name) = "evil.jpg" by the handler,
	// so it may succeed but only within session dir — or 400 if no files extracted.
	// Either way, verify nothing escaped to the parent.
	escaped := filepath.Join(dir, "..", "..", "evil.jpg")
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatal("zip slip: file was written outside upload dir")
	}
}
