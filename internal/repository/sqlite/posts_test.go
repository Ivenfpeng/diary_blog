package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func TestPostRepositoryCreatesAndLoadsDraft(t *testing.T) {
	ctx := context.Background()
	repo, db := newPostRepository(t)
	categoryID, tagIDs := seedTaxonomy(t, db, 2)
	now := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	input := content.PostInput{
		Slug:       "sqlite-notes",
		Title:      "SQLite Notes",
		Summary:    "Transactions and WAL",
		ContentMD:  "# SQLite\n\nReliable storage.",
		CategoryID: &categoryID,
		TagIDs:     tagIDs,
	}

	created, err := repo.CreateDraft(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Status != content.StatusDraft || created.Revision != 1 {
		t.Fatalf("unexpected created post: %+v", created)
	}
	if !reflect.DeepEqual(created.TagIDs, tagIDs) {
		t.Fatalf("TagIDs = %v, want %v", created.TagIDs, tagIDs)
	}
	if !created.CreatedAt.Equal(now) || !created.UpdatedAt.Equal(now) {
		t.Fatalf("unexpected timestamps: %+v", created)
	}

	loaded, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, created) {
		t.Fatalf("loaded = %+v, want %+v", loaded, created)
	}

	if _, err := repo.CreateDraft(ctx, input, now); err == nil {
		t.Fatal("expected duplicate slug error")
	}
}

func TestPostRepositorySavesRevisionAndRejectsStaleUpdate(t *testing.T) {
	ctx := context.Background()
	repo, db := newPostRepository(t)
	_, tagIDs := seedTaxonomy(t, db, 2)
	now := time.Date(2026, 9, 3, 11, 0, 0, 0, time.UTC)
	original := content.PostInput{
		Slug:      "versioned-post",
		Title:     "Version One",
		Summary:   "Original",
		ContentMD: "First body",
		TagIDs:    tagIDs[:1],
	}
	created, err := repo.CreateDraft(ctx, original, now)
	if err != nil {
		t.Fatal(err)
	}

	updatedInput := content.PostInput{
		Slug:      original.Slug,
		Title:     "Version Two",
		Summary:   "Updated",
		ContentMD: "Second body",
		TagIDs:    tagIDs[1:],
	}
	updated, err := repo.SaveDraft(ctx, created.ID, updatedInput, 1, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Title != "Version Two" {
		t.Fatalf("unexpected updated post: %+v", updated)
	}
	if !reflect.DeepEqual(updated.TagIDs, tagIDs[1:]) {
		t.Fatalf("TagIDs = %v, want %v", updated.TagIDs, tagIDs[1:])
	}

	revisions, err := repo.ListRevisions(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revision count = %d, want 1", len(revisions))
	}
	if revisions[0].Revision != 1 || revisions[0].Title != original.Title || revisions[0].ContentMD != original.ContentMD {
		t.Fatalf("unexpected revision: %+v", revisions[0])
	}
	if !reflect.DeepEqual(revisions[0].TagIDs, original.TagIDs) {
		t.Fatalf("revision TagIDs = %v, want %v", revisions[0].TagIDs, original.TagIDs)
	}

	_, err = repo.SaveDraft(ctx, created.ID, updatedInput, 1, now.Add(2*time.Minute))
	if !errors.Is(err, posts.ErrConflict) {
		t.Fatalf("stale update error = %v, want ErrConflict", err)
	}
}

func TestPostRepositoryRetainsNewestThirtyRevisions(t *testing.T) {
	ctx := context.Background()
	repo, db := newPostRepository(t)
	_, tagIDs := seedTaxonomy(t, db, 3)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	input := content.PostInput{
		Slug:      "long-history",
		Title:     "Version 0",
		ContentMD: "Body 0",
		TagIDs:    tagIDs[:1],
	}
	post, err := repo.CreateDraft(ctx, input, now)
	if err != nil {
		t.Fatal(err)
	}

	for version := 1; version <= 31; version++ {
		input.Title = fmt.Sprintf("Version %d", version)
		input.ContentMD = fmt.Sprintf("Body %d", version)
		input.TagIDs = []int64{tagIDs[version%len(tagIDs)]}
		post, err = repo.SaveDraft(ctx, post.ID, input, post.Revision, now.Add(time.Duration(version)*time.Minute))
		if err != nil {
			t.Fatalf("save version %d: %v", version, err)
		}
	}

	revisions, err := repo.ListRevisions(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 30 {
		t.Fatalf("revision count = %d, want 30", len(revisions))
	}
	if revisions[0].Revision != 31 || revisions[len(revisions)-1].Revision != 2 {
		t.Fatalf("retained revisions = %d..%d, want 31..2", revisions[0].Revision, revisions[len(revisions)-1].Revision)
	}
	if !reflect.DeepEqual(post.TagIDs, input.TagIDs) {
		t.Fatalf("final TagIDs = %v, want %v", post.TagIDs, input.TagIDs)
	}
}

func TestPostRepositoryRestoresRevision(t *testing.T) {
	ctx := context.Background()
	repo, db := newPostRepository(t)
	_, tagIDs := seedTaxonomy(t, db, 2)
	now := time.Date(2026, 9, 3, 13, 0, 0, 0, time.UTC)
	original := content.PostInput{
		Slug:      "restore-me",
		Title:     "Original",
		Summary:   "First summary",
		ContentMD: "First body",
		TagIDs:    tagIDs[:1],
	}
	post, err := repo.CreateDraft(ctx, original, now)
	if err != nil {
		t.Fatal(err)
	}

	revisedInput := content.PostInput{
		Slug:      original.Slug,
		Title:     "Revised",
		Summary:   "Second summary",
		ContentMD: "Second body",
		TagIDs:    tagIDs[1:],
	}
	post, err = repo.SaveDraft(ctx, post.ID, revisedInput, post.Revision, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	revisions, err := repo.ListRevisions(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}

	restored, err := repo.RestoreRevision(ctx, post.ID, revisions[0].ID, post.Revision, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Revision != 3 || restored.Title != original.Title || restored.ContentMD != original.ContentMD {
		t.Fatalf("unexpected restored post: %+v", restored)
	}
	if !reflect.DeepEqual(restored.TagIDs, original.TagIDs) {
		t.Fatalf("restored TagIDs = %v, want %v", restored.TagIDs, original.TagIDs)
	}

	revisions, err = repo.ListRevisions(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0].Revision != 2 || revisions[0].Title != revisedInput.Title {
		t.Fatalf("unexpected revisions after restore: %+v", revisions)
	}
}

func newPostRepository(t *testing.T) (*sqliterepo.PostRepository, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return sqliterepo.NewPostRepository(db), db
}

func seedTaxonomy(t *testing.T, db *sql.DB, tagCount int) (int64, []int64) {
	t.Helper()
	ctx := context.Background()
	now := "2026-09-03T00:00:00Z"
	result, err := db.ExecContext(ctx,
		"INSERT INTO categories (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)",
		"Database", "database", now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	tagIDs := make([]int64, 0, tagCount)
	for i := 1; i <= tagCount; i++ {
		result, err := db.ExecContext(ctx,
			"INSERT INTO tags (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)",
			fmt.Sprintf("Tag %d", i), fmt.Sprintf("tag-%d", i), now, now,
		)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		tagIDs = append(tagIDs, id)
	}
	return categoryID, tagIDs
}
