package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kont1n/face-grouper/internal/config"
	"github.com/kont1n/face-grouper/internal/model"
	"github.com/kont1n/face-grouper/internal/service/extraction"
	"github.com/kont1n/face-grouper/internal/service/organizer"
	"github.com/kont1n/face-grouper/internal/service/report"
)

// PipelineAPI defines the API interface used by pipeline steps.
type PipelineAPI interface {
	Scan(ctx context.Context, inputDir string) ([]string, error)
	Extract(
		ctx context.Context,
		files []string,
		thumbDir string,
		w io.Writer,
		onProgress extraction.ProgressCallback,
	) (*extraction.ExtractionResult, error)
	Cluster(ctx context.Context, faces []model.Face, threshold float64) ([]model.Cluster, error)
	Organize(ctx context.Context, clusters []model.Cluster, outputDir string, avatarUpdateThreshold float64, w io.Writer) ([]organizer.PersonInfo, error)
}

// PipelineStep represents a single step in the processing pipeline.
type PipelineStep interface {
	// Execute runs the step and returns the result.
	Execute(ctx context.Context, pc *PipelineContext) error
	// Name returns the step name for logging.
	Name() string
}

// PipelineContext holds the shared state across pipeline steps.
type PipelineContext struct {
	Config         *config.Config
	Files          []string
	ThumbDir       string
	OutputDir      string
	ExtractResult  *extraction.ExtractionResult
	Persons        []organizer.PersonInfo
	Report         *report.Report
	StageDurations map[string]time.Duration
	Writer         io.Writer
	StartTime      time.Time
}

// NewPipelineContext creates a new pipeline context.
func NewPipelineContext(cfg *config.Config, outputDir, thumbDir string, w io.Writer) *PipelineContext {
	return &PipelineContext{
		Config:         cfg,
		OutputDir:      outputDir,
		ThumbDir:       thumbDir,
		Writer:         w,
		StageDurations: make(map[string]time.Duration),
		StartTime:      time.Now(),
	}
}

// ScanStep scans the input directory for images.
type ScanStep struct {
	api      PipelineAPI
	inputDir string
}

// NewScanStep creates a new scan step.
func NewScanStep(api PipelineAPI, inputDir string) *ScanStep {
	return &ScanStep{api: api, inputDir: inputDir}
}

// Name returns the step name.
func (s *ScanStep) Name() string { return "scan" }

// Execute runs the scan step.
func (s *ScanStep) Execute(ctx context.Context, pc *PipelineContext) error {
	_, _ = fmt.Fprintf(pc.Writer, "=== Scanning directory ===\n")
	files, err := s.api.Scan(ctx, s.inputDir)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	_, _ = fmt.Fprintf(pc.Writer, "Found %d image(s)\n\n", len(files))
	pc.Files = files
	return nil
}

// ThumbnailsStep prepares the thumbnails directory.
type ThumbnailsStep struct{}

// NewThumbnailsStep creates a new thumbnails step.
func NewThumbnailsStep() *ThumbnailsStep { return &ThumbnailsStep{} }

// Name returns the step name.
func (s *ThumbnailsStep) Name() string { return "thumbnails" }

// Execute runs the thumbnails step.
func (s *ThumbnailsStep) Execute(ctx context.Context, pc *PipelineContext) error {
	if err := os.RemoveAll(pc.ThumbDir); err != nil {
		return fmt.Errorf("cannot clean thumbnails dir: %w", err)
	}
	return os.MkdirAll(pc.ThumbDir, 0o750)
}

// ExtractStep extracts face embeddings from images.
type ExtractStep struct {
	api      PipelineAPI
	thumbDir string
	writer   io.Writer
}

// NewExtractStep creates a new extract step.
func NewExtractStep(api PipelineAPI, thumbDir string, w io.Writer) *ExtractStep {
	return &ExtractStep{api: api, thumbDir: thumbDir, writer: w}
}

// Name returns the step name.
func (s *ExtractStep) Name() string { return "extract" }

// Execute runs the extract step.
func (s *ExtractStep) Execute(ctx context.Context, pc *PipelineContext) error {
	_, _ = fmt.Fprintf(pc.Writer, "=== Extracting face embeddings ===\n")
	result, err := s.api.Extract(ctx, pc.Files, s.thumbDir, pc.Writer, nil)
	if err != nil {
		return fmt.Errorf("extraction error: %w", err)
	}
	_, _ = fmt.Fprintf(pc.Writer, "Total faces detected: %d (errors: %d)\n\n", len(result.Faces), result.ErrorCount)
	pc.ExtractResult = result
	return nil
}

// ClusterStep clusters faces by similarity.
type ClusterStep struct {
	api       PipelineAPI
	threshold float64
}

// NewClusterStep creates a new cluster step.
func NewClusterStep(api PipelineAPI, threshold float64) *ClusterStep {
	return &ClusterStep{api: api, threshold: threshold}
}

// Name returns the step name.
func (s *ClusterStep) Name() string { return "cluster" }

// Execute runs the cluster step.
func (s *ClusterStep) Execute(ctx context.Context, pc *PipelineContext) error {
	_, _ = fmt.Fprintf(pc.Writer, "=== Clustering faces ===\n")
	if len(pc.ExtractResult.Faces) == 0 {
		_, _ = fmt.Fprintf(pc.Writer, "No faces found, nothing to group.\n")
		return nil
	}
	clusters, err := s.api.Cluster(ctx, pc.ExtractResult.Faces, s.threshold)
	if err != nil {
		return fmt.Errorf("clustering error: %w", err)
	}
	_, _ = fmt.Fprintf(pc.Writer, "Found %d person(s)\n\n", len(clusters))
	pc.ExtractResult.Clusters = clusters
	return nil
}

// OrganizeStep organizes the output files.
type OrganizeStep struct {
	api                   PipelineAPI
	outputDir             string
	avatarUpdateThreshold float64
	writer                io.Writer
}

// NewOrganizeStep creates a new organize step.
func NewOrganizeStep(api PipelineAPI, outputDir string, avatarUpdateThreshold float64, w io.Writer) *OrganizeStep {
	return &OrganizeStep{api: api, outputDir: outputDir, avatarUpdateThreshold: avatarUpdateThreshold, writer: w}
}

// Name returns the step name.
func (s *OrganizeStep) Name() string { return "organize" }

// Execute runs the organize step.
func (s *OrganizeStep) Execute(ctx context.Context, pc *PipelineContext) error {
	_, _ = fmt.Fprintf(pc.Writer, "=== Organizing output ===\n")
	persons, err := s.api.Organize(ctx, pc.ExtractResult.Clusters, s.outputDir, s.avatarUpdateThreshold, pc.Writer)
	if err != nil {
		return fmt.Errorf("organizer error: %w", err)
	}
	pc.Persons = persons
	return nil
}

// ReportStep generates the final report.
type ReportStep struct {
	config *config.Config
}

// NewReportStep creates a new report step.
func NewReportStep(cfg *config.Config) *ReportStep {
	return &ReportStep{config: cfg}
}

// Name returns the step name.
func (s *ReportStep) Name() string { return "report" }

// Execute runs the report step.
func (s *ReportStep) Execute(ctx context.Context, pc *PipelineContext) error {
	pc.Report = buildReportFromResults(pc.StartTime, s.config, len(pc.Files), pc.ExtractResult, pc.Persons)
	if err := report.Save(pc.Report, pc.OutputDir); err != nil {
		_, _ = fmt.Fprintf(pc.Writer, "WARNING: cannot save report: %v\n", err)
	}
	return nil
}

// ProcessingPipeline represents the full processing pipeline.
type ProcessingPipeline struct {
	steps []PipelineStep
}

// NewProcessingPipeline creates a new processing pipeline.
func NewProcessingPipeline(steps ...PipelineStep) *ProcessingPipeline {
	return &ProcessingPipeline{steps: steps}
}

// Execute runs all pipeline steps in sequence.
func (p *ProcessingPipeline) Execute(ctx context.Context, pc *PipelineContext) error {
	for _, step := range p.steps {
		stageStart := time.Now()
		if err := step.Execute(ctx, pc); err != nil {
			return err
		}
		pc.StageDurations[step.Name()] = time.Since(stageStart)
	}
	return nil
}
