package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kont1n/face-grouper/internal/clustering"
	"github.com/kont1n/face-grouper/internal/extractor"
	"github.com/kont1n/face-grouper/internal/inference"
	"github.com/kont1n/face-grouper/internal/organizer"
	"github.com/kont1n/face-grouper/internal/report"
	"github.com/kont1n/face-grouper/internal/scanner"
	"github.com/kont1n/face-grouper/internal/web"
)

func main() {
	inputDir := flag.String("input", "./dataset", "path to directory with photos")
	outputDir := flag.String("output", "./output", "path for grouped output")
	workers := flag.Int("workers", 4, "number of parallel extraction workers")
	gpuDetSessions := flag.Int("gpu-det-sessions", 2, "number of detector ONNX sessions for GPU mode")
	gpuRecSessions := flag.Int("gpu-rec-sessions", 2, "number of recognizer ONNX sessions for GPU mode")
	embedBatchSize := flag.Int("embed-batch-size", 64, "cross-file recognition batch size")
	embedFlushMS := flag.Int("embed-flush-ms", 10, "embedding batch flush timeout in milliseconds")
	threshold := flag.Float64("threshold", 0.5, "cosine similarity threshold for face grouping")
	gpu := flag.Bool("gpu", false, "use GPU (CUDA) for ONNX Runtime inference")
	intraThreads := flag.Int("intra-threads", 0, "ONNX Runtime intra-op threads (0 = default)")
	interThreads := flag.Int("inter-threads", 0, "ONNX Runtime inter-op threads (0 = default)")
	maxDim := flag.Int("max-dim", 1920, "downscale images so longest side <= this value (0 = no resize)")
	serve := flag.Bool("serve", false, "start web UI after processing")
	port := flag.Int("port", 8080, "web UI port")
	avatarUpdateThreshold := flag.Float64("avatar-update-threshold", 0.10, "minimum relative quality increase required to update avatar (0.10 = 10%)")
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

	inference.SetSessionTuning(inference.SessionTuning{
		IntraOpThreads: *intraThreads,
		InterOpThreads: *interThreads,
	})

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
	stageDurations := make(map[string]time.Duration)

	// --- Scan ---
	stageStart := time.Now()
	fmt.Fprintf(w, "=== Scanning directory ===\n")
	files, err := scanner.Scan(*inputDir)
	if err != nil {
		log.Fatalf("scan error: %v", err)
	}
	fmt.Fprintf(w, "Found %d image(s)\n\n", len(files))
	stageDurations["scan"] = time.Since(stageStart)

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
	stageStart = time.Now()
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
		ModelsDir:      *modelsDir,
		Workers:        *workers,
		GPU:            *gpu,
		GPUDetSessions: *gpuDetSessions,
		GPURecSessions: *gpuRecSessions,
		EmbedBatchSize: *embedBatchSize,
		EmbedFlushMS:   *embedFlushMS,
		ThumbDir:       thumbDir,
		MaxDim:         *maxDim,
		DetThresh:      float32(*detThresh),
	}
	extractResult, err := extractor.Extract(files, extractCfg, w)
	if err != nil {
		log.Fatalf("extraction error: %v", err)
	}
	fmt.Fprintf(w, "\nTotal faces detected: %d (errors: %d)\n\n", len(extractResult.Faces), extractResult.ErrorCount)
	stageDurations["extract"] = time.Since(stageStart)

	if len(extractResult.Faces) == 0 {
		fmt.Fprintf(w, "No faces found, nothing to group.\n")
		return
	}

	// --- Cluster ---
	stageStart = time.Now()
	fmt.Fprintf(w, "=== Clustering faces ===\n")
	clusters := clustering.Cluster(extractResult.Faces, *threshold)
	fmt.Fprintf(w, "Found %d person(s)\n\n", len(clusters))
	stageDurations["cluster"] = time.Since(stageStart)

	// --- Organize ---
	stageStart = time.Now()
	fmt.Fprintf(w, "=== Organizing output ===\n")
	persons, err := organizer.Organize(clusters, *outputDir, *avatarUpdateThreshold, w)
	if err != nil {
		log.Fatalf("organizer error: %v", err)
	}
	stageDurations["organize_avatar"] = time.Since(stageStart)

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
			ID:           p.ID,
			PhotoCount:   p.PhotoCount,
			FaceCount:    p.FaceCount,
			Thumbnail:    p.Thumbnail,
			AvatarPath:   p.AvatarPath,
			QualityScore: p.QualityScore,
			Photos:       p.Photos,
		})
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
	fmt.Fprintf(w, "\n=== Stage timings ===\n")
	fmt.Fprintf(w, "Scan:           %s\n", stageDurations["scan"].Round(time.Millisecond))
	fmt.Fprintf(w, "Extract:        %s\n", stageDurations["extract"].Round(time.Millisecond))
	fmt.Fprintf(w, "Cluster:        %s\n", stageDurations["cluster"].Round(time.Millisecond))
	fmt.Fprintf(w, "OrganizeAvatar: %s\n", stageDurations["organize_avatar"].Round(time.Millisecond))
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
