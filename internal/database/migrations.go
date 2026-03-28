// Package database provides database connection and management.
package database

import (
	"context"
	"embed"
	"fmt"
	"strings"

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
	return &Migrator{pool: pool}
}

// Migrate runs all pending migrations using pgx directly.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Read all migration files
	files, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Apply migrations in order
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + file.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file.Name(), err)
		}

		// Execute migration
		// Split by semicolons to handle multiple statements
		statements := strings.Split(string(content), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}

			_, err = m.pool.Exec(ctx, stmt)
			if err != nil {
				// Ignore "already exists" errors
				if strings.Contains(err.Error(), "already exists") {
					continue
				}
				return fmt.Errorf("execute migration %s: %w", file.Name(), err)
			}
		}
	}

	return nil
}

// MigrateTo migrates to a specific version (not implemented for simple pgx migrations).
func (m *Migrator) MigrateTo(ctx context.Context, version uint) error {
	return fmt.Errorf("MigrateTo not implemented - use Migrate for sequential migrations")
}

// GetVersion returns the current database schema version (not implemented).
func (m *Migrator) GetVersion(ctx context.Context) (uint, bool, error) {
	return 0, false, fmt.Errorf("GetVersion not implemented")
}

// Rollback rolls back the last migration (not implemented).
func (m *Migrator) Rollback(ctx context.Context) error {
	return fmt.Errorf("Rollback not implemented")
}
