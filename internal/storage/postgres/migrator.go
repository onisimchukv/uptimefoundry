package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/forgeops/forgeops/migrations"
)

// Migrate runs embedded SQL migrations in order against the PostgreSQL database.
func (db *DB) Migrate(ctx context.Context) error {
	db.Logger.Info("running database migrations")

	// Ensure schema_migrations table exists
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`
	if _, err := db.Pool.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations fs: %w", err)
	}

	var filenames []string
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[len(entry.Name())-4:] == ".sql" {
			filenames = append(filenames, entry.Name())
		}
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		var applied bool
		err := db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration status for %s: %w", filename, err)
		}

		if applied {
			continue
		}

		db.Logger.Info("applying migration", slog.String("file", filename))

		content, err := fs.ReadFile(migrations.FS, filename)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", filename, err)
		}

		// Execute migration within a transaction
		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}

		db.Logger.Info("successfully applied migration", slog.String("file", filename))
	}

	// Re-detect extensions after migrations in case age or vector were just enabled
	db.detectExtensions(ctx)

	return nil
}

// EnsureGraphExists verifies or creates the Apache AGE graph if the extension is present.
func (db *DB) EnsureGraphExists(ctx context.Context, graphName string) error {
	if !db.HasAGE {
		return nil
	}

	query := fmt.Sprintf(`
	DO $$
	BEGIN
		PERFORM ag_catalog.create_graph('%s');
	EXCEPTION
		WHEN duplicate_table THEN
			NULL;
		WHEN OTHERS THEN
			RAISE NOTICE 'create_graph notice: %%', SQLERRM;
	END $$;`, graphName)

	_, err := db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("ensure age graph %s exists: %w", graphName, err)
	}

	return nil
}

// ExecuteTx is a helper to run operations within a database transaction.
func (db *DB) ExecuteTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
