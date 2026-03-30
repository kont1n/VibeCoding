package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip_SafeExtraction(t *testing.T) {
	// Create a temporary directory for testing.
	tmpDir := t.TempDir()

	// Create a test ZIP archive with safe files.
	archivePath := filepath.Join(tmpDir, "test.zip")
	createTestZip(t, archivePath, []zipFile{
		{name: "image1.jpg", content: "fake jpg content 1"},
		{name: "image2.png", content: "fake png content 2"},
		{name: "subdir/image3.jpg", content: "fake jpg content 3"},
	})

	// Extract the archive.
	destDir := filepath.Join(tmpDir, "extracted")
	err := ExtractZip(archivePath, destDir)
	if err != nil {
		t.Fatalf("ExtractZip failed: %v", err)
	}

	// Verify extracted files.
	verifyFileExists(t, destDir, "image1.jpg")
	verifyFileExists(t, destDir, "image2.png")
	verifyFileExists(t, destDir, "subdir", "image3.jpg")
}

func TestExtractZip_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ZIP archive with path traversal attempt.
	archivePath := filepath.Join(tmpDir, "malicious.zip")
	createTestZip(t, archivePath, []zipFile{
		{name: "../../../etc/passwd", content: "malicious content"},
		{name: "normal.jpg", content: "normal content"},
	})

	destDir := filepath.Join(tmpDir, "extracted")
	err := ExtractZip(archivePath, destDir)
	if err == nil {
		t.Fatal("ExtractZip should fail with path traversal attempt")
	}

	// Verify that malicious file was not created.
	maliciousPath := filepath.Join(tmpDir, "etc", "passwd")
	if _, err := os.Stat(maliciousPath); err == nil {
		t.Fatal("Malicious file was created despite path traversal protection")
	}
}

func TestExtractZip_FileSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test ZIP archive with oversized file.
	archivePath := filepath.Join(tmpDir, "oversized.zip")
	createTestZipWithSize(t, archivePath, "large.jpg", MaxArchiveFileSize+1)

	destDir := filepath.Join(tmpDir, "extracted")
	err := ExtractZip(archivePath, destDir)
	if err == nil {
		t.Fatal("ExtractZip should fail with oversized file")
	}
}

func TestIsAllowedImageExt(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".jpg", true},
		{".jpeg", true},
		{".png", true},
		{".webp", true},
		{".gif", true},
		{".bmp", true},
		{".tiff", true},
		{".tif", true},
		{".JPG", false}, // case sensitive.
		{".txt", false},
		{".exe", false},
		{".sh", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := IsAllowedImageExt(tt.ext)
			if result != tt.expected {
				t.Errorf("IsAllowedImageExt(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

type zipFile struct {
	name    string
	content string
}

func createTestZip(t *testing.T, path string, files []zipFile) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	for _, f := range files {
		fw, err := w.Create(f.name)
		if err != nil {
			t.Fatalf("Failed to create file in zip: %v", err)
		}
		if _, err := fw.Write([]byte(f.content)); err != nil {
			t.Fatalf("Failed to write to zip: %v", err)
		}
	}
}

func createTestZipWithSize(t *testing.T, path, name string, size int64) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	fw, err := w.Create(name)
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}

	// Write dummy data to reach the desired size.
	buf := make([]byte, 1024)
	written := int64(0)
	for written < size {
		toWrite := int64(len(buf))
		if written+toWrite > size {
			toWrite = size - written
		}
		if _, err := fw.Write(buf[:toWrite]); err != nil {
			t.Fatalf("Failed to write to zip: %v", err)
		}
		written += toWrite
	}
}

func verifyFileExists(t *testing.T, baseDir string, parts ...string) {
	t.Helper()
	path := filepath.Join(baseDir, filepath.Join(parts...))
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File %s does not exist: %v", path, err)
	}
}
