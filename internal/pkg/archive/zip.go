// Package archive provides utilities for working with archive files.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxArchiveFileSize is the maximum allowed file size in an archive (50MB).
const MaxArchiveFileSize = 50 << 20

var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
	".bmp":  true,
	".tiff": true,
	".tif":  true,
}

// ExtractZip safely extracts a ZIP archive to the specified destination directory.
// It protects against Zip Slip vulnerabilities and validates file types and sizes.
func ExtractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = r.Close() }()

	// Resolve destination directory to absolute path.
	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve dest dir: %w", err)
	}

	// Ensure destination directory exists.
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	for _, f := range r.File {
		if err := extractFile(f, destDir); err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the archive.
func extractFile(f *zip.File, destDir string) error {
	target, err := sanitizePath(f.Name, destDir)
	if err != nil {
		return err
	}

	// Skip if file is too large.
	if f.UncompressedSize64 > MaxArchiveFileSize {
		return fmt.Errorf("file too large: %s (%d bytes)", f.Name, f.UncompressedSize64)
	}

	// Validate file extension.
	ext := strings.ToLower(filepath.Ext(f.Name))
	if !isAllowedImageExt(ext) {
		// Skip non-image files silently.
		return nil
	}

	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", target, err)
		}
		return nil
	}

	// Create parent directories.
	parentDir := filepath.Dir(target)
	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	// Extract file.
	if err := extractFileContent(f, target); err != nil {
		return err
	}

	// Set file permissions.
	if err := os.Chmod(target, 0o640); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}

	return nil
}

// sanitizePath validates and sanitizes the file path to prevent Zip Slip.
func sanitizePath(name, destDir string) (string, error) {
	target := filepath.Join(destDir, name)

	// Clean the path.
	cleanTarget := filepath.Clean(target)

	// Ensure the path is within the destination directory.
	// Add path separator to prevent prefix attacks (e.g., "dest" vs "destevil").
	if !strings.HasPrefix(cleanTarget+string(os.PathSeparator), destDir+string(os.PathSeparator)) &&
		cleanTarget != destDir {
		return "", fmt.Errorf("illegal file path in archive: %s", name)
	}

	return cleanTarget, nil
}

// extractFileContent extracts the content of a file.
func extractFileContent(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open file in archive: %w", err)
	}
	defer func() { _ = rc.Close() }()

	outFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create file %s: %w", target, err)
	}
	defer func() { _ = outFile.Close() }()

	// Limit the amount of data read to prevent decompression bombs.
	limitedReader := io.LimitReader(rc, MaxArchiveFileSize+1)
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return fmt.Errorf("write file %s: %w", target, err)
	}

	if written > MaxArchiveFileSize {
		_ = os.Remove(target)
		return fmt.Errorf("file too large after extraction: %s", f.Name)
	}

	return nil
}

// isAllowedImageExt checks if the file extension is an allowed image type.
func isAllowedImageExt(ext string) bool {
	return allowedExts[ext]
}

// IsAllowedImageExt exports the extension check for external use.
func IsAllowedImageExt(ext string) bool {
	return isAllowedImageExt(ext)
}
