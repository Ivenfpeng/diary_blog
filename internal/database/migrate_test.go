package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
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
		"published_posts",
		"published_post_categories",
		"published_post_tags",
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
	if migrationCount != 4 {
		t.Fatalf("migration count = %d, want 4", migrationCount)
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

func TestMigrationToPublicationSnapshotsLeavesAmbiguousLegacyRowsPrivate(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{"001_initial.sql", "002_search.sql"} {
		if err := applyMigration(ctx, db, migration); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO posts (slug, title, summary, content_md, content_html, content_plain, status, revision, published_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mutable-draft", "Mutable draft title", "Mutable draft summary", "# Mutable draft", "<p>Legacy rendered body</p>", "Legacy rendered body",
		content.StatusPublished, 1, now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO posts_fts (post_id, title, summary, content_plain) VALUES (?, ?, ?, ?)`, 1, "Legacy public title", "Legacy summary", "Legacy rendered body"); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := sqliterepo.NewPostRepository(db)
	if _, err := repo.GetPublishedBySlug(ctx, "mutable-draft"); !errors.Is(err, posts.ErrNotFound) {
		t.Fatalf("legacy row became public: %v", err)
	}
	if results, total, err := repo.SearchPublished(ctx, "Legacy", 1, 10); err != nil || total != 0 || len(results) != 0 {
		t.Fatalf("legacy row appeared in public search: total=%d results=%+v err=%v", total, results, err)
	}
	var ftsCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM posts_fts WHERE post_id = 1`).Scan(&ftsCount); err != nil {
		t.Fatal(err)
	}
	if ftsCount != 1 {
		t.Fatalf("migration changed legacy FTS rows: count=%d, want 1", ftsCount)
	}

	legacy, err := repo.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(ctx, legacy.ID, content.RenderedContent{HTML: "<p>Explicitly republished</p>", PlainText: "Explicitly republished"}, legacy.Revision, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if published, err := repo.GetPublishedBySlug(ctx, "mutable-draft"); err != nil || published.Title != "Mutable draft title" {
		t.Fatalf("republished legacy row = %+v, err=%v", published, err)
	}
}
