package posts_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func TestServicePublishValidatesAndRendersPost(t *testing.T) {
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	service := posts.NewService(sqliterepo.NewPostRepository(db), content.NewRenderer(), func() time.Time { return now })

	invalid, err := service.CreateDraft(ctx, content.PostInput{Slug: "missing-title", ContentMD: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, invalid.ID, invalid.Revision); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("missing title error = %v, want ErrValidation", err)
	}
	missingSlug, err := service.CreateDraft(ctx, content.PostInput{Title: "Missing Slug", ContentMD: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(ctx, missingSlug.ID, missingSlug.Revision); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("missing slug error = %v, want ErrValidation", err)
	}

	valid, err := service.CreateDraft(ctx, content.PostInput{Slug: "rendered", Title: "Rendered", ContentMD: "# 数据库\n\n<script>x</script>"})
	if err != nil {
		t.Fatal(err)
	}
	published, err := service.Publish(ctx, valid.ID, valid.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if published.Status != content.StatusPublished || published.ContentHTML == "" || strings.Contains(published.ContentHTML, "<script") {
		t.Fatalf("unexpected published post: %+v", published)
	}
}

func TestServiceRenderFailureLeavesPublishedPostUnchanged(t *testing.T) {
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 11, 0, 0, 0, time.UTC)
	repo := sqliterepo.NewPostRepository(db)
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	post, err := repo.CreateDraft(ctx, content.PostInput{Slug: "render-failure", Title: "Render Failure", ContentMD: "old"}, now)
	if err != nil {
		t.Fatal(err)
	}
	post, err = service.Publish(ctx, post.ID, post.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE posts SET content_md = ? WHERE id = ?", strings.Repeat("x", 2*1024*1024+1), post.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Publish(ctx, post.ID, post.Revision); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("render error = %v, want ErrValidation", err)
	}
	unchanged, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != post.Revision || unchanged.ContentHTML != post.ContentHTML || unchanged.Status != content.StatusPublished {
		t.Fatalf("render failure changed published row: %+v", unchanged)
	}
}

func TestServiceRejectsInvalidIdentifiersPaginationAndOversizedContent(t *testing.T) {
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	service := posts.NewService(sqliterepo.NewPostRepository(db), content.NewRenderer(), time.Now)

	if _, err := service.Get(ctx, 0); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("id error = %v, want ErrValidation", err)
	}
	if _, _, err := service.ListAdmin(ctx, posts.AdminFilter{Page: 0, PageSize: 10}); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("pagination error = %v, want ErrValidation", err)
	}
	if _, err := service.CreateDraft(ctx, content.PostInput{Slug: "too-large", Title: "Too Large", ContentMD: strings.Repeat("x", 2*1024*1024+1)}); !errors.Is(err, posts.ErrValidation) {
		t.Fatalf("content size error = %v, want ErrValidation", err)
	}
}
