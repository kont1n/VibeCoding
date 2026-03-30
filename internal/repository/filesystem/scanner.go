// Package filesystem provides file system repository implementations.
package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// ScannerRepository определяет интерфейс для сканирования файловой системы.
type ScannerRepository interface {
	Scan(dir string) ([]string, error)
}

type scannerRepository struct {
	supportedExts map[string]bool
}

// NewScannerRepository создаёт новый экземпляр репозитория.
func NewScannerRepository() ScannerRepository {
	return &scannerRepository{
		supportedExts: map[string]bool{
			".jpeg": true,
			".jpg":  true,
			".png":  true,
		},
	}
}

// Scan сканирует директорию и возвращает список поддерживаемых изображений.
func (r *scannerRepository) Scan(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if r.supportedExts[ext] {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}

		return nil
	})

	return files, err
}
