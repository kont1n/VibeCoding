// Package postgres provides PostgreSQL database connection pool management.
package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthStatus holds database health information.
type HealthStatus struct {
	Version     string
	Connections int32
	Extensions  []string
}

// CheckHealth checks database health and returns status.
func CheckHealth(ctx context.Context, pool *pgxpool.Pool) (*HealthStatus, error) {
	status := &HealthStatus{}

	// Get PostgreSQL version.
	var version string
	err := pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	// Extract version number from full string.
	if idx := strings.Index(version, " "); idx > 0 {
		version = version[:idx]
	}
	status.Version = version

	// Get connection count.
	var conns int32
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active'").Scan(&conns)
	if err != nil {
		return nil, fmt.Errorf("get connections: %w", err)
	}
	status.Connections = conns

	// Get enabled extensions.
	rows, err := pool.Query(ctx, "SELECT extname FROM pg_extension ORDER BY extname")
	if err != nil {
		return nil, fmt.Errorf("query extensions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ext string
		if err := rows.Scan(&ext); err != nil {
			return nil, fmt.Errorf("scan extension: %w", err)
		}
		status.Extensions = append(status.Extensions, ext)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate extensions: %w", err)
	}

	return status, nil
}
