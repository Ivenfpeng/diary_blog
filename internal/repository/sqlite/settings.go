package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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
	if len(value.Navigation) == 0 {
		value.Navigation = json.RawMessage("[]")
	}
	if len(value.SocialLinks) == 0 {
		value.SocialLinks = json.RawMessage("{}")
	}
	if len(value.SEODefaults) == 0 {
		value.SEODefaults = json.RawMessage("{}")
	}
	for _, raw := range []json.RawMessage{value.Navigation, value.SocialLinks, value.SEODefaults} {
		if !json.Valid(raw) {
			return Settings{}, ErrValidation
		}
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO site_settings (id, site_title, description, author, navigation_json, social_links_json, seo_defaults_json, updated_at) VALUES (1, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET site_title=excluded.site_title, description=excluded.description, author=excluded.author, navigation_json=excluded.navigation_json, social_links_json=excluded.social_links_json, seo_defaults_json=excluded.seo_defaults_json, updated_at=excluded.updated_at", value.SiteTitle, value.Description, value.Author, string(value.Navigation), string(value.SocialLinks), string(value.SEODefaults), formatTime(now))
	if err != nil {
		return Settings{}, fmt.Errorf("save settings: %w", err)
	}
	value.UpdatedAt = now
	return value, nil
}
