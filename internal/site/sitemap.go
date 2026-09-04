package site

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/posts"
)

// Sitemap generates a sitemap from immutable published-post snapshots.
type Sitemap struct {
	baseURL *url.URL
	now     func() time.Time
}

// NewSitemap configures a sitemap generator with the public canonical URL and
// clock used for static public routes.
func NewSitemap(publicBaseURL string, now func() time.Time) (*Sitemap, error) {
	baseURL, err := parsePublicBaseURL(publicBaseURL)
	if err != nil {
		return nil, err
	}
	if now == nil {
		return nil, fmt.Errorf("sitemap clock is required")
	}
	return &Sitemap{baseURL: baseURL, now: now}, nil
}

// Generate emits public listing, taxonomy, and article routes. Its input is
// intentionally limited to PublishedPost, so draft and archived content has
// no path into a sitemap.
func (s *Sitemap) Generate(published []posts.PublishedPost) ([]byte, error) {
	staticLastMod := s.now().UTC().Format("2006-01-02")
	urls := []sitemapURL{
		{Location: absoluteURL(s.baseURL), LastMod: staticLastMod},
		{Location: absoluteURL(s.baseURL, "archive"), LastMod: staticLastMod},
	}
	categories := newestTaxonomy(published, func(post posts.PublishedPost) *posts.Taxonomy { return post.Category })
	tags := newestTags(published)
	for _, taxonomy := range categories {
		urls = append(urls, sitemapURL{Location: absoluteURL(s.baseURL, "categories", taxonomy.slug), LastMod: taxonomy.lastMod.Format("2006-01-02")})
	}
	for _, taxonomy := range tags {
		urls = append(urls, sitemapURL{Location: absoluteURL(s.baseURL, "tags", taxonomy.slug), LastMod: taxonomy.lastMod.Format("2006-01-02")})
	}
	for _, post := range sortedPosts(published) {
		urls = append(urls, sitemapURL{Location: absoluteURL(s.baseURL, "posts", post.Slug), LastMod: post.PublishedAt.UTC().Format("2006-01-02")})
	}
	return marshalXML(sitemapDocument{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls})
}

type sitemapDocument struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Location string `xml:"loc"`
	LastMod  string `xml:"lastmod"`
}

type taxonomyLastMod struct {
	slug    string
	lastMod time.Time
}

func newestTaxonomy(source []posts.PublishedPost, selectTaxonomy func(posts.PublishedPost) *posts.Taxonomy) []taxonomyLastMod {
	values := make(map[string]time.Time)
	for _, post := range source {
		if taxonomy := selectTaxonomy(post); taxonomy != nil && post.PublishedAt.After(values[taxonomy.Slug]) {
			values[taxonomy.Slug] = post.PublishedAt
		}
	}
	return sortedTaxonomy(values)
}

func newestTags(source []posts.PublishedPost) []taxonomyLastMod {
	values := make(map[string]time.Time)
	for _, post := range source {
		for _, tag := range post.Tags {
			if post.PublishedAt.After(values[tag.Slug]) {
				values[tag.Slug] = post.PublishedAt
			}
		}
	}
	return sortedTaxonomy(values)
}

func sortedTaxonomy(values map[string]time.Time) []taxonomyLastMod {
	result := make([]taxonomyLastMod, 0, len(values))
	for slug, lastMod := range values {
		result = append(result, taxonomyLastMod{slug: slug, lastMod: lastMod})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].slug < result[right].slug })
	return result
}
