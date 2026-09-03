package config

import "os"

type Config struct {
	Addr       string
	DataDir    string
	PublicURL  string
	CookieName string
}

func Load() (Config, error) {
	return Config{
		Addr:       value("BLOG_ADDR", ":8080"),
		DataDir:    value("BLOG_DATA_DIR", "./data"),
		PublicURL:  value("BLOG_PUBLIC_URL", "http://localhost:8080"),
		CookieName: value("BLOG_COOKIE_NAME", "diary_blog_session"),
	}, nil
}

func value(key, fallback string) string {
	if configured := os.Getenv(key); configured != "" {
		return configured
	}
	return fallback
}
