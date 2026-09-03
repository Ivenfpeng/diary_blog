package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"

	blogmigrations "github.com/Ivenfpeng/diary_blog/migrations"
)

const createMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	filenames, err := fs.Glob(blogmigrations.Files, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		if err := applyMigration(ctx, db, filename); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, filename string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", filename, err)
	}
	defer func() { _ = tx.Rollback() }()

	var applied int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?",
		filename,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", filename, err)
	}
	if applied == 1 {
		return tx.Commit()
	}

	contents, err := blogmigrations.Files.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", filename, err)
	}
	if _, err := tx.ExecContext(ctx, string(contents)); err != nil {
		return fmt.Errorf("execute migration %s: %w", filename, err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))",
		filename,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", filename, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", filename, err)
	}
	return nil
}
