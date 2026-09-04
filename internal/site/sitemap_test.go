package site_test

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/Ivenfpeng/diary_blog/internal/site"
)

func TestSitemapIncludesOnlyPublishedPublicRoutes(t *testing.T) {
	generatedAt := time.Date(2026, 9, 4, 12, 30, 0, 0, time.UTC)
	sitemap, err := site.NewSitemap("https://diary.example", func() time.Time { return generatedAt })
	if err != nil {
		t.Fatal(err)
	}
	output, err := sitemap.Generate([]posts.PublishedPost{
		{Slug: "later", PublishedAt: generatedAt.Add(-time.Hour), Category: &posts.Taxonomy{Slug: "go", Name: "Go"}, Tags: []posts.Taxonomy{{Slug: "testing", Name: "Testing"}}},
		{Slug: "earlier", PublishedAt: generatedAt.Add(-2 * time.Hour), Category: &posts.Taxonomy{Slug: "go", Name: "Go"}, Tags: []posts.Taxonomy{{Slug: "testing", Name: "Testing"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Location string `xml:"loc"`
			LastMod  string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(output, &document); err != nil {
		t.Fatalf("sitemap is not valid XML: %v\n%s", err, output)
	}
	locations := make([]string, 0, len(document.URLs))
	for _, entry := range document.URLs {
		locations = append(locations, entry.Location)
	}
	for _, want := range []string{
		"https://diary.example/",
		"https://diary.example/archive",
		"https://diary.example/categories/go",
		"https://diary.example/tags/testing",
		"https://diary.example/posts/later",
		"https://diary.example/posts/earlier",
	} {
		if !contains(locations, want) {
			t.Fatalf("sitemap locations = %v, missing %q", locations, want)
		}
	}
	if contains(locations, "https://diary.example/posts/draft-only") || contains(locations, "https://diary.example/posts/archived-only") {
		t.Fatalf("sitemap leaked non-public routes: %v", locations)
	}
	if document.URLs[4].Location != "https://diary.example/posts/later" || document.URLs[4].LastMod != "2026-09-04" {
		t.Fatalf("published URLs were not deterministically ordered: %+v", document.URLs)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
