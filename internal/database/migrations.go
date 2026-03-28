// Package database provides database connection and management.
package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator handles database migrations.
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{
		pool: pool,
	}
}

// Migrate runs all pending migrations.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Use pool's underlying database connection
	db := m.pool.Pool()
	
	// Create database driver
	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create db driver: %w", err)
	}

	// Create source driver from embedded FS
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create source driver: %w", err)
	}

	// Create migrator
	migt, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"postgres",
		dbDriver,
	)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	// Run migrations
	if err := migt.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		return nil
	}

	return nil
}

// MigrateTo migrates to a specific version.
func (m *Migrator) MigrateTo(ctx context.Context, version uint) error {
	db := m.pool.Pool()

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create db driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create source driver: %w", err)
	}

	migt, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := migt.Migrate(version); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate to version %d: %w", version, err)
	}

	return nil
}

// GetVersion returns the current database schema version.
func (m *Migrator) GetVersion(ctx context.Context) (uint, bool, error) {
	db := m.pool.Pool()

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("create db driver: %w", err)
	}

	migt, err := migrate.NewWithInstance("postgres", dbDriver, "postgres", nil)
	if err != nil {
		return 0, false, fmt.Errorf("create migrator: %w", err)
	}

	version, dirty, err := migt.Version()
	if err != nil {
		return 0, false, fmt.Errorf("get version: %w", err)
	}

	return version, dirty, nil
}

// Rollback rolls back the last migration.
func (m *Migrator) Rollback(ctx context.Context) error {
	db := m.pool.Pool()

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create db driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create source driver: %w", err)
	}

	migt, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := migt.Steps(-1); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	return nil
}

// GetAvailableMigrations returns a list of available migrations.
func (m *Migrator) GetAvailableMigrations() ([]string, error) {
	files, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var migrations []string
	for _, file := range files {
		if !file.IsDir() && file.Name() != ".gitkeep" {
			migrations = append(migrations, file.Name())
		}
	}

	return migrations, nil
}
