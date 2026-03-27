package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kont1n/face-grouper/internal/clustering"
	"github.com/kont1n/face-grouper/internal/describer"
	"github.com/kont1n/face-grouper/internal/extractor"
	"github.com/kont1n/face-grouper/internal/inference"
	"github.com/kont1n/face-grouper/internal/organizer"
	"github.com/kont1n/face-grouper/internal/report"
	"github.com/kont1n/face-grouper/internal/scanner"
	"github.com/kont1n/face-grouper/internal/web"
)

type appConfig struct {
	LMStudio describer.Config `json:"lm_studio"`
}

func main() {
	inputDir := flag.String("input", "./dataset", "path to directory with photos")
	outputDir := flag.String("output", "./output", "path for grouped output")
	workers := flag.Int("workers", 4, "number of parallel extraction workers")
	threshold := flag.Float64("threshold", 0.5, "cosine similarity threshold for face grouping")
	gpu := flag.Bool("gpu", false, "use GPU (CUDA) for ONNX Runtime inference")
	maxDim := flag.Int("max-dim", 1920, "downscale images so longest side <= this value (0 = no resize)")
	serve := flag.Bool("serve", false, "start web UI after processing")
	port := flag.Int("port", 8080, "web UI port")
	describe := flag.Bool("describe", false, "generate person descriptions via LM Studio (Moondream2)")
	configPath := flag.String("config", "config.json", "path to config file")
	viewOnly := flag.Bool("view", false, "only start web UI without processing (requires previous output)")
	modelsDir := flag.String("models-dir", "./models", "path to directory with ONNX models (det_10g.onnx, w600k_r50.onnx)")
	ortLib := flag.String("ort-lib", "", "path to ONNX Runtime shared library (auto-detected if empty)")
	detThresh := flag.Float64("det-thresh", 0.5, "face detection confidence threshold")
	flag.Parse()

	if *viewOnly {
		fmt.Printf("Starting web UI for %s\n", *outputDir)
		if err := web.Serve(*outputDir, *port); err != nil {
			log.Fatalf("web server error: %v", err)
		}
		return
	}

	if err := inference.InitORT(*ortLib); err != nil {
		log.Fatalf("ONNX Runtime init error: %v", err)
	}
	defer inference.DestroyORT()

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		log.Fatalf("cannot create output dir: %v", err)
	}

	logFile, err := os.Create(filepath.Join(*outputDir, "processing.log"))
	if err != nil {
		log.Fatalf("cannot create log file: %v", err)
	}
	defer logFile.Close()
	w := io.MultiWriter(os.Stdout, logFile)

	start := time.Now()

	// --- Scan ---
	fmt.Fprintf(w, "=== Scanning directory ===\n")
	files, err := scanner.Scan(*inputDir)
	if err != nil {
		log.Fatalf("scan error: %v", err)
	}
	fmt.Fprintf(w, "Found %d image(s)\n\n", len(files))

	if len(files) == 0 {
		fmt.Fprintf(w, "No images found, nothing to do.\n")
		return
	}

	// --- Thumbnails dir ---
	thumbDir := filepath.Join(*outputDir, ".thumbnails")
	if err := os.RemoveAll(thumbDir); err != nil {
		log.Fatalf("cannot clean thumbnails dir: %v", err)
	}
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		log.Fatalf("cannot create thumbnails dir: %v", err)
	}

	// --- Extract ---
	fmt.Fprintf(w, "=== Extracting face embeddings ===\n")
	if *gpu {
		fmt.Fprintf(w, "Mode: GPU (CUDA)\n")
	} else {
		fmt.Fprintf(w, "Mode: CPU, %d worker(s)\n", *workers)
	}
	if *maxDim > 0 {
		fmt.Fprintf(w, "Pre-resize: max %dpx\n", *maxDim)
	}

	extractCfg := extractor.Config{
		ModelsDir: *modelsDir,
		Workers:   *workers,
		GPU:       *gpu,
		ThumbDir:  thumbDir,
		MaxDim:    *maxDim,
		DetThresh: float32(*detThresh),
	}
	extractResult, err := extractor.Extract(files, extractCfg, w)
	if err != nil {
		log.Fatalf("extraction error: %v", err)
	}
	fmt.Fprintf(w, "\nTotal faces detected: %d (errors: %d)\n\n", len(extractResult.Faces), extractResult.ErrorCount)

	if len(extractResult.Faces) == 0 {
		fmt.Fprintf(w, "No faces found, nothing to group.\n")
		return
	}

	// --- Cluster ---
	fmt.Fprintf(w, "=== Clustering faces ===\n")
	clusters := clustering.Cluster(extractResult.Faces, *threshold)
	fmt.Fprintf(w, "Found %d person(s)\n\n", len(clusters))

	// --- Organize ---
	fmt.Fprintf(w, "=== Organizing output ===\n")
	persons, err := organizer.Organize(clusters, *outputDir, w)
	if err != nil {
		log.Fatalf("organizer error: %v", err)
	}

	// --- Build report ---
	rpt := &report.Report{
		StartedAt:    start,
		InputDir:     *inputDir,
		OutputDir:    *outputDir,
		TotalImages:  len(files),
		TotalFaces:   len(extractResult.Faces),
		TotalPersons: len(clusters),
		Errors:       extractResult.ErrorCount,
		FileErrors:   extractResult.FileErrors,
		Threshold:    *threshold,
		GPU:          *gpu,
	}

	for _, p := range persons {
		rpt.Persons = append(rpt.Persons, report.PersonReport{
			ID:         p.ID,
			PhotoCount: p.PhotoCount,
			FaceCount:  p.FaceCount,
			Thumbnail:  p.Thumbnail,
			Photos:     p.Photos,
		})
	}

	// --- Describe via Moondream2 ---
	if *describe {
		appCfg, cfgErr := loadConfig(*configPath)
		if cfgErr != nil {
			fmt.Fprintf(w, "\nWARNING: cannot load config %s: %v — skipping descriptions\n", *configPath, cfgErr)
		} else {
			fmt.Fprintf(w, "\n=== Generating person descriptions (Moondream2) ===\n")
			fmt.Fprintf(w, "Endpoint: %s, Model: %s\n", appCfg.LMStudio.Endpoint, appCfg.LMStudio.Model)
			for i, p := range rpt.Persons {
				if p.Thumbnail == "" {
					continue
				}
				thumbPath := filepath.Join(*outputDir, p.Thumbnail)
				desc, descErr := describer.Describe(appCfg.LMStudio, thumbPath)
				if descErr != nil {
					fmt.Fprintf(w, "Person_%d: error: %v\n", p.ID, descErr)
					continue
				}
				rpt.Persons[i].Description = desc
				fmt.Fprintf(w, "Person_%d: %s\n", p.ID, desc)
			}
		}
	}

	// --- Finalize report ---
	rpt.FinishedAt = time.Now()
	rpt.Duration = time.Since(start).Round(time.Millisecond).String()

	if err := report.Save(rpt, *outputDir); err != nil {
		fmt.Fprintf(w, "WARNING: cannot save report: %v\n", err)
	}

	// --- Summary ---
	fmt.Fprintf(w, "\n=== Summary ===\n")
	fmt.Fprintf(w, "Images:  %d\n", rpt.TotalImages)
	fmt.Fprintf(w, "Faces:   %d\n", rpt.TotalFaces)
	fmt.Fprintf(w, "Persons: %d\n", rpt.TotalPersons)
	fmt.Fprintf(w, "Errors:  %d\n", rpt.Errors)
	fmt.Fprintf(w, "Time:    %s\n", rpt.Duration)
	fmt.Fprintf(w, "Report:  %s\n", filepath.Join(*outputDir, "report.json"))
	fmt.Fprintf(w, "Log:     %s\n", filepath.Join(*outputDir, "processing.log"))

	// --- Web UI ---
	if *serve {
		fmt.Fprintf(w, "\n=== Starting web UI ===\n")
		if err := web.Serve(*outputDir, *port); err != nil {
			log.Fatalf("web server error: %v", err)
		}
	} else {
		fmt.Fprintf(w, "\nTip: run with --serve to view results in browser, or --view to view previous results\n")
	}
}

func loadConfig(path string) (*appConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
