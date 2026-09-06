package config

import (
	"os"
	"strings"
)

type Config struct {
	Addr              string
	DataDir           string
	PublicURL         string
	CookieName        string
	TrustedProxyCIDRs []string
}

func Load() (Config, error) {
	return Config{
		Addr:              value("BLOG_ADDR", ":8080"),
		DataDir:           value("BLOG_DATA_DIR", "./data"),
		PublicURL:         value("BLOG_PUBLIC_URL", "http://localhost:8080"),
		CookieName:        value("BLOG_COOKIE_NAME", "diary_blog_session"),
		TrustedProxyCIDRs: list("BLOG_TRUSTED_PROXY_CIDRS"),
	}, nil
}

func value(key, fallback string) string {
	if configured := os.Getenv(key); configured != "" {
		return configured
	}
	return fallback
}

func list(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
