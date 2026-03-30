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
	"github.com/kont1n/face-grouper/internal/report"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organization"
	"github.com/kont1n/face-grouper/platform/pkg/logger"
)

// Pipeline implements handler.PipelineRunner for async processing.
type Pipeline struct {
	di *diContainer
}

// NewPipeline creates a new Pipeline runner.
func NewPipeline(di *diContainer) *Pipeline {
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

	send := func(stage, label string, progress float64) {
		ch <- handler.ProgressEvent{
			SessionID:  sessionID,
			Stage:      stage,
			StageLabel: label,
			Progress:   progress,
			ElapsedMs:  time.Since(start).Milliseconds(),
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

	logFile, err := os.Create(filepath.Join(outputDir, "processing.log")) //nolint:gosec
	if err != nil {
		fail(fmt.Sprintf("cannot create log file: %v", err))
		return
	}
	defer func() { _ = logFile.Close() }()
	w := io.MultiWriter(os.Stdout, logFile)

	api := p.di.API(ctx)
	stageDurations := make(map[string]time.Duration)

	// --- Scan. ---
	send("scan", "Сканирование...", 0.05)
	stageStart := time.Now()
	files, err := api.Scan(ctx, inputDir)
	if err != nil {
		fail(fmt.Sprintf("scan error: %v", err))
		return
	}
	_, _ = fmt.Fprintf(w, "Found %d image(s)\n", len(files))
	stageDurations["scan"] = time.Since(stageStart)
	send("scan", "Сканирование...", 1.0)

	if len(files) == 0 {
		fail("no images found")
		return
	}

	// --- Thumbnails. ---
	_ = os.RemoveAll(thumbDir)
	_ = os.MkdirAll(thumbDir, 0o750)

	// --- Extract. ---
	send("extract", "Обнаружение лиц...", 0.05)
	stageStart = time.Now()

	extractResult, err := api.Extract(ctx, files, thumbDir, w)
	if err != nil {
		fail(fmt.Sprintf("extraction error: %v", err))
		return
	}
	_, _ = fmt.Fprintf(w, "Total faces: %d, errors: %d\n", len(extractResult.Faces), extractResult.ErrorCount)
	stageDurations["extract"] = time.Since(stageStart)
	send("extract", "Обнаружение лиц...", 1.0)

	if len(extractResult.Faces) == 0 {
		fail("no faces found")
		return
	}

	// --- Cluster. ---
	send("cluster", "Группировка...", 0.05)
	stageStart = time.Now()

	clusters, err := api.Cluster(ctx, extractResult.Faces, appCfg.Cluster.Threshold)
	if err != nil {
		fail(fmt.Sprintf("clustering error: %v", err))
		return
	}
	_, _ = fmt.Fprintf(w, "Found %d person(s)\n", len(clusters))
	stageDurations["cluster"] = time.Since(stageStart)
	send("cluster", "Группировка...", 1.0)

	// --- Organize. ---
	send("organize", "Организация результатов...", 0.05)
	stageStart = time.Now()

	persons, err := api.Organize(ctx, clusters, outputDir, appCfg.Organizer.AvatarUpdateThreshold, w)
	if err != nil {
		fail(fmt.Sprintf("organizer error: %v", err))
		return
	}
	stageDurations["organize"] = time.Since(stageStart)
	send("organize", "Организация результатов...", 1.0)

	// --- Report. ---
	rpt := buildReport(start, appCfg, len(files), extractResult, persons)
	if err := report.Save(rpt, outputDir); err != nil {
		logger.Warn(ctx, "cannot save report", zap.Error(err))
	}

	finish()
}

func buildReport(start time.Time, cfg *config.Config, totalImages int, extractResult *extraction.ExtractionResult, persons []organization.PersonInfo) *report.Report {
	rpt := &report.Report{
		StartedAt:    start,
		InputDir:     cfg.App.InputDir,
		OutputDir:    cfg.App.OutputDir,
		TotalImages:  totalImages,
		TotalFaces:   len(extractResult.Faces),
		TotalPersons: len(persons),
		Errors:       extractResult.ErrorCount,
		FileErrors:   extractResult.FileErrors,
		Threshold:    cfg.Cluster.Threshold,
		GPU:          cfg.Extract.GPU,
	}

	for _, p := range persons {
		rpt.Persons = append(rpt.Persons, report.PersonReport{
			ID:           p.ID,
			PhotoCount:   p.PhotoCount,
			FaceCount:    p.FaceCount,
			Thumbnail:    p.Thumbnail,
			AvatarPath:   p.AvatarPath,
			QualityScore: p.QualityScore,
			Photos:       p.Photos,
		})
	}

	rpt.FinishedAt = time.Now()
	rpt.Duration = time.Since(start).Round(time.Millisecond).String()

	return rpt
}
