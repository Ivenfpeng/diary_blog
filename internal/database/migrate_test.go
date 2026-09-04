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
	if migrationCount != 6 {
		t.Fatalf("migration count = %d, want 6", migrationCount)
	}
	var singletonIndexCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_admins_singleton'").Scan(&singletonIndexCount); err != nil {
		t.Fatal(err)
	}
	if singletonIndexCount != 1 {
		t.Fatalf("singleton administrator index count = %d, want 1", singletonIndexCount)
	}
}

func TestSessionTimestampMigrationPreservesRFC3339NanoFractions(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{"001_initial.sql", "002_search.sql", "003_publication_snapshot.sql", "004_published_cover_media.sql", "005_single_administrator.sql"} {
		if err := applyMigration(ctx, db, migration); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO admins (id, username, password_hash, created_at, updated_at)
		VALUES (1, 'admin', 'hash', '2026-09-04T12:00:00Z', '2026-09-04T12:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	expiries := []string{"2026-09-04T12:00:00.1Z", "2026-09-04T12:00:00.12Z", "2026-09-04T12:00:00.123456789+02:30"}
	lastSeen := "2026-09-04T11:00:00.01Z"
	created := "2026-09-04T10:00:00.001Z"
	for index, expiresAt := range expiries {
		token := make([]byte, 32)
		token[0] = byte(index + 1)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO sessions (token_hash, csrf_hash, admin_id, expires_at, last_seen_at, created_at)
			VALUES (?, ?, 1, ?, ?, ?)`, token, make([]byte, 32), expiresAt, lastSeen, created); err != nil {
			t.Fatal(err)
		}
	}
	if err := applyMigration(ctx, db, "006_session_unix_timestamps.sql"); err != nil {
		t.Fatal(err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT typeof(expires_at), expires_at,
		       typeof(last_seen_at), last_seen_at,
		       typeof(created_at), created_at
		FROM sessions ORDER BY token_hash`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wants := expiries
	wantLastSeen, err := time.Parse(time.RFC3339Nano, lastSeen)
	if err != nil {
		t.Fatal(err)
	}
	wantCreated, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		t.Fatal(err)
	}
	index := 0
	for rows.Next() {
		var expiryType, lastSeenType, createdType string
		var storedExpiry, storedLastSeen, storedCreated int64
		if err := rows.Scan(&expiryType, &storedExpiry, &lastSeenType, &storedLastSeen, &createdType, &storedCreated); err != nil {
			t.Fatal(err)
		}
		wantExpiry, err := time.Parse(time.RFC3339Nano, wants[index])
		if err != nil {
			t.Fatal(err)
		}
		if expiryType != "integer" || storedExpiry != wantExpiry.UnixNano() {
			t.Fatalf("migrated expiry %d type/value = %s/%d, want integer/%d", index, expiryType, storedExpiry, wantExpiry.UnixNano())
		}
		if lastSeenType != "integer" || storedLastSeen != wantLastSeen.UnixNano() {
			t.Fatalf("migrated last-seen %d type/value = %s/%d, want integer/%d", index, lastSeenType, storedLastSeen, wantLastSeen.UnixNano())
		}
		if createdType != "integer" || storedCreated != wantCreated.UnixNano() {
			t.Fatalf("migrated created %d type/value = %s/%d, want integer/%d", index, createdType, storedCreated, wantCreated.UnixNano())
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(wants) {
		t.Fatalf("migrated session count = %d, want %d", index, len(wants))
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
