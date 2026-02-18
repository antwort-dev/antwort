package postgres

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrate applies pending schema migrations. It reads embedded SQL files,
// tracks applied versions in the schema_migrations table, and applies
// any that haven't been run yet.
func (s *Store) migrate(ctx context.Context) error {
	// Read all migration files.
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	// Sort by filename (which starts with version number).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Extract version from filename (e.g., "001_create_responses.sql" -> 1).
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		// Check if already applied.
		var exists bool
		err = s.pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&exists)
		// If schema_migrations doesn't exist yet, this will fail, which is fine
		// for the first migration that creates the table.
		if err != nil {
			exists = false
		}

		if exists {
			continue
		}

		// Read and apply migration.
		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", entry.Name(), err)
		}

		slog.Info("applying migration", "file", entry.Name(), "version", version)

		if _, err := s.pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("applying migration %s: %w", entry.Name(), err)
		}

		// Record migration.
		if _, err := s.pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING",
			version,
		); err != nil {
			return fmt.Errorf("recording migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
