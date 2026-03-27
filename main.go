package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/kont1n/face-grouper/internal/clustering"
	"github.com/kont1n/face-grouper/internal/extractor"
	"github.com/kont1n/face-grouper/internal/organizer"
	"github.com/kont1n/face-grouper/internal/scanner"
)

func main() {
	inputDir := flag.String("input", "./dataset", "path to directory with photos")
	outputDir := flag.String("output", "./output", "path for grouped output")
	workers := flag.Int("workers", 4, "number of parallel extraction workers")
	threshold := flag.Float64("threshold", 0.5, "cosine similarity threshold for face grouping")
	pythonBin := flag.String("python", "python", "path to Python interpreter")
	flag.Parse()

	scriptPath, err := filepath.Abs(filepath.Join("scripts", "extract_faces.py"))
	if err != nil {
		log.Fatalf("cannot resolve script path: %v", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		log.Fatalf("Python script not found at %s: %v", scriptPath, err)
	}

	start := time.Now()

	fmt.Println("=== Scanning directory ===")
	files, err := scanner.Scan(*inputDir)
	if err != nil {
		log.Fatalf("scan error: %v", err)
	}
	fmt.Printf("Found %d image(s)\n\n", len(files))

	if len(files) == 0 {
		fmt.Println("No images found, nothing to do.")
		return
	}

	fmt.Println("=== Extracting face embeddings ===")
	cfg := extractor.Config{
		PythonBin:  *pythonBin,
		ScriptPath: scriptPath,
		Workers:    *workers,
	}
	faces, err := extractor.Extract(files, cfg)
	if err != nil {
		log.Fatalf("extraction error: %v", err)
	}
	fmt.Printf("\nTotal faces detected: %d\n\n", len(faces))

	if len(faces) == 0 {
		fmt.Println("No faces found, nothing to group.")
		return
	}

	fmt.Println("=== Clustering faces ===")
	clusters := clustering.Cluster(faces, *threshold)
	fmt.Printf("Found %d person(s)\n\n", len(clusters))

	fmt.Println("=== Organizing output ===")
	if err := organizer.Organize(clusters, *outputDir); err != nil {
		log.Fatalf("organizer error: %v", err)
	}

	fmt.Printf("\nDone in %s. Results in: %s\n", time.Since(start).Round(time.Millisecond), *outputDir)
}
