package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// Allowed image extensions.
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// UploadHandler handles file upload requests.
type UploadHandler struct {
	uploadDir string
	maxSize   int64
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(uploadDir string, maxSize int64) *UploadHandler {
	return &UploadHandler{
		uploadDir: uploadDir,
		maxSize:   maxSize,
	}
}

// UploadResponse is returned after successful upload.
type UploadResponse struct {
	SessionID  string   `json:"session_id"`
	FileCount  int      `json:"file_count"`
	TotalSize  int64    `json:"total_size"`
	Files      []string `json:"files"`
	UploadPath string   `json:"upload_path"`
}

// Upload handles POST /api/v1/upload.
// Accepts multipart/form-data with field "files" (images or .zip).
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxSize)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "failed to parse upload: " + err.Error(),
		})
		return
	}

	sessionID := uuid.New().String()
	sessionDir := filepath.Join(h.uploadDir, sessionID)

	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to create upload directory",
		})
		return
	}

	var savedFiles []string
	var totalSize int64

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		os.RemoveAll(sessionDir)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no files provided",
		})
		return
	}

	for _, fileHeader := range files {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

		if ext == ".zip" {
			// Handle zip archive.
			extracted, size, err := h.handleZip(fileHeader, sessionDir)
			if err != nil {
				os.RemoveAll(sessionDir)
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error": "failed to process zip: " + err.Error(),
				})
				return
			}
			savedFiles = append(savedFiles, extracted...)
			totalSize += size
			continue
		}

		if !allowedExtensions[ext] {
			continue // Skip unsupported files silently.
		}

		src, err := fileHeader.Open()
		if err != nil {
			continue
		}

		dstPath := filepath.Join(sessionDir, fileHeader.Filename)
		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(dstPath), filepath.Clean(sessionDir)) {
			src.Close()
			continue
		}

		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			continue
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()

		if err != nil {
			os.Remove(dstPath)
			continue
		}

		savedFiles = append(savedFiles, fileHeader.Filename)
		totalSize += written
	}

	if len(savedFiles) == 0 {
		os.RemoveAll(sessionDir)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no valid image files found in upload",
		})
		return
	}

	resp := UploadResponse{
		SessionID:  sessionID,
		FileCount:  len(savedFiles),
		TotalSize:  totalSize,
		Files:      savedFiles,
		UploadPath: sessionDir,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *UploadHandler) handleZip(fileHeader *multipart.FileHeader, sessionDir string) ([]string, int64, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, 0, err
	}

	// Save zip to temp file for processing.
	tmpFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		src.Close()
		return nil, 0, err
	}
	defer os.Remove(tmpFile.Name())

	written, err := io.Copy(tmpFile, src)
	src.Close()
	tmpFile.Close()
	if err != nil {
		return nil, 0, err
	}

	// Zip bomb protection: limit total extracted size to 2GB.
	const maxExtractedSize = 2 << 30

	reader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return nil, 0, fmt.Errorf("invalid zip file: %w", err)
	}
	defer reader.Close()

	var files []string
	var totalSize int64

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		if !allowedExtensions[ext] {
			continue
		}

		// Prevent zip slip.
		destPath := filepath.Join(sessionDir, filepath.Base(f.Name))
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(sessionDir)) {
			continue
		}

		// Check uncompressed size.
		if totalSize+int64(f.UncompressedSize64) > maxExtractedSize {
			return nil, 0, fmt.Errorf("zip extraction would exceed %d bytes limit", maxExtractedSize)
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}

		dst, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			continue
		}

		// Limit copy to prevent zip bomb.
		n, err := io.Copy(dst, io.LimitReader(rc, maxExtractedSize-totalSize))
		rc.Close()
		dst.Close()

		if err != nil {
			os.Remove(destPath)
			continue
		}

		files = append(files, filepath.Base(f.Name))
		totalSize += n
		_ = written // suppress unused
	}

	return files, totalSize, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
