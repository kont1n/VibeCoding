// Package database provides database connection and management.
package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
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
// Parses migration files with proper handling of PL/pgSQL functions.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Read all migration files.
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Filter and sort SQL files.
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)

	// Create migrations table if not exists.
	_, err = m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			dirty BOOLEAN NOT NULL DEFAULT FALSE
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get applied migrations.
	applied := make(map[string]bool)
	rows, err := m.pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = true
	}
	rows.Close()

	// Apply migrations in order.
	for _, file := range files {
		version := strings.TrimSuffix(file, ".sql")
		if applied[version] {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		// Parse migration file, handling PL/pgSQL functions.
		statements := parseMigrationStatements(string(content))

		// Execute statements in a transaction.
		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin transaction for %s: %w", file, err)
		}

		for _, stmt := range statements {
			_, err = tx.Exec(ctx, stmt)
			if err != nil {
				_ = tx.Rollback(ctx)
				// Ignore "already exists" errors.
				if strings.Contains(err.Error(), "already exists") {
					continue
				}
				return fmt.Errorf("execute migration %s: %w", file, err)
			}
		}

		// Mark migration as applied.
		_, err = tx.Exec(ctx,
			"INSERT INTO schema_migrations (version, dirty) VALUES ($1, FALSE)",
			version)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("mark migration %s as applied: %w", file, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", file, err)
		}
	}

	return nil
}

// parseMigrationStatements parses SQL content into individual statements,
// properly handling PL/pgSQL function bodies with embedded semicolons.
func parseMigrationStatements(content string) []string {
	// Remove comments.
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	content = strings.Join(cleaned, "\n")

	// Split by migration directives.
	parts := strings.Split(content, "+migrate")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		// Skip "Down" sections for forward migrations.
		if strings.HasPrefix(strings.TrimSpace(part), "Down") {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(part), "Up") {
			part = strings.TrimPrefix(part, "Up")
		}

		// Parse statements, handling $$ delimited blocks.
		return parseSQLWithDollarQuoted(part)
	}
	return nil
}

// parseSQLWithDollarQuoted splits SQL into statements, respecting $$ quoted blocks.
func parseSQLWithDollarQuoted(sql string) []string {
	var statements []string
	var current strings.Builder
	inDollarQuoted := false

	for i := 0; i < len(sql); i++ {
		// Check for $$ delimiter.
		if i+1 < len(sql) && sql[i:i+2] == "$$" {
			inDollarQuoted = !inDollarQuoted
			current.WriteString("$$")
			i++
			continue
		}

		// Split on semicolon only outside $$ blocks.
		if sql[i] == ';' && !inDollarQuoted {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(sql[i])
	}

	// Add remaining statement.
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// MigrateTo migrates to a specific version (not implemented for simple pgx migrations).
func (m *Migrator) MigrateTo(ctx context.Context, version uint) error {
	return fmt.Errorf("MigrateTo not implemented - use Migrate for sequential migrations")
}

// GetVersion returns the current database schema version (not implemented).
func (m *Migrator) GetVersion(ctx context.Context) (ver uint, ok bool, err error) {
	return 0, false, fmt.Errorf("GetVersion not implemented")
}

// Rollback rolls back the last migration (not implemented).
func (m *Migrator) Rollback(ctx context.Context) error {
	return fmt.Errorf("Rollback not implemented")
}
