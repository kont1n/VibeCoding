// Package web provides the HTTP server and static file serving for the web UI.
package web

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/kont1n/face-grouper/internal/api/http/handler"
	"github.com/kont1n/face-grouper/internal/api/http/middleware"
	"github.com/kont1n/face-grouper/internal/repository/database"
)

//go:embed index.html
var indexHTML []byte

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port           int
	OutputDir      string
	UploadDir      string
	DB             *database.DB
	AllowedOrigins []string // CORS allowed origins. Empty = same-origin only.
}

// Server is the main HTTP server with all API routes.
type Server struct {
	cfg     ServerConfig
	mux     *http.ServeMux
	handler http.Handler

	// Handlers.
	uploadHandler  *api.UploadHandler
	sessionHandler *api.SessionHandler
	personHandler  *api.PersonHandler
	errorHandler   *api.ErrorHandler
	healthHandler  *api.HealthHandler

	// Pipeline runner for async processing.
	pipelineRunner api.PipelineRunner

	// stopCh signals background goroutines (e.g. rate limiter cleanup) to stop.
	stopCh chan struct{}
}

// NewServer creates and configures a new HTTP server.
func NewServer(cfg ServerConfig, pipelineRunner api.PipelineRunner) *Server {
	s := &Server{
		cfg:            cfg,
		mux:            http.NewServeMux(),
		pipelineRunner: pipelineRunner,
		stopCh:         make(chan struct{}),
	}

	s.initHandlers()
	s.registerRoutes()
	s.applyMiddleware()

	return s
}

func (s *Server) initHandlers() {
	var healthChecker api.HealthChecker
	if s.cfg.DB != nil {
		healthChecker = s.cfg.DB.Pool
	}
	s.healthHandler = api.NewHealthHandler(healthChecker, "1.0.0")

	uploadDir := s.cfg.UploadDir
	if uploadDir == "" {
		uploadDir = filepath.Join(s.cfg.OutputDir, ".uploads")
	}

	s.uploadHandler = api.NewUploadHandler(uploadDir, 500<<20) // 500MB max.
	s.sessionHandler = api.NewSessionHandler(s.pipelineRunner, uploadDir)
	s.personHandler = api.NewPersonHandler(s.cfg.OutputDir, s.cfg.DB)
	s.errorHandler = api.NewErrorHandler(s.cfg.OutputDir, s.cfg.DB)
}

func (s *Server) registerRoutes() {
	// Static: SPA frontend.
	s.mux.HandleFunc("GET /", s.serveIndex)
	s.mux.HandleFunc("GET /api/report", s.serveReport)

	// File serving: only allow image files to prevent directory listing exposure.
	s.mux.HandleFunc("GET /output/", s.serveOutputFile)

	// Health.
	s.mux.HandleFunc("GET /health", s.healthHandler.HealthCheck)
	s.mux.HandleFunc("GET /ready", s.healthHandler.ReadyCheck)

	// Upload API.
	s.mux.HandleFunc("POST /api/v1/upload", s.uploadHandler.Upload)

	// Session / Processing API.
	s.mux.HandleFunc("POST /api/v1/sessions/{id}/process", s.sessionHandler.StartProcessing)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/status", s.sessionHandler.GetStatus)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/stream", s.sessionHandler.StreamProgress)
	s.mux.HandleFunc("POST /api/v1/sessions/{id}/cancel", s.sessionHandler.CancelProcessing)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/errors", s.errorHandler.GetSessionErrors)

	// Persons API.
	s.mux.HandleFunc("GET /api/v1/persons", s.personHandler.List)
	s.mux.HandleFunc("GET /api/v1/persons/{id}", s.personHandler.Get)
	s.mux.HandleFunc("PUT /api/v1/persons/{id}", s.personHandler.Rename)
	s.mux.HandleFunc("GET /api/v1/persons/{id}/photos", s.personHandler.Photos)
	s.mux.HandleFunc("GET /api/v1/persons/{id}/relations", s.personHandler.Relations)
}

// allowedExtensions defines which file extensions can be served from output directory.
var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
	".svg":  true,
}

// serveOutputFile serves files from output directory but only allows image files.
// This prevents directory listing and exposure of sensitive files like logs.
func (s *Server) serveOutputFile(w http.ResponseWriter, r *http.Request) {
	// Clean the path to prevent directory traversal.
	requestPath := strings.TrimPrefix(r.URL.Path, "/output/")
	requestPath = filepath.Clean(requestPath)

	// Build full path.
	fullPath := filepath.Join(s.cfg.OutputDir, requestPath)

	// Check file extension.
	ext := strings.ToLower(filepath.Ext(fullPath))
	if !allowedExtensions[ext] {
		http.Error(w, "file type not allowed", http.StatusForbidden)
		return
	}

	// Verify the file is within output directory (prevent traversal via symlinks).
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	absOutput, err := filepath.Abs(s.cfg.OutputDir)
	if err != nil {
		http.Error(w, "invalid output dir", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(absPath, absOutput) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	// Check if file exists and is a regular file.
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "error accessing file", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "directory listing not allowed", http.StatusForbidden)
		return
	}

	// Serve the file with cache headers for static assets.
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	http.ServeFile(w, r, fullPath)
}

func (s *Server) applyMiddleware() {
	// Build middleware chain: Recovery → RateLimit → MaxBody → CORS → Handler.
	// Use per-endpoint rate limiting with different limits for different endpoints.
	rateLimiter := middleware.NewMultiRateLimiter(100, 200) // Default: 100 RPS, burst 200.

	// Stricter limit for upload endpoint (resource-intensive).
	rateLimiter.AddEndpointLimit("/api/v1/upload", 10, 20) // 10 RPS, burst 20.

	// Higher limit for health checks (lightweight).
	rateLimiter.AddEndpointLimit("/health", 1000, 2000) // 1000 RPS, burst 2000.

	go rateLimiter.Cleanup(5*time.Minute, s.stopCh)

	var h http.Handler = s.mux
	// Embedded SPA is served from same origin, so no CORS needed by default.
	// Allow same-origin only; override via ServerConfig.AllowedOrigins if needed.
	h = middleware.CORS(h, s.cfg.AllowedOrigins...)
	h = middleware.MaxBodySize(500 << 20)(h)
	h = rateLimiter.Middleware(h)
	h = middleware.Recovery(nil)(h)
	h = middleware.RequestLogger(h)

	s.handler = h
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}

func (s *Server) serveReport(w http.ResponseWriter, r *http.Request) {
	reportPath := filepath.Join(s.cfg.OutputDir, "report.json")
	data, err := os.ReadFile(reportPath) //nolint:gosec
	if err != nil {
		api.WriteJSON(w, http.StatusNotFound, map[string]string{"error": "report not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// ListenAndServe starts the HTTP server with graceful shutdown.
// It listens for context cancellation (e.g. from signal.NotifyContext in main)
// instead of registering its own signal handler, avoiding double signal.Notify conflicts.
func (s *Server) ListenAndServe() error {
	return s.ListenAndServeContext(context.Background())
}

// ListenAndServeContext starts the HTTP server and shuts it down when ctx is canceled.
func (s *Server) ListenAndServeContext(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // SSE needs longer write timeout.
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Web UI: http://localhost%s", addr)
		log.Printf("API:    http://localhost%s/api/v1/", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Printf("Context canceled, shutting down...")
	case err := <-errCh:
		return err
	}

	// Stop background goroutines (e.g. rate limiter cleanup).
	close(s.stopCh)

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Printf("Server stopped")
	return nil
}
