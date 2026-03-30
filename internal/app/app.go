// Package app provides the main application logic.
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/database"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml/provider"
	"github.com/kont1n/face-grouper/internal/report"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organization"
	"github.com/kont1n/face-grouper/internal/web"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

const providerPriorityAuto = "auto"

// App представляет основное приложение.
type App struct {
	diContainer *DiContainer
}

// New создаёт новое приложение.
func New(ctx context.Context) (*App, error) {
	a := &App{}

	err := a.initDeps(ctx)
	if err != nil {
		return nil, err
	}

	return a, nil
}

// initDeps инициализирует зависимости.
func (a *App) initDeps(ctx context.Context) error {
	inits := []func(context.Context) error{
		a.initDI,
		a.initDatabase,
		a.initLogger,
		a.initCloser,
	}

	for _, f := range inits {
		if err := f(ctx); err != nil {
			return err
		}
	}

	return nil
}

// initDI инициализирует DI контейнер.
func (a *App) initDI(_ context.Context) error {
	a.diContainer = NewDiContainer(config.AppConfig)
	return nil
}

// initDatabase инициализирует базу данных.
func (a *App) initDatabase(ctx context.Context) error {
	cfg := config.AppConfig.Database

	db, err := database.New(ctx, cfg)
	if err != nil {
		// Database is optional for now, log warning and continue.
		logger.Warn(ctx, "database initialization failed (running without database)",
			zap.Error(err),
		)
		return nil
	}

	a.diContainer.SetDatabase(db)

	// Register database close.
	closer.AddNamed("database", func(_ context.Context) error {
		db.Close()
		return nil
	})

	// Log database health.
	health, err := db.Health(ctx)
	if err != nil {
		logger.Warn(ctx, "database health check failed", zap.Error(err))
	} else {
		logger.Info(ctx, "database connected",
			zap.String("version", health.Version),
			zap.Int32("connections", health.Connections),
			zap.Strings("extensions", health.Extensions),
		)
	}

	return nil
}

// initLogger инициализирует логгер.
func (a *App) initLogger(_ context.Context) error {
	cfg := config.AppConfig.Logger
	return logger.Init(cfg.Level, cfg.AsJSON)
}

// initCloser настраивает graceful shutdown.
func (a *App) initCloser(_ context.Context) error {
	closer.SetLogger(logger.Logger())
	return nil
}

// Run запускает основное приложение.
func (a *App) Run(ctx context.Context, viewOnly bool) error {
	if viewOnly {
		return a.runViewOnly(ctx)
	}
	return a.runProcess(ctx)
}

// runViewOnly запускает только веб-интерфейс для просмотра предыдущих результатов.
func (a *App) runViewOnly(ctx context.Context) error {
	outputDir := config.AppConfig.App.OutputDir
	port := config.AppConfig.Web.Port

	logger.Info(ctx, "starting web UI (view only)",
		zap.String("output_dir", outputDir),
		zap.Int("port", port),
	)

	return a.runWebUI(ctx, outputDir, port)
}

// runProcess запускает полный пайплайн обработки.
func (a *App) runProcess(ctx context.Context) error {
	// Select and initialize ONNX Runtime provider.
	cfg := config.AppConfig.Extract

	// Determine preferred provider type.
	var preferred provider.ProviderType
	if cfg.GPU {
		preferred = provider.ProviderCUDA // Default to CUDA for GPU.
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
		LogSelection:  true,
	}

	// Determine library path.
	var ortLibPath string
	if runtime.GOOS == "windows" && cfg.GPU && !cfg.ForceCPU {
		// Try GPU path first.
		ortLibPath = "runtime/onnxruntime-win-x64-gpu-1.23.0/lib/onnxruntime.dll"
	}

	if err := ml.SelectAndInitializeProvider(providerCfg, ortLibPath); err != nil {
		return fmt.Errorf("ONNX Runtime init: %w", err)
	}
	defer ml.DestroyORT()

	// Log selected provider.
	selectedProvider := ml.GetSelectedProvider()
	logger.Info(ctx, "ONNX Runtime provider initialized",
		zap.String("provider", selectedProvider.Name),
		zap.String("type", string(selectedProvider.Type)),
		zap.Int("device_id", selectedProvider.DeviceID),
	)

	api := a.diContainer.API(ctx)
	appCfg := config.AppConfig

	outputDir := appCfg.App.OutputDir
	thumbDir := filepath.Join(outputDir, ".thumbnails")

	// Создаём директорию вывода.
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("cannot create output dir: %w", err)
	}

	// Создаём лог-файл.
	logFile, err := os.Create(filepath.Join(outputDir, "processing.log")) //nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot create log file: %w", err)
	}
	defer func() { _ = logFile.Close() }()
	w := io.MultiWriter(os.Stdout, logFile)

	start := time.Now()
	stageDurations := make(map[string]time.Duration)

	// --- Scan. ---.
	stageStart := time.Now()
	_, _ = fmt.Fprintf(w, "=== Scanning directory ===\n")
	files, err := api.Scan(ctx, appCfg.App.InputDir)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	_, _ = fmt.Fprintf(w, "Found %d image(s)\n\n", len(files))
	stageDurations["scan"] = time.Since(stageStart)

	if len(files) == 0 {
		_, _ = fmt.Fprintf(w, "No images found, nothing to do.\n")
		return nil
	}

	// --- Thumbnails dir. ---.
	err = os.RemoveAll(thumbDir)
	if err != nil {
		return fmt.Errorf("cannot clean thumbnails dir: %w", err)
	}
	err = os.MkdirAll(thumbDir, 0o750)
	if err != nil {
		return fmt.Errorf("cannot create thumbnails dir: %w", err)
	}

	// --- Extract. ---.
	stageStart = time.Now()
	_, _ = fmt.Fprintf(w, "=== Extracting face embeddings ===\n")
	_, _ = fmt.Fprintf(w, "Mode: %s, %d worker(s)\n", selectedProvider.Name, appCfg.Extract.Workers)
	if appCfg.Extract.MaxDim > 0 {
		_, _ = fmt.Fprintf(w, "Pre-resize: max %dpx\n", appCfg.Extract.MaxDim)
	}

	extractResult, err := api.Extract(ctx, files, thumbDir, w)
	if err != nil {
		return fmt.Errorf("extraction error: %w", err)
	}
	_, _ = fmt.Fprintf(w, "\nTotal faces detected: %d (errors: %d)\n\n", len(extractResult.Faces), extractResult.ErrorCount)
	stageDurations["extract"] = time.Since(stageStart)

	if len(extractResult.Faces) == 0 {
		_, _ = fmt.Fprintf(w, "No faces found, nothing to group.\n")
		return nil
	}

	// --- Cluster. ---.
	stageStart = time.Now()
	_, _ = fmt.Fprintf(w, "=== Clustering faces ===\n")
	clusters, err := api.Cluster(ctx, extractResult.Faces, appCfg.Cluster.Threshold)
	if err != nil {
		return fmt.Errorf("clustering error: %w", err)
	}
	_, _ = fmt.Fprintf(w, "Found %d person(s)\n\n", len(clusters))
	stageDurations["cluster"] = time.Since(stageStart)

	// --- Organize. ---.
	stageStart = time.Now()
	_, _ = fmt.Fprintf(w, "=== Organizing output ===\n")
	persons, err := api.Organize(ctx, clusters, outputDir, appCfg.Organizer.AvatarUpdateThreshold, w)
	if err != nil {
		return fmt.Errorf("organizer error: %w", err)
	}
	stageDurations["organize_avatar"] = time.Since(stageStart)

	// --- Build report. ---.
	rpt := buildReportFromResults(start, appCfg, len(files), extractResult, persons)

	if err := report.Save(rpt, outputDir); err != nil {
		_, _ = fmt.Fprintf(w, "WARNING: cannot save report: %v\n", err)
	}

	// --- Summary. ---.
	a.printSummary(w, rpt, stageDurations)

	// --- Web UI. ---.
	if appCfg.Web.Serve {
		_, _ = fmt.Fprintf(w, "\n=== Starting web UI ===\n")
		return a.runWebUI(ctx, outputDir, appCfg.Web.Port)
	}

	_, _ = fmt.Fprintf(w, "\nTip: run with --serve to view results in browser, or --view to view previous results\n")
	return nil
}

// buildReportFromResults creates a report from extraction and organization results.
func buildReportFromResults(start time.Time, cfg *config.Config, totalImages int, extractResult *extraction.ExtractionResult, persons []organization.PersonInfo) *report.Report {
	reportPersons := make([]report.PersonBuildInfo, len(persons))
	for i, p := range persons {
		reportPersons[i] = report.PersonBuildInfo{
			ID:           p.ID,
			PhotoCount:   p.PhotoCount,
			FaceCount:    p.FaceCount,
			Thumbnail:    p.Thumbnail,
			AvatarPath:   p.AvatarPath,
			QualityScore: p.QualityScore,
			Photos:       p.Photos,
		}
	}

	return report.Build(report.BuildParams{
		StartedAt:   start,
		InputDir:    cfg.App.InputDir,
		OutputDir:   cfg.App.OutputDir,
		TotalImages: totalImages,
		TotalFaces:  len(extractResult.Faces),
		Errors:      extractResult.ErrorCount,
		FileErrors:  extractResult.FileErrors,
		Threshold:   cfg.Cluster.Threshold,
		GPU:         cfg.Extract.GPU,
		Persons:     reportPersons,
	})
}

func (a *App) printSummary(w io.Writer, rpt *report.Report, stageDurations map[string]time.Duration) {
	_, _ = fmt.Fprintf(w, "\n=== Summary ===\n")
	_, _ = fmt.Fprintf(w, "Images:  %d\n", rpt.TotalImages)
	_, _ = fmt.Fprintf(w, "Faces:   %d\n", rpt.TotalFaces)
	_, _ = fmt.Fprintf(w, "Persons: %d\n", rpt.TotalPersons)
	_, _ = fmt.Fprintf(w, "Errors:  %d\n", rpt.Errors)
	_, _ = fmt.Fprintf(w, "Time:    %s\n", rpt.Duration)
	_, _ = fmt.Fprintf(w, "\n=== Stage timings ===\n")
	_, _ = fmt.Fprintf(w, "Scan:           %s\n", stageDurations["scan"].Round(time.Millisecond))
	_, _ = fmt.Fprintf(w, "Extract:        %s\n", stageDurations["extract"].Round(time.Millisecond))
	_, _ = fmt.Fprintf(w, "Cluster:        %s\n", stageDurations["cluster"].Round(time.Millisecond))
	_, _ = fmt.Fprintf(w, "OrganizeAvatar: %s\n", stageDurations["organize_avatar"].Round(time.Millisecond))
	_, _ = fmt.Fprintf(w, "Report:  %s\n", filepath.Join(rpt.OutputDir, "report.json"))
	_, _ = fmt.Fprintf(w, "Log:     %s\n", filepath.Join(rpt.OutputDir, "processing.log"))
}

func (a *App) runWebUI(ctx context.Context, outputDir string, port int) error {
	logger.Info(ctx, "starting web UI",
		zap.String("output_dir", outputDir),
		zap.Int("port", port),
	)

	pipeline := NewPipeline(a.diContainer)

	srv := web.NewServer(web.ServerConfig{
		Port:      port,
		OutputDir: outputDir,
		DB:        a.diContainer.db,
	}, pipeline)

	return srv.ListenAndServeContext(ctx)
}
