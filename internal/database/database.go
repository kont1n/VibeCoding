// Package database provides database connection and management.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kont1n/face-grouper/internal/config/env"
	dbpostgres "github.com/kont1n/face-grouper/internal/database/postgres"
	repopostgres "github.com/kont1n/face-grouper/internal/repository/postgres"
)

// DB holds database connections and repositories.
type DB struct {
	Pool *pgxpool.Pool

	// Repositories.
	Persons   *repopostgres.PersonRepository
	Faces     *repopostgres.FaceRepository
	Photos    *repopostgres.PhotoRepository
	Relations *repopostgres.RelationRepository
	Sessions  *repopostgres.SessionRepository
}

// New creates a new database connection and initializes repositories.
func New(ctx context.Context, cfg env.DatabaseConfig) (*DB, error) {
	// Create connection pool.
	pool, err := dbpostgres.NewPool(ctx, dbpostgres.Config{
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

	// Run migrations if enabled.
	if cfg.RunMigrations {
		migrator := Migrator{pool: pool}
		if err := migrator.Migrate(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	// Create repositories.
	db := &DB{
		Pool:      pool,
		Persons:   repopostgres.NewPersonRepository(pool),
		Faces:     repopostgres.NewFaceRepository(pool),
		Photos:    repopostgres.NewPhotoRepository(pool),
		Relations: repopostgres.NewRelationRepository(pool),
		Sessions:  repopostgres.NewSessionRepository(pool),
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
func (d *DB) Health(ctx context.Context) (*dbpostgres.HealthStatus, error) {
	return dbpostgres.CheckHealth(ctx, d.Pool)
}
