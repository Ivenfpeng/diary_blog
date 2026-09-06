package web_test

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/site"
	web "github.com/Ivenfpeng/diary_blog/internal/web"
)

const defaultPublicURL = "http://localhost:8080"

func TestPublicHomeUsesCacheUntilPublicationInvalidation(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	cache := site.NewCache(time.Hour)
	cache.Set(site.HomeCacheKey(), []byte("cached home"))
	server := httptest.NewServer(web.NewServer(repo, web.ServerOptions{Cache: cache}))
	t.Cleanup(server.Close)

	if body := getHTML(t, server.URL+"/"); body != "cached home" {
		t.Fatalf("cached home = %q, want cached response", body)
	}
	cache.InvalidatePublication("reading-safely")
	if body := getHTML(t, server.URL+"/"); !strings.Contains(body, "Reading Safely") {
		t.Fatalf("home after invalidation did not render repository data: %s", body)
	}
}

func TestPublicCacheDoesNotRetainUnknownTaxonomyPages(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	cache := site.NewCache(time.Hour)
	server := httptest.NewServer(web.NewServer(repo, web.ServerOptions{Cache: cache}))
	t.Cleanup(server.Close)

	for _, path := range []string{"/categories/missing-one", "/categories/missing-two", "/tags/missing-three"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, response.StatusCode)
		}
	}
	if cache.EntryCount() != 0 {
		t.Fatalf("cache retained %d unknown taxonomy pages", cache.EntryCount())
	}
}

func TestPublicCacheDoesNotResurrectContentInvalidatedDuringRender(t *testing.T) {
	repo, published, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	cache := site.NewCache(time.Hour)
	blocking := &blockingListRepository{Repository: repo, started: make(chan struct{}), release: make(chan struct{})}
	server := httptest.NewServer(web.NewServer(blocking, web.ServerOptions{Cache: cache}))
	t.Cleanup(server.Close)

	firstBody := make(chan string, 1)
	go func() {
		response, err := http.Get(server.URL + "/")
		if err != nil {
			firstBody <- "request error: " + err.Error()
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		firstBody <- string(body)
	}()
	<-blocking.started
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC) })
	if _, err := service.Archive(context.Background(), published.ID, published.Revision); err != nil {
		t.Fatal(err)
	}
	cache.InvalidatePublication(published.Slug)
	close(blocking.release)
	if body := <-firstBody; !strings.Contains(body, "Reading Safely") {
		t.Fatalf("in-flight response did not contain its original snapshot: %s", body)
	}
	if body := getHTML(t, server.URL+"/"); strings.Contains(body, "Reading Safely") {
		t.Fatalf("next request received stale content inserted after invalidation: %s", body)
	}
}

func TestPublicArticleRendersCanonicalSafeContentReadingTimeAndTOC(t *testing.T) {
	repo, published, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/posts/reading-safely")
	for _, want := range []string{
		`<link rel="canonical" href="` + defaultPublicURL + `/posts/reading-safely">`,
		`<title>Reading Safely`,
		`<meta name="description" content="A published article.">`,
		`<meta name="robots" content="index,follow">`,
		`<meta property="og:title" content="Reading Safely">`,
		`<meta property="og:description" content="A published article.">`,
		`<meta property="og:url" content="` + defaultPublicURL + `/posts/reading-safely">`,
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
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	for _, path := range []string{"/", "/categories/databases", "/tags/go", "/archive"} {
		t.Run(path, func(t *testing.T) {
			body := getHTML(t, server.URL+path)
			for _, want := range []string{
				"<!doctype html>", "Reading Safely",
				`<meta name="description" content=`,
				`<meta name="robots" content="index,follow">`,
				`<link rel="canonical" href="` + defaultPublicURL + path + `">`,
				`<meta property="og:title" content=`,
				`<meta property="og:description" content=`,
				`<meta property="og:url" content="` + defaultPublicURL + path + `">`,
				`<meta property="og:type" content="website">`,
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("%s did not render public SEO field %q\n%s", path, want, body)
				}
			}
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
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	admin := getHTML(t, server.URL+"/admin")
	if !strings.Contains(admin, "Diary Blog Admin") {
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

func TestAdminClientRoutesServeTheSPAEntryPoint(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	for _, path := range []string{"/admin/login", "/admin/posts/42"} {
		body := getHTML(t, server.URL+path)
		if !strings.Contains(body, "Diary Blog Admin") {
			t.Fatalf("GET %s did not serve the admin entry point\n%s", path, body)
		}
	}
}

func TestPublicPublicationSnapshotSurvivesUnpublishedEdits(t *testing.T) {
	repo, published, db, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO categories (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, "Operations", "operations", now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tags (name, slug, created_at, updated_at) VALUES (?, ?, ?, ?)`, "Linux", "linux", now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	var categoryID, tagID int64
	if err := db.QueryRow(`SELECT id FROM categories WHERE slug = ?`, "operations").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM tags WHERE slug = ?`, "linux").Scan(&tagID); err != nil {
		t.Fatal(err)
	}
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	if _, err := service.SaveDraft(context.Background(), published.ID, content.PostInput{
		Slug: "unpublished-rewrite", Title: "Unpublished rewrite", Summary: "Do not expose this.",
		ContentMD: "# Replacement heading\n\nReplacement body.", CategoryID: &categoryID, TagIDs: []int64{tagID},
	}, published.Revision); err != nil {
		t.Fatal(err)
	}

	article := getHTML(t, server.URL+"/posts/reading-safely")
	for _, want := range []string{"Reading Safely", "A published article.", "Visible paragraph.", `href="#getting-started"`, `href="/categories/databases">Databases`, `href="/tags/go">Go`} {
		if !strings.Contains(article, want) {
			t.Fatalf("publication snapshot missing %q\n%s", want, article)
		}
	}
	for _, forbidden := range []string{"Unpublished rewrite", "Replacement body.", "replacement-heading", "operations", "linux"} {
		if strings.Contains(article, forbidden) {
			t.Fatalf("publication snapshot leaked %q\n%s", forbidden, article)
		}
	}
	response, err := http.Get(server.URL + "/posts/unpublished-rewrite")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unpublished slug status = %d, want 404", response.StatusCode)
	}
	home := getHTML(t, server.URL+"/")
	if !strings.Contains(home, "Reading Safely") || strings.Contains(home, "Unpublished rewrite") {
		t.Fatalf("home did not retain publication snapshot\n%s", home)
	}
	for _, path := range []string{"/categories/databases", "/tags/go"} {
		listing := getHTML(t, server.URL+path)
		if !strings.Contains(listing, "Reading Safely") || strings.Contains(listing, "Unpublished rewrite") {
			t.Fatalf("%s did not retain publication snapshot\n%s", path, listing)
		}
	}
}

func TestPublicNavigationAndMobileTOCAreUsableWithoutJavaScript(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/posts/reading-safely")
	for _, want := range []string{
		`<details class="mobile-navigation">`,
		`<a href="/categories/databases">Databases</a>`,
		`<a href="/tags/go">Go</a>`,
		`<details class="mobile-table-of-contents">`,
		`href="#getting-started"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("mobile no-JS navigation/TOC missing %q\n%s", want, body)
		}
	}
}

func TestPublicCanonicalURLUsesSafeDefault(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	server := httptest.NewTLSServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL + "/posts/reading-safely")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `<link rel="canonical" href="`+defaultPublicURL+`/posts/reading-safely">`) {
		t.Fatalf("response did not use the safe configured canonical default\n%s", body)
	}
}

func TestPublicArticleUsesPublishedCoverMediaForOpenGraph(t *testing.T) {
	repo, _, db, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	result, err := db.Exec(`INSERT INTO media (path, mime_type, width, height, size, alt_text, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "2026/09/cover.png", "image/png", 1200, 630, 1, "Cover", now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	coverID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	post, err := service.CreateDraft(context.Background(), content.PostInput{
		Slug: "with-cover", Title: "With cover", Summary: "Published cover description.", ContentMD: "body", CoverMediaID: &coverID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), post.ID, post.Revision); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/posts/with-cover")
	if !strings.Contains(body, `<meta property="og:image" content="`+defaultPublicURL+`/media/2026/09/cover.png">`) {
		t.Fatalf("article response did not include the published cover as og:image\n%s", body)
	}
}

func TestPublicURLsUseConfiguredOriginDespiteHostAndForwardedHeaders(t *testing.T) {
	repo, _, db, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	result, err := db.Exec(`INSERT INTO media (path, mime_type, width, height, size, alt_text, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "2026/09/cover.png", "image/png", 1200, 630, 1, "Cover", now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	coverID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	post, err := service.CreateDraft(context.Background(), content.PostInput{
		Slug: "configured-origin", Title: "Configured origin", Summary: "Uses the configured origin.", ContentMD: "body", CoverMediaID: &coverID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), post.ID, post.Revision); err != nil {
		t.Fatal(err)
	}
	const publicURL = "https://diary.example/blog%20notes/"
	server := httptest.NewServer(web.NewServer(repo, web.ServerOptions{PublicURL: publicURL}))
	t.Cleanup(server.Close)

	for _, check := range []struct {
		path  string
		wants []string
	}{
		{path: "/posts/configured-origin", wants: []string{
			`<link rel="canonical" href="https://diary.example/blog%20notes/posts/configured-origin">`,
			`<meta property="og:url" content="https://diary.example/blog%20notes/posts/configured-origin">`,
			`<meta property="og:image" content="https://diary.example/blog%20notes/media/2026/09/cover.png">`,
		}},
		{path: "/search?q=Configured", wants: []string{
			`<link rel="canonical" href="https://diary.example/blog%20notes/search">`,
			`<meta property="og:url" content="https://diary.example/blog%20notes/search">`,
		}},
		{path: "/rss.xml", wants: []string{"https://diary.example/blog%20notes/posts/configured-origin"}},
		{path: "/sitemap.xml", wants: []string{"https://diary.example/blog%20notes/posts/configured-origin"}},
	} {
		t.Run(check.path, func(t *testing.T) {
			body := hostileBody(t, server.URL+check.path)
			for _, want := range check.wants {
				if !strings.Contains(body, want) {
					t.Fatalf("GET %s missing configured public URL %q\n%s", check.path, want, body)
				}
			}
			if strings.Contains(body, "attacker.example") {
				t.Fatalf("GET %s trusted request origin\n%s", check.path, body)
			}
		})
	}
}

func TestSearchEscapesSnippetsAndMarksResultPagesNoIndex(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	unsafe, err := service.CreateDraft(context.Background(), content.PostInput{
		Slug: "unsafe-snippet", Title: "Unsafe snippet", Summary: "Summary <script>alert('unsafe')</script>", ContentMD: "safe body",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), unsafe.ID, unsafe.Revision); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/search?q=Unsafe")
	for _, want := range []string{
		`<meta name="robots" content="noindex,follow">`,
		`<link rel="canonical" href="` + defaultPublicURL + `/search">`,
		`<h1>Search</h1>`,
		`Unsafe snippet`,
		`&lt;script&gt;`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("search response missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, `<script>alert('unsafe')</script>`) {
		t.Fatalf("search response rendered an unsafe snippet\n%s", body)
	}
}

func TestEmptySearchRendersInstructionsWithoutFTS(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	spy := &searchSpyRepository{Repository: repo}
	server := httptest.NewServer(web.NewServer(spy))
	t.Cleanup(server.Close)

	body := getHTML(t, server.URL+"/search?q=%20%20")
	if !strings.Contains(body, "Enter a word or phrase to search published articles.") {
		t.Fatalf("empty search did not render instructions\n%s", body)
	}
	if spy.searchCalls != 0 {
		t.Fatalf("empty search called FTS %d times", spy.searchCalls)
	}
}

func TestPublicListingsAndSearchPaginateBeyondTwentyArticles(t *testing.T) {
	repo, _, db, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	publishPaginationPosts(t, repo, db, 21)
	cache := site.NewCache(time.Hour)
	server := httptest.NewServer(web.NewServer(repo, web.ServerOptions{Cache: cache}))
	t.Cleanup(server.Close)

	checks := []struct {
		path         string
		canonical    string
		previousHref string
	}{
		{"/?page=2", defaultPublicURL + "/?page=2", `href="/"`},
		{"/archive?page=2", defaultPublicURL + "/archive?page=2", `href="/archive"`},
		{"/categories/databases?page=2", defaultPublicURL + "/categories/databases?page=2", `href="/categories/databases"`},
		{"/tags/go?page=2", defaultPublicURL + "/tags/go?page=2", `href="/tags/go"`},
		{"/search?q=paginationtoken&page=2", defaultPublicURL + "/search?page=2", `href="/search?q=paginationtoken"`},
	}
	for _, check := range checks {
		t.Run(check.path, func(t *testing.T) {
			body := getHTML(t, server.URL+check.path)
			for _, want := range []string{"Pagination article 00", `rel="prev"`, check.previousHref, `<link rel="canonical" href="` + check.canonical + `">`} {
				if !strings.Contains(body, want) {
					t.Fatalf("GET %s missing %q\n%s", check.path, want, body)
				}
			}
		})
	}
	if cache.EntryCount() != 4 {
		t.Fatalf("listing cache entries = %d, want one page-aware entry per cached listing", cache.EntryCount())
	}
}

func TestPublicPaginationRejectsInvalidPage(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	cache := site.NewCache(time.Hour)
	server := httptest.NewServer(web.NewServer(repo, web.ServerOptions{Cache: cache}))
	t.Cleanup(server.Close)
	_ = getHTML(t, server.URL+"/")
	for _, path := range []string{"/?page=0", "/archive?page=word", "/search?q=Go&page=-1"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want 400", path, response.StatusCode)
		}
	}
}

func TestRSSAndSitemapOnlyExposePublishedPosts(t *testing.T) {
	repo, _, _, closeDB := publicFixture(t)
	t.Cleanup(closeDB)
	now := time.Date(2026, 9, 4, 13, 0, 0, 0, time.UTC)
	service := posts.NewService(repo, content.NewRenderer(), func() time.Time { return now })
	archived, err := service.CreateDraft(context.Background(), content.PostInput{Slug: "archived-only", Title: "Archived only", ContentMD: "private"})
	if err != nil {
		t.Fatal(err)
	}
	archived, err = service.Publish(context.Background(), archived.ID, archived.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Archive(context.Background(), archived.ID, archived.Revision); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(web.NewServer(repo))
	t.Cleanup(server.Close)

	for _, path := range []string{"/rss.xml", "/sitemap.xml"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "application/xml") {
			t.Fatalf("GET %s status/content type = %d/%q", path, response.StatusCode, response.Header.Get("Content-Type"))
		}
		if !strings.Contains(string(body), defaultPublicURL+"/posts/reading-safely") || strings.Contains(string(body), "draft-only") || strings.Contains(string(body), "archived-only") {
			t.Fatalf("GET %s did not expose only published posts\n%s", path, body)
		}
	}
}

type searchSpyRepository struct {
	posts.Repository
	searchCalls int
}

type blockingListRepository struct {
	posts.Repository
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingListRepository) ListPublished(ctx context.Context, filter posts.PublishedFilter) ([]posts.PublishedPost, int, error) {
	items, total, err := r.Repository.ListPublished(ctx, filter)
	if err == nil && filter.Page == 1 {
		r.once.Do(func() {
			close(r.started)
			<-r.release
		})
	}
	return items, total, err
}

func publishPaginationPosts(t *testing.T, repository posts.Repository, db *sql.DB, count int) {
	t.Helper()
	var categoryID, tagID int64
	if err := db.QueryRow(`SELECT id FROM categories WHERE slug = ?`, "databases").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT id FROM tags WHERE slug = ?`, "go").Scan(&tagID); err != nil {
		t.Fatal(err)
	}
	service := posts.NewService(repository, content.NewRenderer(), func() time.Time { return time.Date(2026, 9, 4, 14, 0, 0, 0, time.UTC) })
	for index := 0; index < count; index++ {
		suffix := string(rune('a'+index/26)) + string(rune('a'+index%26))
		post, err := service.CreateDraft(context.Background(), content.PostInput{
			Slug: "pagination-" + suffix, Title: "Pagination article " + twoDigits(index),
			Summary: "paginationtoken", ContentMD: "paginationtoken", CategoryID: &categoryID, TagIDs: []int64{tagID},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Publish(context.Background(), post.ID, post.Revision); err != nil {
			t.Fatal(err)
		}
	}
}

func twoDigits(value int) string {
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

func (r *searchSpyRepository) SearchPublished(ctx context.Context, query string, page, pageSize int) ([]posts.PublishedPost, int, error) {
	r.searchCalls++
	return r.Repository.SearchPublished(ctx, query, page, pageSize)
}

func publicFixture(t *testing.T) (posts.Repository, content.Post, *sql.DB, func()) {
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
	return repo, published, db, func() { _ = db.Close() }
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

func hostileBody(t *testing.T, target string) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "attacker.example"
	request.Header.Set("X-Forwarded-Host", "attacker.example")
	request.Header.Set("X-Forwarded-Proto", "http")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", target, response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
