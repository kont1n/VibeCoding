package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/kont1n/face-grouper/internal/api/cli"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/database"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml/provider"
	"github.com/kont1n/face-grouper/internal/repository/filesystem"
	"github.com/kont1n/face-grouper/internal/repository/postgres"
	"github.com/kont1n/face-grouper/internal/service/clustering"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organization"
	"github.com/kont1n/face-grouper/internal/service/scan"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// DiContainer управляет зависимостями приложения с lazy initialization.
type DiContainer struct {
	api *cli.API
	db  *database.DB

	scanService       scan.ScanService
	extractionService extraction.ExtractionService
	clusterService    clustering.ClusterService
	organizeService   organization.OrganizeService

	scannerRepo    filesystem.ScannerRepository
	detectorPool   []ml.DetectorRepository
	recognizerPool []ml.RecognizerRepository
}

// SetDatabase устанавливает соединение с базой данных.
func (d *DiContainer) SetDatabase(db *database.DB) {
	d.db = db
}

// NewDiContainer создаёт новый DI контейнер.
func NewDiContainer() *DiContainer {
	return &DiContainer{}
}

// API возвращает CLI API, инициализируя при необходимости.
func (d *DiContainer) API(ctx context.Context) *cli.API {
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
func (d *DiContainer) ScanService(ctx context.Context) scan.ScanService {
	if d.scanService == nil {
		d.scanService = scan.NewScanService(d.ScannerRepository())
	}
	return d.scanService
}

// ExtractionService возвращает сервис экстракции.
func (d *DiContainer) ExtractionService(ctx context.Context) extraction.ExtractionService {
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
func (d *DiContainer) ClusterService(ctx context.Context) clustering.ClusterService {
	if d.clusterService == nil {
		d.clusterService = clustering.NewClusterService()
	}
	return d.clusterService
}

// OrganizeService возвращает сервис организации.
func (d *DiContainer) OrganizeService(ctx context.Context) organization.OrganizeService {
	if d.organizeService == nil {
		d.organizeService = organization.NewOrganizeService()
	}
	return d.organizeService
}

// ScannerRepository возвращает репозиторий сканирования.
func (d *DiContainer) ScannerRepository() filesystem.ScannerRepository {
	if d.scannerRepo == nil {
		d.scannerRepo = filesystem.NewScannerRepository()
	}
	return d.scannerRepo
}

// DetectorPool возвращает пул детекторов.
func (d *DiContainer) DetectorPool(ctx context.Context) []ml.DetectorRepository {
	if d.detectorPool == nil {
		cfg := config.AppConfig.Extract
		modelsDir := config.AppConfig.Models.Dir

		// Determine preferred provider type.
		var preferred provider.ProviderType
		if cfg.GPU {
			preferred = provider.ProviderCUDA
			if cfg.ProviderPriority != "" && cfg.ProviderPriority != providerPriorityAuto {
				preferred = provider.ParseProviderType(cfg.ProviderPriority)
			}
		} else {
			preferred = provider.ProviderCPU
		}

		providerCfg := ml.ProviderConfig{
			Preferred:     preferred,
			ForceCPU:      cfg.ForceCPU,
			DeviceID:      cfg.GPUDeviceID,
			AllowFallback: true,
		}

		sessions := cfg.Workers
		if cfg.GPU && !cfg.ForceCPU {
			sessions = cfg.GPUDetSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]ml.DetectorRepository, sessions)
		for i := 0; i < sessions; i++ {
			modelPath := filepath.Join(modelsDir, "det_10g.onnx")
			det, err := ml.NewDetectorRepository(ml.DetectorConfig{
				ModelPath: modelPath,
				Provider:  providerCfg,
				DetThresh: float32(cfg.DetThresh),
			})
			if err != nil {
				logger.Error(ctx, "failed to init detector repository",
					"index", i,
					"model_path", modelPath,
					"preferred_provider", string(providerCfg.Preferred),
					"force_cpu", providerCfg.ForceCPU,
					"device_id", providerCfg.DeviceID,
					"err", err,
				)
				// Не валим весь сервер: extraction вернёт ошибку при пустых пулах.
				for j := 0; j < i; j++ {
					if pool[j] != nil {
						pool[j].Close()
					}
				}
				return nil
			}
			pool[i] = det

			// Регистрация в graceful shutdown.
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
func (d *DiContainer) RecognizerPool(ctx context.Context) []ml.RecognizerRepository {
	if d.recognizerPool == nil {
		cfg := config.AppConfig.Extract
		modelsDir := config.AppConfig.Models.Dir

		// Determine preferred provider type.
		var preferred provider.ProviderType
		if cfg.GPU {
			preferred = provider.ProviderCUDA
			if cfg.ProviderPriority != "" && cfg.ProviderPriority != providerPriorityAuto {
				preferred = provider.ParseProviderType(cfg.ProviderPriority)
			}
		} else {
			preferred = provider.ProviderCPU
		}

		providerCfg := ml.ProviderConfig{
			Preferred:     preferred,
			ForceCPU:      cfg.ForceCPU,
			DeviceID:      cfg.GPUDeviceID,
			AllowFallback: true,
		}

		sessions := cfg.Workers
		if cfg.GPU && !cfg.ForceCPU {
			sessions = cfg.GPURecSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]ml.RecognizerRepository, sessions)
		for i := 0; i < sessions; i++ {
			modelPath := filepath.Join(modelsDir, "w600k_r50.onnx")
			rec, err := ml.NewRecognizerRepository(ml.RecognizerConfig{
				ModelPath: modelPath,
				Provider:  providerCfg,
			})
			if err != nil {
				logger.Error(ctx, "failed to init recognizer repository",
					"index", i,
					"model_path", modelPath,
					"preferred_provider", string(providerCfg.Preferred),
					"force_cpu", providerCfg.ForceCPU,
					"device_id", providerCfg.DeviceID,
					"err", err,
				)
				for j := 0; j < i; j++ {
					if pool[j] != nil {
						pool[j].Close()
					}
				}
				return nil
			}
			pool[i] = rec

			// Регистрация в graceful shutdown.
			closer.AddNamed(fmt.Sprintf("Recognizer %d", i), func(ctx context.Context) error {
				rec.Close()
				return nil
			})
		}
		d.recognizerPool = pool
	}
	return d.recognizerPool
}

// PersonRepository возвращает репозиторий персон.
func (d *DiContainer) PersonRepository() *postgres.PersonRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Persons
}

// FaceRepository возвращает репозиторий лиц.
func (d *DiContainer) FaceRepository() *postgres.FaceRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Faces
}

// PhotoRepository возвращает репозиторий фото.
func (d *DiContainer) PhotoRepository() *postgres.PhotoRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Photos
}

// RelationRepository возвращает репозиторий связей.
func (d *DiContainer) RelationRepository() *postgres.RelationRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Relations
}

// SessionRepository возвращает репозиторий сессий.
func (d *DiContainer) SessionRepository() *postgres.SessionRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Sessions
}
