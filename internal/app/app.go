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
	"github.com/kont1n/face-grouper/internal/config/env"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml"
	"github.com/kont1n/face-grouper/internal/infrastructure/ml/provider"
	"github.com/kont1n/face-grouper/internal/repository/database"
	"github.com/kont1n/face-grouper/internal/service/clustering"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organizer"
	"github.com/kont1n/face-grouper/internal/service/report"
	"github.com/kont1n/face-grouper/internal/web"
	"github.com/kont1n/face-grouper/platform/pkg/closer"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

const providerPriorityAuto = "auto"

// buildProviderConfig builds a unified provider configuration from extraction config.
func buildProviderConfig(cfg env.ExtractConfig, logSelection bool) ml.ProviderConfig {
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

	return ml.ProviderConfig{
		Preferred:     preferred,
		ForceCPU:      cfg.ForceCPU,
		DeviceID:      cfg.GPUDeviceID,
		AllowFallback: true,
		LogSelection:  logSelection,
	}
}

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

	providerCfg := buildProviderConfig(cfg, true)

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
	requestedProvider, fallback, fallbackReason := ml.GetProviderDiagnostics()
	logger.Info(ctx, "ONNX Runtime provider initialized",
		zap.String("provider", selectedProvider.Name),
		zap.String("type", string(selectedProvider.Type)),
		zap.Int("device_id", selectedProvider.DeviceID),
		zap.String("requested_provider", string(requestedProvider)),
		zap.Bool("fallback", fallback),
		zap.String("fallback_reason", fallbackReason),
	)

	api := a.diContainer.API(ctx)
	appCfg := config.AppConfig
	clustering.SetRefineFactor(appCfg.Cluster.RefineFactor)
	clustering.SetTwoStageConfig(
		appCfg.Cluster.EnableTwoStage,
		appCfg.Cluster.PreclusterThreshold,
		appCfg.Cluster.CentroidMergeThreshold,
		appCfg.Cluster.MutualK,
	)
	clustering.SetAmbiguityGateConfig(
		appCfg.Cluster.EnableAmbiguityGate,
		appCfg.Cluster.AmbiguityTopK,
		appCfg.Cluster.AmbiguityMeanMin,
		appCfg.Cluster.AmbiguityMeanMax,
		appCfg.Cluster.AmbiguityCentroidMax,
	)

	outputDir := appCfg.App.OutputDir
	thumbDir := filepath.Join(outputDir, ".thumbnails")

	// Create output directory.
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("cannot create output dir: %w", err)
	}

	// Create log file.
	logFile, err := os.OpenFile(filepath.Join(outputDir, "processing.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("cannot create log file: %w", err)
	}
	defer func() { _ = logFile.Close() }()
	w := io.MultiWriter(os.Stdout, logFile)

	// Create pipeline context.
	pc := NewPipelineContext(appCfg, outputDir, thumbDir, w)

	// Build and execute pipeline.
	pipeline := NewProcessingPipeline(
		NewScanStep(api, appCfg.App.InputDir),
		NewThumbnailsStep(),
		NewExtractStep(api, thumbDir, w),
		NewClusterStep(api, appCfg.Cluster.Threshold),
		NewOrganizeStep(api, outputDir, appCfg.Organizer.AvatarUpdateThreshold, w),
		NewReportStep(appCfg),
	)

	if appCfg.Web.Serve {
		_, _ = fmt.Fprintf(w, "\n=== Starting web UI ===\n")
		// Run processing in background so UI becomes available immediately.
		go func() {
			if err := pipeline.Execute(ctx, pc); err != nil {
				logger.Error(ctx, "processing pipeline failed", zap.Error(err))
				return
			}

			// Print summary after pipeline completion.
			a.printSummary(w, pc.Report, pc.StageDurations)
		}()

		return a.runWebUI(ctx, outputDir, appCfg.Web.Port)
	}

	if err := pipeline.Execute(ctx, pc); err != nil {
		return err
	}

	// Print summary.
	a.printSummary(w, pc.Report, pc.StageDurations)

	_, _ = fmt.Fprintf(w, "\nTip: run with --serve to view results in browser, or --view to view previous results\n")
	return nil
}

// buildReportFromResults creates a report from extraction and organization results.
func buildReportFromResults(start time.Time, cfg *config.Config, totalImages int, extractResult *extraction.ExtractionResult, persons []organizer.PersonInfo) *report.Report {
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
		Diagnostics: report.AnalyzeClusters(extractResult.Clusters, cfg.Cluster.Threshold, cfg.Cluster.RefineFactor, 5),
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
	_, _ = fmt.Fprintf(w, "Organize:       %s\n", stageDurations["organize"].Round(time.Millisecond))
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
