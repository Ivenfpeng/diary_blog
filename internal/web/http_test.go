package web_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	web "github.com/Ivenfpeng/diary_blog/internal/web"
)

func TestPublicArticleRendersCanonicalSafeContentReadingTimeAndTOC(t *testing.T) {
	repo, published, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/posts/reading-safely")
	for _, want := range []string{
		`<link rel="canonical" href="` + server.URL + `/posts/reading-safely">`,
		`<title>Reading Safely`,
		`<p>Visible paragraph.</p>`,
		`1 min read`,
		`href="#getting-started"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("article response missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "alert('unsafe')") || strings.Contains(body, "Draft only") {
		t.Fatalf("article leaked unsafe or unpublished content\n%s", body)
	}
	if published.Status != content.StatusPublished {
		t.Fatalf("fixture post status = %q, want published", published.Status)
	}
}

func TestPublicListingRoutesRenderPublishedHTMLOnly(t *testing.T) {
	repo, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	for _, path := range []string{"/", "/categories/databases", "/tags/go", "/archive"} {
		t.Run(path, func(t *testing.T) {
			body := getHTML(t, server.URL+path)
			if !strings.Contains(body, "<!doctype html>") || !strings.Contains(body, "Reading Safely") {
				t.Fatalf("%s did not render full published HTML\n%s", path, body)
			}
			if strings.Contains(body, "Draft only") {
				t.Fatalf("%s leaked draft content\n%s", path, body)
			}
		})
	}
}

func TestPublicServerServesEmbeddedAssetsAndHealthChecks(t *testing.T) {
	repo, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	admin := getHTML(t, server.URL+"/admin")
	if !strings.Contains(admin, "Diary Blog admin") {
		t.Fatalf("admin entry point did not serve embedded index\n%s", admin)
	}
	for _, path := range []string{"/static/site.css", "/healthz", "/readyz"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
			t.Fatalf("GET %s status = %d, want success", path, response.StatusCode)
		}
		if response.Header.Get("X-Request-ID") == "" {
			t.Fatalf("GET %s did not receive a request ID", path)
		}
	}
}

func publicFixture(t *testing.T) (posts.Repository, content.Post, func()) {
	t.Helper()
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := appdb.Migrate(ctx, db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `INSERT INTO categories (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, "Databases", "databases", now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO tags (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, "Go", "go", now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	var categoryID, tagID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM categories WHERE slug = ?`, "databases").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM tags WHERE slug = ?`, "go").Scan(&tagID); err != nil {
		t.Fatal(err)
	}
	repo := sqliterepo.NewPostRepository(db)
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	published, err := service.CreateDraft(ctx, content.PostInput{
		Slug: "reading-safely", Title: "Reading Safely", Summary: "A published article.",
		ContentMD:  "# Getting started\n\nVisible paragraph.\n\n<script>alert('unsafe')</script>",
		CategoryID: &categoryID, TagIDs: []int64{tagID},
	})
	if err != nil {
		t.Fatal(err)
	}
	published, err = service.Publish(ctx, published.ID, published.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateDraft(ctx, content.PostInput{Slug: "draft-only", Title: "Draft only", ContentMD: "private"}); err != nil {
		t.Fatal(err)
	}
	return repo, published, func() { _ = db.Close() }
}

func getHTML(t *testing.T, url string) string {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", url, response.StatusCode, http.StatusOK)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("GET %s Content-Type = %q, want text/html", url, contentType)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
