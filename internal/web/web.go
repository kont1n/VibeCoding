package web

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed index.html
var indexHTML []byte

// Serve starts an HTTP server exposing the report and photos from outputDir.
func Serve(outputDir string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	mux.HandleFunc("/api/report", func(w http.ResponseWriter, r *http.Request) {
		reportPath := filepath.Join(outputDir, "report.json")
		data, err := os.ReadFile(reportPath)
		if err != nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	fs := http.FileServer(http.Dir(outputDir))
	mux.Handle("/output/", http.StripPrefix("/output/", fs))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Web UI: http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}
