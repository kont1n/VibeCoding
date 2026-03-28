package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthStatus represents database health information.
type HealthStatus struct {
	Status      string        `json:"status"`
	Version     string        `json:"version"`
	Connections int32         `json:"connections"`
	Latency     time.Duration `json:"latency_ms"`
	Extensions  []string      `json:"extensions"`
}

// CheckHealth performs a health check on the database connection.
func CheckHealth(ctx context.Context, pool *pgxpool.Pool) (*HealthStatus, error) {
	start := time.Now()

	// Check connection
	if err := pool.Ping(ctx); err != nil {
		return &HealthStatus{Status: "unhealthy"}, fmt.Errorf("ping: %w", err)
	}

	// Get PostgreSQL version
	var version string
	err := pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return &HealthStatus{Status: "degraded"}, fmt.Errorf("version: %w", err)
	}

	// Check installed extensions
	rows, err := pool.Query(ctx, `
		SELECT extname 
		FROM pg_extension 
		WHERE extname IN ('vector', 'pg_stat_statements', 'uuid-ossp')
		ORDER BY extname
	`)
	if err != nil {
		return &HealthStatus{Status: "degraded"}, fmt.Errorf("extensions: %w", err)
	}
	defer rows.Close()

	var extensions []string
	for rows.Next() {
		var ext string
		if err := rows.Scan(&ext); err != nil {
			continue
		}
		extensions = append(extensions, ext)
	}

	// Get connection statistics
	stats := pool.Stat()

	return &HealthStatus{
		Status:      "healthy",
		Version:     version,
		Connections: int32(stats.TotalConns()),
		Latency:     time.Since(start),
		Extensions:  extensions,
	}, nil
}

// IsHealthy checks if the database connection is healthy.
func IsHealthy(ctx context.Context, pool *pgxpool.Pool) bool {
	return pool.Ping(ctx) == nil
}

// GetConnectionStats returns connection pool statistics.
func GetConnectionStats(pool *pgxpool.Pool) map[string]uint32 {
	stats := pool.Stat()
	return map[string]uint32{
		"total_conns":    uint32(stats.TotalConns()),
		"acquired_conns": uint32(stats.AcquiredConns()),
		"idle_conns":     uint32(stats.IdleConns()),
		"max_conns":      uint32(stats.MaxConns()),
	}
}
