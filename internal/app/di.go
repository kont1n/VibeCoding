package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/kont1n/face-grouper/internal/api/cli"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/repository/database"
	"github.com/kont1n/face-grouper/internal/repository/filesystem"
	"github.com/kont1n/face-grouper/internal/repository/postgres"
	"github.com/kont1n/face-grouper/internal/service/clustering"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organizer"
	"github.com/kont1n/face-grouper/internal/service/scan"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// DiContainer управляет зависимостями приложения с lazy initialization.
type DiContainer struct {
	mu  sync.Mutex
	cfg *config.Config

	api *cli.API
	db  *database.DB

	scanService       scan.ScanService
	extractionService extraction.ExtractionService
	clusterService    clustering.ClusterService
	organizeService   *organizer.Organizer

	scannerRepo    filesystem.ScannerRepository
	detectorPool   []ml.DetectorGateway
	recognizerPool []ml.RecognizerGateway
}

// SetDatabase устанавливает соединение с базой данных.
func (d *DiContainer) SetDatabase(db *database.DB) {
	d.db = db
}

// NewDiContainer создаёт новый DI контейнер.
func NewDiContainer(cfg *config.Config) *DiContainer {
	return &DiContainer{cfg: cfg}
}

// API возвращает CLI API, инициализируя при необходимости.
func (d *DiContainer) API(ctx context.Context) *cli.API {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.api == nil {
		d.api = cli.NewAPI(
			d.scanServiceLocked(ctx),
			d.extractionServiceLocked(ctx),
			d.clusterServiceLocked(ctx),
			d.organizeServiceLocked(ctx),
		)
	}
	return d.api
}

// ScanService возвращает сервис сканирования.
func (d *DiContainer) ScanService(ctx context.Context) scan.ScanService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scanServiceLocked(ctx)
}

func (d *DiContainer) scanServiceLocked(_ context.Context) scan.ScanService {
	if d.scanService == nil {
		d.scanService = scan.NewScanService(d.scannerRepositoryLocked())
	}
	return d.scanService
}

// ExtractionService возвращает сервис экстракции.
func (d *DiContainer) ExtractionService(ctx context.Context) extraction.ExtractionService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.extractionServiceLocked(ctx)
}

func (d *DiContainer) extractionServiceLocked(ctx context.Context) extraction.ExtractionService {
	if d.extractionService == nil {
		d.extractionService = extraction.NewExtractionService(
			d.cfg.Extract,
			d.detectorPoolLocked(ctx),
			d.recognizerPoolLocked(ctx),
		)
	}
	return d.extractionService
}

// ClusterService возвращает сервис кластеризации.
func (d *DiContainer) ClusterService(ctx context.Context) clustering.ClusterService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.clusterServiceLocked(ctx)
}

func (d *DiContainer) clusterServiceLocked(_ context.Context) clustering.ClusterService {
	if d.clusterService == nil {
		d.clusterService = clustering.NewClusterService()
	}
	return d.clusterService
}

// OrganizeService возвращает сервис организации.
func (d *DiContainer) OrganizeService(ctx context.Context) *organizer.Organizer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.organizeServiceLocked(ctx)
}

func (d *DiContainer) organizeServiceLocked(_ context.Context) *organizer.Organizer {
	if d.organizeService == nil {
		d.organizeService = organizer.NewOrganizer()
	}
	return d.organizeService
}

// ScannerRepository возвращает репозиторий сканирования.
func (d *DiContainer) ScannerRepository() filesystem.ScannerRepository {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.scannerRepositoryLocked()
}

func (d *DiContainer) scannerRepositoryLocked() filesystem.ScannerRepository {
	if d.scannerRepo == nil {
		d.scannerRepo = filesystem.NewScannerRepository()
	}
	return d.scannerRepo
}

// DetectorPool возвращает пул детекторов.
func (d *DiContainer) DetectorPool(ctx context.Context) []ml.DetectorGateway {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.detectorPoolLocked(ctx)
}

func (d *DiContainer) detectorPoolLocked(ctx context.Context) []ml.DetectorGateway {
	if d.detectorPool == nil {
		cfg := d.cfg.Extract
		modelsDir := d.cfg.Models.Dir

		providerCfg := buildProviderConfig(cfg, false)

		sessions := cfg.Workers
		if cfg.GPU && !cfg.ForceCPU {
			sessions = cfg.GPUDetSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]ml.DetectorGateway, sessions)
		for i := 0; i < sessions; i++ {
			modelPath := filepath.Join(modelsDir, "det_10g.onnx")
			det, err := ml.NewDetectorGateway(ml.DetectorConfig{
				ModelPath: modelPath,
				Provider:  providerCfg,
				InputSize: cfg.DetInputSize,
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
				// Don't crash the server: extraction will return error on empty pools.
				for j := 0; j < i; j++ {
					if pool[j] != nil {
						pool[j].Close()
					}
				}
				return nil
			}
			pool[i] = det

			// Register in graceful shutdown.
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
func (d *DiContainer) RecognizerPool(ctx context.Context) []ml.RecognizerGateway {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.recognizerPoolLocked(ctx)
}

func (d *DiContainer) recognizerPoolLocked(ctx context.Context) []ml.RecognizerGateway {
	if d.recognizerPool == nil {
		cfg := d.cfg.Extract
		modelsDir := d.cfg.Models.Dir

		providerCfg := buildProviderConfig(cfg, false)

		sessions := cfg.Workers
		if cfg.GPU && !cfg.ForceCPU {
			sessions = cfg.GPURecSessions
		}
		if sessions <= 0 {
			sessions = 1
		}

		pool := make([]ml.RecognizerGateway, sessions)
		for i := 0; i < sessions; i++ {
			modelPath := filepath.Join(modelsDir, "w600k_r50.onnx")
			rec, err := ml.NewRecognizerGateway(ml.RecognizerConfig{
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

			// Register in graceful shutdown.
			closer.AddNamed(fmt.Sprintf("Recognizer %d", i), func(ctx context.Context) error {
				rec.Close()
				return nil
			})
		}
		d.recognizerPool = pool
	}
	return d.recognizerPool
}

// PersonRepository returns the person repository.
func (d *DiContainer) PersonRepository() postgres.PersonRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Persons
}

// FaceRepository returns the face repository.
func (d *DiContainer) FaceRepository() postgres.FaceRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Faces
}

// PhotoRepository returns the photo repository.
func (d *DiContainer) PhotoRepository() postgres.PhotoRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Photos
}

// RelationRepository returns the relation repository.
func (d *DiContainer) RelationRepository() postgres.RelationRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Relations
}

// SessionRepository returns the session repository.
func (d *DiContainer) SessionRepository() postgres.SessionRepository {
	if d.db == nil {
		return nil
	}
	return d.db.Sessions
}
