package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanReturnsOnlySupportedImages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	mustWriteFile(t, filepath.Join(root, "a.jpg"))
	mustWriteFile(t, filepath.Join(root, "b.JPEG"))
	mustWriteFile(t, filepath.Join(nested, "c.png"))
	mustWriteFile(t, filepath.Join(root, "ignored.txt"))

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 images, got %d: %v", len(got), got)
	}

	gotSet := make(map[string]bool, len(got))
	for _, p := range got {
		if !filepath.IsAbs(p) {
			t.Fatalf("path must be absolute: %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatalf("filepath.Rel failed for %s: %v", p, err)
		}
		gotSet[filepath.ToSlash(rel)] = true
	}

	want := []string{"a.jpg", "b.JPEG", "nested/c.png"}
	for _, rel := range want {
		if !gotSet[rel] {
			t.Fatalf("expected file %q in scan result, got: %v", rel, gotSet)
		}
	}
}

func TestScanRejectsNonDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "file.jpg")
	mustWriteFile(t, filePath)

	_, err := Scan(filePath)
	if err == nil {
		t.Fatal("expected error for non-directory path, got nil")
	}
}

func TestScanMissingDirectory(t *testing.T) {
	t.Parallel()

	_, err := Scan(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
