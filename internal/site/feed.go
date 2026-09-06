// Package site produces deterministic public-site documents.
package site

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/posts"
)

const rssLimit = 20

// RSS generates a feed for the immutable posts published by the site.
type RSS struct {
	baseURL     *url.URL
	now         func() time.Time
	title       string
	description string
	author      string
}

// NewRSS configures an RSS generator with the public canonical URL and a
// clock. Keeping both dependencies explicit makes output repeatable.
func NewRSS(publicBaseURL string, now func() time.Time) (*RSS, error) {
	return NewRSSWithIdentity(publicBaseURL, now, "Diary Blog", "Diary Blog RSS feed", "")
}

func NewRSSWithIdentity(publicBaseURL string, now func() time.Time, title, description, author string) (*RSS, error) {
	baseURL, err := parsePublicBaseURL(publicBaseURL)
	if err != nil {
		return nil, err
	}
	if now == nil {
		return nil, fmt.Errorf("RSS clock is required")
	}
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		title = "Diary Blog"
	}
	if description == "" {
		description = title + " RSS feed"
	}
	return &RSS{baseURL: baseURL, now: now, title: title, description: description, author: strings.TrimSpace(author)}, nil
}

// Generate emits the newest twenty published entries in a stable order.
func (r *RSS) Generate(postsToPublish []posts.PublishedPost) ([]byte, error) {
	items := sortedPosts(postsToPublish)
	if len(items) > rssLimit {
		items = items[:rssLimit]
	}
	document := rssDocument{Version: "2.0"}
	document.Channel = rssChannel{
		Title:          r.title,
		Link:           absoluteURL(r.baseURL),
		Description:    r.description,
		ManagingEditor: r.author,
		LastBuildDate:  r.now().UTC().Format(time.RFC1123Z),
		Items:          make([]rssItem, 0, len(items)),
	}
	for _, post := range items {
		link := r.urlFor("posts", post.Slug)
		document.Channel.Items = append(document.Channel.Items, rssItem{
			Title:       post.Title,
			Link:        link,
			GUID:        link,
			Description: post.Summary,
			PubDate:     post.PublishedAt.UTC().Format(time.RFC1123Z),
		})
	}
	return marshalXML(document)
}

type rssDocument struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title          string    `xml:"title"`
	Link           string    `xml:"link"`
	Description    string    `xml:"description"`
	ManagingEditor string    `xml:"managingEditor,omitempty"`
	LastBuildDate  string    `xml:"lastBuildDate"`
	Items          []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func sortedPosts(source []posts.PublishedPost) []posts.PublishedPost {
	result := append([]posts.PublishedPost(nil), source...)
	sort.Slice(result, func(left, right int) bool {
		if result[left].PublishedAt.Equal(result[right].PublishedAt) {
			return result[left].Slug < result[right].Slug
		}
		return result[left].PublishedAt.After(result[right].PublishedAt)
	})
	return result
}

func parsePublicBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid public base URL %q", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid public base URL scheme %q", parsed.Scheme)
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func (r *RSS) urlFor(parts ...string) string {
	return absoluteURL(r.baseURL, parts...)
}

func absoluteURL(baseURL *url.URL, parts ...string) string {
	copy := *baseURL
	segments := make([]string, 0, len(parts)+1)
	if path := strings.Trim(copy.Path, "/"); path != "" {
		segments = append(segments, path)
	}
	segments = append(segments, parts...)
	copy.Path = "/" + strings.Join(segments, "/")
	if len(parts) == 0 && !strings.HasSuffix(copy.Path, "/") {
		copy.Path += "/"
	}
	copy.RawPath = ""
	return copy.String()
}

func marshalXML(value any) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString(xml.Header)
	encoder := xml.NewEncoder(&output)
	encoder.Indent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
