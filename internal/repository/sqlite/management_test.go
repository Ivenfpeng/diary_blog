package sqlite_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func TestTaxonomyRejectsDuplicateSlugsAndReferencedCategoryDeletion(t *testing.T) {
	ctx := context.Background()
	repo, _ := newPostRepository(t)
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	category, err := repo.CreateCategory(ctx, sqliterepo.TaxonomyInput{Name: "Engineering", Slug: "engineering"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag(ctx, sqliterepo.TaxonomyInput{Name: "Duplicate", Slug: "engineering"}, now); err == nil {
		t.Fatal("expected globally unique taxonomy slug")
	}
	post, err := repo.CreateDraft(ctx, content.PostInput{Slug: "category-post", Title: "Category post", ContentMD: "body", CategoryID: &category.ID}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteCategory(ctx, category.ID); !errors.Is(err, sqliterepo.ErrReferenced) {
		t.Fatalf("delete referenced category = %v, want ErrReferenced", err)
	}
	if _, err := repo.GetByID(ctx, post.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMediaMetadataAndSettingsValidation(t *testing.T) {
	ctx := context.Background()
	repo, _ := newPostRepository(t)
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	media, err := repo.CreateMedia(ctx, sqliterepo.MediaInput{Path: "2026/09/image.png", MIMEType: "image/png", Width: 4, Height: 3, Size: 12, AltText: "A tiny image"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if media.Width != 4 || media.Height != 3 || media.AltText != "A tiny image" {
		t.Fatalf("media metadata = %+v", media)
	}
	if _, err := repo.CreateMedia(ctx, sqliterepo.MediaInput{Path: "bad", MIMEType: "text/plain", Width: 1, Height: 1, Size: 1}, now); !errors.Is(err, sqliterepo.ErrValidation) {
		t.Fatalf("invalid media = %v, want validation", err)
	}
	if _, err := repo.SaveSettings(ctx, sqliterepo.Settings{SiteTitle: "", Description: "desc"}, now); !errors.Is(err, sqliterepo.ErrValidation) {
		t.Fatalf("invalid settings = %v, want validation", err)
	}
	saved, err := repo.SaveSettings(ctx, sqliterepo.Settings{SiteTitle: "Diary", Description: "Notes", Author: "Iven"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SiteTitle != "Diary" {
		t.Fatalf("saved settings = %+v", saved)
	}
	if _, err := repo.GetSettings(ctx); err != nil {
		t.Fatal(err)
	}
}
