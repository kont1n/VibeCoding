package scan

import (
	"context"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/repository/filesystem"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// ScanService определяет интерфейс сервиса сканирования.
type ScanService interface {
	Scan(ctx context.Context, dir string) ([]string, error)
}

type scanService struct {
	scannerRepo filesystem.ScannerRepository
}

// NewScanService создаёт новый экземпляр сервиса сканирования.
func NewScanService(scannerRepo filesystem.ScannerRepository) ScanService {
	return &scanService{
		scannerRepo: scannerRepo,
	}
}

// Scan сканирует директорию и возвращает список изображений.
func (s *scanService) Scan(ctx context.Context, dir string) ([]string, error) {
	logger.Info(ctx, "scanning directory", zap.String("dir", dir))

	files, err := s.scannerRepo.Scan(dir)
	if err != nil {
		logger.Error(ctx, "failed to scan directory", zap.Error(err))
		return nil, err
	}

	logger.Info(ctx, "scan completed", zap.Int("files_found", len(files)))
	return files, nil
}
