package extractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"github.com/kont1n/face-grouper/internal/models"
)

// Config holds extractor settings.
type Config struct {
	PythonBin  string
	ScriptPath string
	Workers    int
}

type fileResult struct {
	FilePath string
	Faces    []models.Face
	Err      error
}

// Extract runs the Python face extractor on each file using a worker pool.
// Returns all detected faces across all files with their FilePath set.
func Extract(files []string, cfg Config) ([]models.Face, error) {
	jobs := make(chan string, len(files))
	results := make(chan fileResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				faces, err := extractOne(path, cfg)
				results <- fileResult{FilePath: path, Faces: faces, Err: err}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var allFaces []models.Face
	processed := 0
	total := len(files)
	for r := range results {
		processed++
		if r.Err != nil {
			fmt.Printf("[%d/%d] ERROR %s: %v\n", processed, total, r.FilePath, r.Err)
			continue
		}
		fmt.Printf("[%d/%d] %s — found %d face(s)\n", processed, total, r.FilePath, len(r.Faces))
		allFaces = append(allFaces, r.Faces...)
	}

	return allFaces, nil
}

func extractOne(imagePath string, cfg Config) ([]models.Face, error) {
	cmd := exec.Command(cfg.PythonBin, cfg.ScriptPath, imagePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python error: %w, stderr: %s", err, stderr.String())
	}

	var result models.ExtractionResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("json decode error: %w, raw: %s", err, stdout.String())
	}

	if result.Error != "" {
		return nil, fmt.Errorf("extraction error: %s", result.Error)
	}

	for i := range result.Faces {
		result.Faces[i].FilePath = imagePath
	}

	return result.Faces, nil
}
