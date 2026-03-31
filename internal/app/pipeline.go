package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/kont1n/face-grouper/internal/api/http/handler"
	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/service/report"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// Pipeline implements handler.PipelineRunner for async processing.
type Pipeline struct {
	di *DiContainer
}

// NewPipeline creates a new Pipeline runner.
func NewPipeline(di *DiContainer) *Pipeline {
	return &Pipeline{di: di}
}

// RunPipeline starts the face processing pipeline asynchronously.
func (p *Pipeline) RunPipeline(ctx context.Context, sessionID, inputDir string) (<-chan handler.ProgressEvent, error) {
	ch := make(chan handler.ProgressEvent, 100)

	go func() {
		defer close(ch)
		p.run(ctx, sessionID, inputDir, ch)
	}()

	return ch, nil
}

func (p *Pipeline) run(ctx context.Context, sessionID, inputDir string, ch chan<- handler.ProgressEvent) {
	start := time.Now()
	appCfg := config.AppConfig
	outputDir := appCfg.App.OutputDir
	thumbDir := filepath.Join(outputDir, ".thumbnails")

	// Stage tracking for ETA calculation.
	stageTimes := make(map[string]time.Time)
	totalProgress := 0.0
	var totalItems, processedItems int
	var currentFile string

	send := func(stage, label string, progress float64) {
		now := time.Now()

		// Track stage start.
		if _, started := stageTimes[stage]; !started {
			stageTimes[stage] = now
		}

		// Update total progress (each stage is 25% of total).
		stageWeights := map[string]float64{
			"scan":     0.25,
			"extract":  0.25,
			"cluster":  0.25,
			"organize": 0.25,
		}

		baseProgress := map[string]float64{
			"scan":     0.0,
			"extract":  0.25,
			"cluster":  0.50,
			"organize": 0.75,
		}

		totalProgress = baseProgress[stage] + (progress * stageWeights[stage])

		// Calculate ETA.
		elapsed := time.Since(start)
		var estimatedTotal, eta time.Duration

		if totalProgress > 0.05 { // Only calculate if we have enough progress (>5%).
			estimatedTotal = time.Duration(float64(elapsed) / totalProgress)
			eta = estimatedTotal - elapsed
		}

		ch <- handler.ProgressEvent{
			SessionID:      sessionID,
			Stage:          stage,
			StageLabel:     label,
			Progress:       totalProgress,
			ProcessedItems: processedItems,
			TotalItems:     totalItems,
			CurrentFile:    currentFile,
			ElapsedMs:      elapsed.Milliseconds(),
			EstimatedMs:    estimatedTotal.Milliseconds(),
			ETAMs:          eta.Milliseconds(),
		}
	}

	fail := func(errMsg string) {
		ch <- handler.ProgressEvent{
			SessionID: sessionID,
			Done:      true,
			Error:     errMsg,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	finish := func() {
		ch <- handler.ProgressEvent{
			SessionID: sessionID,
			Done:      true,
			Progress:  1.0,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
	}

	// Create output dirs.
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		fail(fmt.Sprintf("cannot create output dir: %v", err))
		return
	}

	logFile, err := os.OpenFile(filepath.Join(outputDir, "processing.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec
	if err != nil {
		fail(fmt.Sprintf("cannot create log file: %v", err))
		return
	}
	defer func() { _ = logFile.Close() }()
	w := io.MultiWriter(os.Stdout, logFile)

	api := p.di.API(ctx)

	// --- Scan. ---.
	send("scan", "Сканирование...", 0.05)
	files, scanErr := api.Scan(ctx, inputDir)
	if scanErr != nil {
		fail(fmt.Sprintf("scan error: %v", scanErr))
		return
	}
	totalItems = len(files)
	processedItems = 0
	send("scan", "Сканирование...", 1.0)

	// --- Thumbnails. ---.
	_ = os.RemoveAll(thumbDir)
	_ = os.MkdirAll(thumbDir, 0o750)

	// --- Extract. ---.
	send("extract", "Обнаружение лиц...", 0.05)

	extractResult, err := api.Extract(ctx, files, thumbDir, w)
	if err != nil {
		fail(fmt.Sprintf("extraction error: %v", err))
		return
	}
	processedItems = len(files)
	totalItems = len(files)
	_, _ = fmt.Fprintf(w, "Total faces: %d, errors: %d\n", len(extractResult.Faces), extractResult.ErrorCount)
	send("extract", "Обнаружение лиц...", 1.0)

	if len(extractResult.Faces) == 0 {
		fail("no faces found")
		return
	}

	// --- Cluster. ---.
	send("cluster", "Группировка...", 0.05)

	clusters, err := api.Cluster(ctx, extractResult.Faces, appCfg.Cluster.Threshold)
	if err != nil {
		fail(fmt.Sprintf("clustering error: %v", err))
		return
	}
	processedItems = len(extractResult.Faces)
	totalItems = len(extractResult.Faces)
	_, _ = fmt.Fprintf(w, "Found %d person(s)\n", len(clusters))
	send("cluster", "Группировка...", 1.0)

	// --- Organize. ---.
	send("organize", "Организация результатов...", 0.05)

	persons, err := api.Organize(ctx, clusters, outputDir, appCfg.Organizer.AvatarUpdateThreshold, w)
	if err != nil {
		fail(fmt.Sprintf("organizer error: %v", err))
		return
	}
	processedItems = len(persons)
	totalItems = len(persons)
	send("organize", "Организация результатов...", 1.0)

	// --- Report. ---.
	rpt := buildReportFromResults(start, appCfg, len(files), extractResult, persons)
	if err := report.Save(rpt, outputDir); err != nil {
		logger.Warn(ctx, "cannot save report", zap.Error(err))
	}

	finish()
}
