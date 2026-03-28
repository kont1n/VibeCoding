// Package postgres provides PostgreSQL database connectivity using pgx.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds PostgreSQL connection configuration.
type Config struct {
	Host              string
	Port              int
	Database          string
	User              string
	Password          string
	SSLMode           string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// NewPool creates a new PostgreSQL connection pool.
func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	// Build connection string
	connString := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.SSLMode,
	)

	// Parse configuration
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod

	// Configure connection timeouts
	poolConfig.ConnConfig.ConnectTimeout = 10 * time.Second
	poolConfig.ConnConfig.RuntimeParams = map[string]string{
		"application_name": "face-grouper",
	}

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// DefaultConfig returns default PostgreSQL configuration.
func DefaultConfig() Config {
	return Config{
		Host:              "localhost",
		Port:              5432,
		Database:          "face-grouper",
		User:              "face-grouper",
		Password:          "secret",
		SSLMode:           "disable",
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   30 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}
}

// ConfigFromEnv reads configuration from environment variables.
func ConfigFromEnv() Config {
	cfg := DefaultConfig()

	// Override with environment variables if needed
	// This can be extended to read from actual env vars
	// cfg.Host = getEnv("DB_HOST", cfg.Host)
	// cfg.Port = getInt("DB_PORT", cfg.Port)
	// etc.

	return cfg
}
