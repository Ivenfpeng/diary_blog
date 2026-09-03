package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	requiredTables := []string{
		"admins",
		"sessions",
		"categories",
		"tags",
		"media",
		"posts",
		"post_tags",
		"post_revisions",
		"site_settings",
		"posts_fts",
		"schema_migrations",
	}
	for _, table := range requiredTables {
		var count int
		if err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s count = %d, want 1", table, count)
		}
	}

	var migrationCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatal(err)
	}
	if migrationCount != 2 {
		t.Fatalf("migration count = %d, want 2", migrationCount)
	}
}

func TestMigratedDatabaseEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO posts (
			slug, title, status, category_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "missing-category", "Missing category", "draft", 999, "2026-09-03T00:00:00Z", "2026-09-03T00:00:00Z")
	if err == nil {
		t.Fatal("expected foreign key violation")
	}
}

func TestMigratedDatabaseSupportsChineseTrigramSearch(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO posts_fts (post_id, title, summary, content_plain)
		VALUES (?, ?, ?, ?)
	`, 1, "SQLite 数据库实践", "WAL 与事务", "使用 chi.Router 构建博客"); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM posts_fts WHERE posts_fts MATCH ?",
		"数据库",
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("search result count = %d, want 1", count)
	}
}
