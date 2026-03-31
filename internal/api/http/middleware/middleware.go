// Package middleware provides HTTP middleware components.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	pkgerrors "github.com/kont1n/face-grouper/internal/pkg/errors"
)

// MaxBodySize middleware limits the request body size.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter provides IP-based HTTP rate limiting middleware.
type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	rps      rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter middleware.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns or creates a rate limiter for a given key.
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock.
	if entry, exists = rl.limiters[key]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	rl.limiters[key] = &limiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

// Middleware returns the HTTP middleware for rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := getClientIP(r)
		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP returns the client IP address from RemoteAddr.
// X-Forwarded-For is intentionally ignored to prevent IP spoofing.
// If you need to trust X-Forwarded-For behind a known reverse proxy,
// configure trusted proxy addresses and validate the header accordingly.
func getClientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		return r.RemoteAddr
	}
	return ip
}

// Cleanup removes limiters that haven't been used recently.
func (rl *RateLimiter) Cleanup(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, entry := range rl.limiters {
				// Remove entries that haven't been accessed for more than 3 minutes.
				if now.Sub(entry.lastSeen) > 3*time.Minute {
					delete(rl.limiters, key)
				}
			}
			rl.mu.Unlock()
		case <-stopCh:
			return
		}
	}
}

// Recovery middleware recovers from panics and returns 500 Internal Server Error.
func Recovery(logger interface{ Error(string, ...any) }) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error("panic recovered", "error", err, "path", r.URL.Path)
					}

					// Check if it's a context cancellation.
					if errors.Is(r.Context().Err(), context.Canceled) {
						return
					}

					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string         `json:"error"`
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, statusCode int, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err})
}

// WriteErrorWithCode writes an error response with error code.
func WriteErrorWithCode(w http.ResponseWriter, statusCode int, err, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err,
		Code:    code,
		Message: message,
	})
}

// WriteAppError writes a structured AppError response.
func WriteAppError(w http.ResponseWriter, appErr *pkgerrors.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   appErr.Message,
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Details: appErr.Details,
	})
}

// CORS middleware adds CORS headers.
// allowedOrigins specifies permitted origins. If empty, defaults to same-origin only (no CORS header).
// Pass []string{"*"} to allow all origins (development only).
func CORS(next http.Handler, allowedOrigins ...string) http.Handler {
	originSet := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
		}
		originSet[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if originSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestLogger middleware logs HTTP requests.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		// Skip logging for static assets.
		if r.URL.Path == "/" || r.URL.Path == "/health" {
			return
		}

		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// EndpointRateLimit defines rate limit configuration for a specific endpoint pattern.
type EndpointRateLimit struct {
	Pattern string  // URL pattern (e.g., "/api/v1/upload", "/health").
	RPS     float64 // Requests per second.
	Burst   int     // Maximum burst size.
}

// MultiRateLimiter provides per-endpoint HTTP rate limiting middleware.
// It allows different rate limits for different endpoint patterns.
type MultiRateLimiter struct {
	limits       map[string]*rate.Limiter // pattern -> limiter.
	limiters     map[string]*limiterEntry // ip:pattern -> limiter.
	mu           sync.RWMutex
	defaultRPS   rate.Limit
	defaultBurst int
}

// NewMultiRateLimiter creates a new multi-endpoint rate limiter.
// defaultRPS and defaultBurst are used for endpoints without specific configuration.
func NewMultiRateLimiter(defaultRPS float64, defaultBurst int) *MultiRateLimiter {
	return &MultiRateLimiter{
		limits:       make(map[string]*rate.Limiter),
		limiters:     make(map[string]*limiterEntry),
		defaultRPS:   rate.Limit(defaultRPS),
		defaultBurst: defaultBurst,
	}
}

// AddEndpointLimit adds a rate limit configuration for a specific endpoint pattern.
func (mrl *MultiRateLimiter) AddEndpointLimit(pattern string, rps float64, burst int) {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()
	mrl.limits[pattern] = rate.NewLimiter(rate.Limit(rps), burst)
}

// getLimiterForEndpoint returns the appropriate limiter for the given IP and endpoint.
func (mrl *MultiRateLimiter) getLimiterForEndpoint(ip, path string) *rate.Limiter {
	// Find matching pattern (exact match or prefix match).
	var pattern string
	for p := range mrl.limits {
		if strings.HasPrefix(path, p) {
			if pattern == "" || len(p) > len(pattern) {
				pattern = p
			}
		}
	}

	key := ip + ":" + pattern
	if pattern == "" {
		key = ip + ":default"
	}

	mrl.mu.RLock()
	entry, exists := mrl.limiters[key]
	mrl.mu.RUnlock()

	if exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	// Double-check.
	if entry, exists = mrl.limiters[key]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	// Create new limiter for this key.
	var limiter *rate.Limiter
	if pattern != "" && mrl.limits[pattern] != nil {
		// Use endpoint-specific limit.
		cfg := mrl.limits[pattern]
		limiter = rate.NewLimiter(cfg.Limit(), cfg.Burst())
	} else {
		// Use default limit.
		limiter = rate.NewLimiter(mrl.defaultRPS, mrl.defaultBurst)
	}

	mrl.limiters[key] = &limiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	return limiter
}

// Middleware returns the HTTP middleware for per-endpoint rate limiting.
func (mrl *MultiRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := mrl.getLimiterForEndpoint(ip, r.URL.Path)

		if !limiter.Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Cleanup removes limiters that haven't been used recently.
func (mrl *MultiRateLimiter) Cleanup(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mrl.mu.Lock()
			now := time.Now()
			for key, entry := range mrl.limiters {
				if now.Sub(entry.lastSeen) > 3*time.Minute {
					delete(mrl.limiters, key)
				}
			}
			mrl.mu.Unlock()
		case <-stopCh:
			return
		}
	}
}
