// Package web provides the HTTP server and static file serving for the web UI.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kont1n/face-grouper/internal/api/http/handler"
	"github.com/kont1n/face-grouper/internal/api/http/middleware"
	"github.com/kont1n/face-grouper/internal/repository/database"
)

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
	uploadHandler  *handler.UploadHandler
	sessionHandler *handler.SessionHandler
	personHandler  *handler.PersonHandler
	errorHandler   *handler.ErrorHandler
	healthHandler  *handler.HealthHandler

	// Pipeline runner for async processing.
	pipelineRunner handler.PipelineRunner

	// stopCh signals background goroutines (e.g. rate limiter cleanup) to stop.
	stopCh chan struct{}
}

// NewServer creates and configures a new HTTP server.
func NewServer(cfg ServerConfig, pipelineRunner handler.PipelineRunner) *Server {
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
	var healthChecker handler.HealthChecker
	if s.cfg.DB != nil {
		healthChecker = s.cfg.DB.Pool
	}
	s.healthHandler = handler.NewHealthHandler(healthChecker, "1.0.0")

	uploadDir := s.cfg.UploadDir
	if uploadDir == "" {
		uploadDir = filepath.Join(s.cfg.OutputDir, ".uploads")
	}

	s.uploadHandler = handler.NewUploadHandler(uploadDir, 500<<20) // 500MB max.
	s.sessionHandler = handler.NewSessionHandler(s.pipelineRunner, uploadDir)
	s.personHandler = handler.NewPersonHandler(s.cfg.OutputDir, s.cfg.DB)
	s.errorHandler = handler.NewErrorHandler(s.cfg.OutputDir, s.cfg.DB)
}

func (s *Server) registerRoutes() {
	// Static: SPA frontend.
	s.mux.HandleFunc("GET /", s.serveIndex)
	s.mux.HandleFunc("GET /api/report", s.serveReport)

	// File serving.
	fs := http.FileServer(http.Dir(s.cfg.OutputDir))
	s.mux.Handle("GET /output/", http.StripPrefix("/output/", fs))

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

func (s *Server) applyMiddleware() {
	// Build middleware chain: Recovery → RateLimit → MaxBody → CORS → Handler.
	rateLimiter := middleware.NewRateLimiter(100, 200)

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
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "report not found"})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
