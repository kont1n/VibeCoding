package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Report struct {
	StartedAt    time.Time      `json:"started_at"`
	FinishedAt   time.Time      `json:"finished_at"`
	Duration     string         `json:"duration"`
	InputDir     string         `json:"input_dir"`
	OutputDir    string         `json:"output_dir"`
	TotalImages  int            `json:"total_images"`
	TotalFaces   int            `json:"total_faces"`
	TotalPersons int            `json:"total_persons"`
	Errors       int            `json:"errors"`
	Threshold    float64        `json:"threshold"`
	GPU          bool           `json:"gpu"`
	Persons      []PersonReport `json:"persons"`
}

type PersonReport struct {
	ID          int      `json:"id"`
	PhotoCount  int      `json:"photo_count"`
	FaceCount   int      `json:"face_count"`
	Thumbnail   string   `json:"thumbnail"`
	Description string   `json:"description,omitempty"`
	Photos      []string `json:"photos"`
}

func Save(r *Report, outputDir string) error {
	path := filepath.Join(outputDir, "report.json")
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Load(outputDir string) (*Report, error) {
	path := filepath.Join(outputDir, "report.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
