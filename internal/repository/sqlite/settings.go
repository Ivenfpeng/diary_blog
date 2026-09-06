package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"
)

type NavigationItem struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type SocialLink struct {
	Label string
	URL   string
}

type SEODefaults struct {
	TitleSuffix    string `json:"title_suffix"`
	Description    string `json:"description"`
	Robots         string `json:"robots"`
	OpenGraphImage string `json:"open_graph_image"`
}

type Settings struct {
	SiteTitle   string          `json:"site_title"`
	Description string          `json:"description"`
	Author      string          `json:"author"`
	Navigation  json.RawMessage `json:"navigation"`
	SocialLinks json.RawMessage `json:"social_links"`
	SEODefaults json.RawMessage `json:"seo_defaults"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func (r *PostRepository) GetSettings(ctx context.Context) (Settings, error) {
	var value Settings
	var navigation, socialLinks, seoDefaults, updated string
	err := r.db.QueryRowContext(ctx, "SELECT site_title, description, author, navigation_json, social_links_json, seo_defaults_json, updated_at FROM site_settings WHERE id = 1").Scan(&value.SiteTitle, &value.Description, &value.Author, &navigation, &socialLinks, &seoDefaults, &updated)
	if err == sql.ErrNoRows {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("get settings: %w", err)
	}
	value.Navigation, value.SocialLinks, value.SEODefaults = json.RawMessage(navigation), json.RawMessage(socialLinks), json.RawMessage(seoDefaults)
	value.UpdatedAt, err = parseTime(updated)
	return value, err
}
func (r *PostRepository) SaveSettings(ctx context.Context, value Settings, now time.Time) (Settings, error) {
	value.SiteTitle = strings.TrimSpace(value.SiteTitle)
	value.Description = strings.TrimSpace(value.Description)
	value.Author = strings.TrimSpace(value.Author)
	if value.SiteTitle == "" || len(value.SiteTitle) > 120 || len(value.Description) > 500 || len(value.Author) > 120 {
		return Settings{}, ErrValidation
	}
	if len(value.Navigation) == 0 || string(value.Navigation) == "null" {
		value.Navigation = json.RawMessage("[]")
	}
	if _, err := ParseNavigation(value.Navigation); err != nil {
		return Settings{}, ErrValidation
	}
	if len(value.SocialLinks) == 0 || string(value.SocialLinks) == "null" {
		value.SocialLinks = json.RawMessage("{}")
	}
	if _, err := ParseSocialLinks(value.SocialLinks); err != nil {
		return Settings{}, ErrValidation
	}
	if len(value.SEODefaults) == 0 || string(value.SEODefaults) == "null" {
		value.SEODefaults = json.RawMessage("{}")
	}
	if _, err := ParseSEODefaults(value.SEODefaults); err != nil {
		return Settings{}, ErrValidation
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO site_settings (id, site_title, description, author, navigation_json, social_links_json, seo_defaults_json, updated_at) VALUES (1, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET site_title=excluded.site_title, description=excluded.description, author=excluded.author, navigation_json=excluded.navigation_json, social_links_json=excluded.social_links_json, seo_defaults_json=excluded.seo_defaults_json, updated_at=excluded.updated_at", value.SiteTitle, value.Description, value.Author, string(value.Navigation), string(value.SocialLinks), string(value.SEODefaults), formatTime(now))
	if err != nil {
		return Settings{}, fmt.Errorf("save settings: %w", err)
	}
	value.UpdatedAt = now
	return value, nil
}

func ParseNavigation(raw json.RawMessage) ([]NavigationItem, error) {
	var items []NavigationItem
	if err := decodeSettingsJSON(raw, &items); err != nil || items == nil || len(items) > 20 {
		return nil, ErrValidation
	}
	for index := range items {
		items[index].Label = strings.TrimSpace(items[index].Label)
		items[index].URL = strings.TrimSpace(items[index].URL)
		if items[index].Label == "" || len(items[index].Label) > 80 || !validPublicLink(items[index].URL, true) {
			return nil, ErrValidation
		}
	}
	return items, nil
}

func ParseSocialLinks(raw json.RawMessage) ([]SocialLink, error) {
	var values map[string]string
	if err := decodeSettingsJSON(raw, &values); err != nil || values == nil || len(values) > 20 {
		return nil, ErrValidation
	}
	labels := make([]string, 0, len(values))
	for label := range values {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	links := make([]SocialLink, 0, len(labels))
	for _, label := range labels {
		trimmedLabel := strings.TrimSpace(label)
		target := strings.TrimSpace(values[label])
		if trimmedLabel == "" || len(trimmedLabel) > 80 || !validPublicLink(target, false) {
			return nil, ErrValidation
		}
		links = append(links, SocialLink{Label: trimmedLabel, URL: target})
	}
	return links, nil
}

func ParseSEODefaults(raw json.RawMessage) (SEODefaults, error) {
	var value SEODefaults
	if err := decodeSettingsJSON(raw, &value); err != nil {
		return SEODefaults{}, ErrValidation
	}
	value.TitleSuffix = strings.TrimSpace(value.TitleSuffix)
	value.Description = strings.TrimSpace(value.Description)
	value.Robots = strings.TrimSpace(value.Robots)
	value.OpenGraphImage = strings.TrimSpace(value.OpenGraphImage)
	if len(value.TitleSuffix) > 120 || len(value.Description) > 500 || len(value.OpenGraphImage) > 2048 {
		return SEODefaults{}, ErrValidation
	}
	if value.Robots != "" && value.Robots != "index,follow" && value.Robots != "noindex,follow" && value.Robots != "noindex,nofollow" {
		return SEODefaults{}, ErrValidation
	}
	if value.OpenGraphImage != "" && !validPublicLink(value.OpenGraphImage, true) {
		return SEODefaults{}, ErrValidation
	}
	return value, nil
}

func decodeSettingsJSON(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("settings JSON contains trailing data")
	}
	return nil
}

func validPublicLink(value string, allowRelative bool) bool {
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	if allowRelative && strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		parsed, err := url.Parse(value)
		return err == nil && parsed.Host == "" && parsed.User == nil
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
