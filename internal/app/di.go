package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/kont1n/face-grouper/internal/api/cli"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/inference"
	"github.com/kont1n/face-grouper/internal/repository/filesystem"
	inferenceRepo "github.com/kont1n/face-grouper/internal/repository/inference"
	"github.com/kont1n/face-grouper/internal/service/clustering"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organization"
	"github.com/kont1n/face-grouper/internal/service/scan"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
)

// diContainer управляет зависимостями приложения с lazy initialization.
type diContainer struct {
	api *cli.API

	scanService       scan.ScanService
	extractionService extraction.ExtractionService
	clusterService    clustering.ClusterService
	organizeService   organization.OrganizeService

	scannerRepo    filesystem.ScannerRepository
	detectorPool   []inferenceRepo.DetectorRepository
	recognizerPool []inferenceRepo.RecognizerRepository
}

// NewDiContainer создаёт новый DI контейнер.
func NewDiContainer() *diContainer {
	return &diContainer{}
}

// API возвращает CLI API, инициализируя при необходимости.
func (d *diContainer) API(ctx context.Context) *cli.API {
	if d.api == nil {
		d.api = cli.NewAPI(
			d.ScanService(ctx),
			d.ExtractionService(ctx),
			d.ClusterService(ctx),
			d.OrganizeService(ctx),
		)
	}
	return d.api
}

// ScanService возвращает сервис сканирования.
func (d *diContainer) ScanService(ctx context.Context) scan.ScanService {
	if d.scanService == nil {
		d.scanService = scan.NewScanService(d.ScannerRepository())
	}
	return d.scanService
}

// ExtractionService возвращает сервис экстракции.
func (d *diContainer) ExtractionService(ctx context.Context) extraction.ExtractionService {
	if d.extractionService == nil {
		d.extractionService = extraction.NewExtractionService(
			config.AppConfig.Extract,
			d.DetectorPool(ctx),
			d.RecognizerPool(ctx),
		)
	}
	return d.extractionService
}

// ClusterService возвращает сервис кластеризации.
func (d *diContainer) ClusterService(ctx context.Context) clustering.ClusterService {
	if d.clusterService == nil {
		d.clusterService = clustering.NewClusterService()
	}
	return d.clusterService
}

// OrganizeService возвращает сервис организации.
func (d *diContainer) OrganizeService(ctx context.Context) organization.OrganizeService {
	if d.organizeService == nil {
		d.organizeService = organization.NewOrganizeService()
	}
	return d.organizeService
}

// ScannerRepository возвращает репозиторий сканирования.
func (d *diContainer) ScannerRepository() filesystem.ScannerRepository {
	if d.scannerRepo == nil {
		d.scannerRepo = filesystem.NewScannerRepository()
	}
	return d.scannerRepo
}

// DetectorPool возвращает пул детекторов.
func (d *diContainer) DetectorPool(ctx context.Context) []inferenceRepo.DetectorRepository {
	if d.detectorPool == nil {
		cfg := config.AppConfig.Extract
		modelsDir := config.AppConfig.Models.Dir

		sessions := cfg.Workers
		if cfg.GPU {
			sessions = cfg.GPUDetSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]inferenceRepo.DetectorRepository, sessions)
		for i := 0; i < sessions; i++ {
			det, err := inferenceRepo.NewDetectorRepository(inference.DetectorConfig{
				ModelPath: filepath.Join(modelsDir, "det_10g.onnx"),
				GPU:       cfg.GPU,
				DetThresh: float32(cfg.DetThresh),
			})
			if err != nil {
				panic(fmt.Sprintf("failed to create detector %d: %v", i, err))
			}
			pool[i] = det

			// Регистрация в graceful shutdown
			closer.AddNamed(fmt.Sprintf("Detector %d", i), func(ctx context.Context) error {
				det.Close()
				return nil
			})
		}
		d.detectorPool = pool
	}
	return d.detectorPool
}

// RecognizerPool возвращает пул распознавателей.
func (d *diContainer) RecognizerPool(ctx context.Context) []inferenceRepo.RecognizerRepository {
	if d.recognizerPool == nil {
		cfg := config.AppConfig.Extract
		modelsDir := config.AppConfig.Models.Dir

		sessions := cfg.Workers
		if cfg.GPU {
			sessions = cfg.GPURecSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]inferenceRepo.RecognizerRepository, sessions)
		for i := 0; i < sessions; i++ {
			rec, err := inferenceRepo.NewRecognizerRepository(inference.RecognizerConfig{
				ModelPath: filepath.Join(modelsDir, "w600k_r50.onnx"),
				GPU:       cfg.GPU,
			})
			if err != nil {
				panic(fmt.Sprintf("failed to create recognizer %d: %v", i, err))
			}
			pool[i] = rec

			// Регистрация в graceful shutdown
			closer.AddNamed(fmt.Sprintf("Recognizer %d", i), func(ctx context.Context) error {
				rec.Close()
				return nil
			})
		}
		d.recognizerPool = pool
	}
	return d.recognizerPool
}
