// Package database provides database connection and management.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/config/env"
	"github.com/kont1n/face-grouper/internal/repository/postgres"
)

// DB holds database connections and repositories.
type DB struct {
	Pool *pgxpool.Pool

	// Repositories
	Persons   *postgres.PersonRepository
	Faces     *postgres.FaceRepository
	Photos    *postgres.PhotoRepository
	Relations *postgres.RelationRepository
	Sessions  *postgres.SessionRepository
}

// New creates a new database connection and initializes repositories.
func New(ctx context.Context, cfg env.DatabaseConfig) (*DB, error) {
	// Create connection pool
	pool, err := postgres.NewPool(ctx, postgres.Config{
		Host:              cfg.Host,
		Port:              cfg.Port,
		Database:          cfg.Database,
		User:              cfg.User,
		Password:          cfg.Password,
		SSLMode:           cfg.SSLMode,
		MaxConns:          int32(cfg.MaxConns),
		MinConns:          int32(cfg.MinConns),
		MaxConnLifetime:   time.Duration(cfg.MaxConnLifetime) * time.Second,
		MaxConnIdleTime:   time.Duration(cfg.MaxConnIdleTime) * time.Second,
		HealthCheckPeriod: time.Duration(cfg.HealthCheckPeriod) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Run migrations if enabled
	if cfg.RunMigrations {
		migrator := NewMigrator(pool)
		if err := migrator.Migrate(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	// Create repositories
	db := &DB{
		Pool:      pool,
		Persons:   postgres.NewPersonRepository(pool),
		Faces:     postgres.NewFaceRepository(pool),
		Photos:    postgres.NewPhotoRepository(pool),
		Relations: postgres.NewRelationRepository(pool),
		Sessions:  postgres.NewSessionRepository(pool),
	}

	return db, nil
}

// Close closes all database connections.
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}

// Health returns database health status.
func (d *DB) Health(ctx context.Context) (*postgres.HealthStatus, error) {
	return postgres.CheckHealth(ctx, d.Pool)
}
