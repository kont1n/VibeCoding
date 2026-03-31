package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	pkgerrors "github.com/kont1n/face-grouper/internal/pkg/errors"
	"github.com/kont1n/face-grouper/internal/service/imageutil"
)

// Allowed image extensions.
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

// Allowed MIME types.
var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
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
		WriteError(w, pkgerrors.NewValidation("Failed to parse upload: "+err.Error()))
		return
	}

	sessionID := uuid.New().String()
	sessionDir := filepath.Join(h.uploadDir, sessionID)

	if err := os.MkdirAll(sessionDir, 0o750); err != nil { //nolint:gosec
		WriteError(w, pkgerrors.NewInternal("Failed to create upload directory", err))
		return
	}

	var savedFiles []string
	var totalSize int64

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		_ = os.RemoveAll(sessionDir)
		WriteError(w, pkgerrors.NewFileUpload("No files provided"))
		return
	}

	for _, fileHeader := range files {
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

		if ext == ".zip" {
			// Handle zip archive.
			extracted, size, err := h.handleZip(fileHeader, sessionDir)
			if err != nil {
				_ = os.RemoveAll(sessionDir)
				WriteError(w, pkgerrors.NewFileUpload("Failed to process zip: "+err.Error()))
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

		// Validate MIME type from Content-Type header.
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType != "" && !allowedMimeTypes[contentType] {
			_ = src.Close()
			continue // Skip unsupported MIME types.
		}

		dstPath := filepath.Join(sessionDir, fileHeader.Filename)
		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(dstPath), filepath.Clean(sessionDir)) {
			_ = src.Close()
			continue
		}

		// Validate image header (magic bytes).
		header := make([]byte, 512)
		n, _ := src.Read(header)
		if !imageutil.ValidateImageHeader(header[:n]) {
			_ = src.Close()
			continue // Skip invalid images.
		}

		// Seek back to beginning after reading header.
		if seeker, ok := src.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		} else {
			// If not seekable, close and reopen.
			_ = src.Close()
			src, _ = fileHeader.Open()
		}

		dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
		if err != nil {
			_ = src.Close()
			continue
		}

		written, err := io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()

		if err != nil {
			_ = os.Remove(dstPath) //nolint:gosec // G703: dstPath checked against sessionDir above
			continue
		}

		savedFiles = append(savedFiles, fileHeader.Filename)
		totalSize += written
	}

	if len(savedFiles) == 0 {
		_ = os.RemoveAll(sessionDir)
		WriteError(w, pkgerrors.NewFileUpload("No valid image files found in upload"))
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

func (h *UploadHandler) handleZip(fileHeader *multipart.FileHeader, sessionDir string) (files []string, totalSize int64, err error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, 0, err
	}

	// Save zip to temp file for processing.
	tmpFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		_ = src.Close()
		return nil, 0, err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = io.Copy(tmpFile, src)
	_ = src.Close()
	_ = tmpFile.Close()
	if err != nil {
		return nil, 0, err
	}

	// Zip bomb protection: limit total extracted size to 2GB.
	const maxExtractedSize = 2 << 30

	reader, err := zip.OpenReader(tmpFile.Name()) //nolint:gosec
	if err != nil {
		return nil, 0, fmt.Errorf("invalid zip file: %w", err)
	}
	defer func() { _ = reader.Close() }()

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
		if totalSize+int64(f.UncompressedSize64) > maxExtractedSize { //nolint:gosec
			return nil, 0, fmt.Errorf("zip extraction would exceed %d bytes limit", maxExtractedSize)
		}

		rc, openErr := f.Open()
		if openErr != nil {
			continue
		}

		// Validate image header before extracting.
		header := make([]byte, 512)
		n, _ := rc.Read(header)
		if !imageutil.ValidateImageHeader(header[:n]) {
			_ = rc.Close()
			continue // Skip invalid images silently.
		}

		// Reset to beginning.
		_ = rc.Close()
		rc, _ = f.Open()

		dst, createErr := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
		if createErr != nil {
			_ = rc.Close()
			continue
		}

		// Limit copy to prevent zip bomb.
		copied, copyErr := io.Copy(dst, io.LimitReader(rc, maxExtractedSize-totalSize))
		_ = rc.Close()
		_ = dst.Close()

		if copyErr != nil {
			_ = os.Remove(destPath)
			continue
		}

		files = append(files, filepath.Base(f.Name))
		totalSize += copied
	}

	return files, totalSize, nil
}
