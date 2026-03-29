// Package handler provides HTTP request handlers.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthStatus represents the health status of the application.
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	DB        string                 `json:"db,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker defines the interface for health check dependencies.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler implements the health check endpoint.
type HealthHandler struct {
	checker HealthChecker
	version string
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(checker HealthChecker, version string) *HealthHandler {
	return &HealthHandler{
		checker: checker,
		version: version,
	}
}

// HealthCheck handles the /health endpoint.
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := HealthStatus{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
		Version:   h.version,
		Details:   make(map[string]interface{}),
	}

	// Check database connection if checker is provided.
	if h.checker != nil {
		if err := h.checker.Ping(ctx); err != nil {
			health.Status = "degraded"
			health.DB = "unreachable"
			health.Details["db_error"] = err.Error()
		} else {
			health.DB = "ok"
		}
	}

	statusCode := http.StatusOK
	if health.Status == "degraded" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)
}

// ReadyCheck handles the /ready endpoint for Kubernetes readiness probes.
func (h *HealthHandler) ReadyCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	ready := true
	reasons := make([]string, 0)

	// Check database connection.
	if h.checker != nil {
		if err := h.checker.Ping(ctx); err != nil {
			ready = false
			reasons = append(reasons, "database unreachable")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if ready {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "not ready",
			"reasons": reasons,
		})
	}
}
