// Package report provides report saving and loading functionality.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Report holds the results of a face grouping processing run.
type Report struct {
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at"`
	Duration     string            `json:"duration"`
	InputDir     string            `json:"input_dir"`
	OutputDir    string            `json:"output_dir"`
	TotalImages  int               `json:"total_images"`
	TotalFaces   int               `json:"total_faces"`
	TotalPersons int               `json:"total_persons"`
	Errors       int               `json:"errors"`
	FileErrors   map[string]string `json:"file_errors,omitempty"`
	Threshold    float64           `json:"threshold"`
	GPU          bool              `json:"gpu"`
	Persons      []PersonReport    `json:"persons"`
}

// PersonReport holds per-person metadata within a report.
type PersonReport struct {
	ID           int      `json:"id"`
	PhotoCount   int      `json:"photo_count"`
	FaceCount    int      `json:"face_count"`
	Thumbnail    string   `json:"thumbnail"`
	AvatarPath   string   `json:"avatar_path,omitempty"`
	QualityScore float64  `json:"quality_score,omitempty"`
	Photos       []string `json:"photos"`
}

// BuildParams contains parameters for building a report.
type BuildParams struct {
	StartedAt   time.Time
	InputDir    string
	OutputDir   string
	TotalImages int
	TotalFaces  int
	Errors      int
	FileErrors  map[string]string
	Threshold   float64
	GPU         bool
	Persons     []PersonBuildInfo
}

// PersonBuildInfo contains per-person data for report building.
type PersonBuildInfo struct {
	ID           int
	PhotoCount   int
	FaceCount    int
	Thumbnail    string
	AvatarPath   string
	QualityScore float64
	Photos       []string
}

// Build creates a Report from the given parameters.
func Build(params BuildParams) *Report {
	rpt := &Report{
		StartedAt:    params.StartedAt,
		InputDir:     params.InputDir,
		OutputDir:    params.OutputDir,
		TotalImages:  params.TotalImages,
		TotalFaces:   params.TotalFaces,
		TotalPersons: len(params.Persons),
		Errors:       params.Errors,
		FileErrors:   params.FileErrors,
		Threshold:    params.Threshold,
		GPU:          params.GPU,
	}

	for _, p := range params.Persons {
		rpt.Persons = append(rpt.Persons, PersonReport(p))
	}

	rpt.FinishedAt = time.Now()
	rpt.Duration = time.Since(params.StartedAt).Round(time.Millisecond).String()

	return rpt
}

// Save writes the report as JSON to the output directory.
func Save(r *Report, outputDir string) error {
	path := filepath.Join(outputDir, "report.json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) //nolint:gosec
}

// Load reads and returns the report from the output directory.
func Load(outputDir string) (*Report, error) {
	path := filepath.Join(outputDir, "report.json")
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
