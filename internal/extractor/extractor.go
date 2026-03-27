package extractor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/kont1n/face-grouper/internal/models"
)

// Config holds extractor settings.
type Config struct {
	PythonBin  string
	ScriptPath string
	Workers    int
	GPU        bool
	ThumbDir   string
	MaxDim     int
	GPUWorkers int
}

// Result aggregates extraction output and error statistics.
type Result struct {
	Faces      []models.Face
	ErrorCount int
	FileErrors map[string]string
}

// Extract runs the Python face extractor on all files.
// GPU mode uses a single persistent Python process (batch stdin/stdout streaming).
// CPU mode uses a parallel worker pool.
func Extract(files []string, cfg Config, w io.Writer) (*Result, error) {
	res := &Result{FileErrors: make(map[string]string)}

	if cfg.GPU {
		if err := extractBatch(files, cfg, w, res); err != nil {
			return res, err
		}
	} else {
		extractParallel(files, cfg, w, res)
	}

	return res, nil
}

// extractBatch launches one or more Python batch processes.
// Multiple processes allow overlapping CPU preprocessing with GPU inference.
func extractBatch(files []string, cfg Config, w io.Writer, res *Result) error {
	nWorkers := cfg.GPUWorkers
	if nWorkers <= 0 {
		nWorkers = 1
	}
	if nWorkers > len(files) {
		nWorkers = len(files)
	}

	args := []string{cfg.ScriptPath, "--batch"}
	if cfg.GPU {
		args = append(args, "--gpu")
	}
	if cfg.ThumbDir != "" {
		args = append(args, "--thumb-dir", cfg.ThumbDir)
	}
	if cfg.MaxDim > 0 {
		args = append(args, "--max-dim", fmt.Sprintf("%d", cfg.MaxDim))
	}

	chunks := make([][]string, nWorkers)
	for i, f := range files {
		chunks[i%nWorkers] = append(chunks[i%nWorkers], f)
	}

	var (
		mu        sync.Mutex
		processed int
		total     = len(files)
		errs      []error
	)

	var wg sync.WaitGroup
	for wi := 0; wi < nWorkers; wi++ {
		chunk := chunks[wi]
		if len(chunk) == 0 {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runBatchWorker(chunk, cfg.PythonBin, args, w, res, total, &mu, &processed); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func runBatchWorker(files []string, pythonBin string, args []string, w io.Writer, res *Result, total int, mu *sync.Mutex, processed *int) error {
	cmd := exec.Command(pythonBin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start python: %w, stderr: %s", err, stderr.String())
	}

	go func() {
		defer stdin.Close()
		for _, f := range files {
			fmt.Fprintln(stdin, f)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var result models.ExtractionResult
		if err := json.Unmarshal(line, &result); err != nil {
			raw := string(line)
			if len(raw) > 200 {
				raw = raw[:200] + "..."
			}
			mu.Lock()
			fmt.Fprintf(w, "WARN: skipping non-JSON output from python: %s\n", raw)
			mu.Unlock()
			continue
		}

		mu.Lock()
		*processed++
		p := *processed

		if result.Error != "" {
			fmt.Fprintf(w, "[%d/%d] ERROR %s: %s\n", p, total, result.File, result.Error)
			res.FileErrors[result.File] = result.Error
			res.ErrorCount++
		} else {
			for i := range result.Faces {
				result.Faces[i].FilePath = result.File
			}
			res.Faces = append(res.Faces, result.Faces...)
			fmt.Fprintf(w, "[%d/%d] %s — found %d face(s)\n", p, total, result.File, len(result.Faces))
		}
		mu.Unlock()
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("python process: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

func extractParallel(files []string, cfg Config, w io.Writer, res *Result) {
	type fileResult struct {
		path  string
		faces []models.Face
		err   error
	}

	jobs := make(chan string, len(files))
	results := make(chan fileResult, len(files))

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				faces, err := extractOne(path, cfg)
				results <- fileResult{path: path, faces: faces, err: err}
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

	processed := 0
	total := len(files)
	for r := range results {
		processed++
		if r.err != nil {
			fmt.Fprintf(w, "[%d/%d] ERROR %s: %v\n", processed, total, r.path, r.err)
			res.FileErrors[r.path] = r.err.Error()
			res.ErrorCount++
			continue
		}
		fmt.Fprintf(w, "[%d/%d] %s — found %d face(s)\n", processed, total, r.path, len(r.faces))
		res.Faces = append(res.Faces, r.faces...)
	}
}

func extractOne(imagePath string, cfg Config) ([]models.Face, error) {
	args := []string{cfg.ScriptPath, imagePath}
	if cfg.ThumbDir != "" {
		args = append(args, "--thumb-dir", cfg.ThumbDir)
	}
	if cfg.MaxDim > 0 {
		args = append(args, "--max-dim", fmt.Sprintf("%d", cfg.MaxDim))
	}

	cmd := exec.Command(cfg.PythonBin, args...)
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
