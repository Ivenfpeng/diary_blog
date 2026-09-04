package site_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/Ivenfpeng/diary_blog/internal/site"
)

func TestRSSIncludesNewestPublishedPostsWithAbsoluteURLs(t *testing.T) {
	generatedAt := time.Date(2026, 9, 4, 12, 30, 0, 0, time.UTC)
	feed, err := site.NewRSS("https://diary.example/blog%20notes/", func() time.Time { return generatedAt })
	if err != nil {
		t.Fatal(err)
	}

	entries := make([]posts.PublishedPost, 0, 21)
	for index := 0; index < 21; index++ {
		entries = append(entries, posts.PublishedPost{
			Slug:        "post-" + twoDigits(index),
			Title:       "Post " + twoDigits(index),
			Summary:     "A & B",
			ContentHTML: "<p>Published body</p>",
			PublishedAt: generatedAt.Add(-time.Duration(index) * time.Hour),
		})
	}
	entries[0].Slug = "z-newest"
	entries[1].Slug = "a-newest"
	entries[1].PublishedAt = entries[0].PublishedAt

	output, err := feed.Generate(entries)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			LastBuildDate string `xml:"lastBuildDate"`
			Items         []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(output, &document); err != nil {
		t.Fatalf("RSS is not valid XML: %v\n%s", err, output)
	}
	if document.Channel.LastBuildDate != generatedAt.Format(time.RFC1123Z) {
		t.Fatalf("last build date = %q", document.Channel.LastBuildDate)
	}
	if len(document.Channel.Items) != 20 {
		t.Fatalf("RSS items = %d, want 20", len(document.Channel.Items))
	}
	if document.Channel.Items[0].Title != "Post 01" || document.Channel.Items[1].Title != "Post 00" {
		t.Fatalf("RSS order = %+v, want slug order for tied publication times", document.Channel.Items[:2])
	}
	if document.Channel.Items[0].Link != "https://diary.example/blog%20notes/posts/a-newest" {
		t.Fatalf("RSS first link = %q, want absolute canonical URL", document.Channel.Items[0].Link)
	}
	if strings.Contains(string(output), "post-20") {
		t.Fatalf("RSS included the oldest post beyond the limit:\n%s", output)
	}
}

func twoDigits(value int) string {
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}
