package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var supportedExtensions = map[string]bool{
	".jpeg": true,
	".jpg":  true,
	".png":  true,
}

// Scan walks the given directory and returns absolute paths of all supported image files.
func Scan(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot access directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if supportedExtensions[ext] {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error scanning directory: %w", err)
	}

	return files, nil
}
