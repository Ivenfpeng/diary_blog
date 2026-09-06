package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BLOG_ADDR", "")
	t.Setenv("BLOG_DATA_DIR", "")
	t.Setenv("BLOG_PUBLIC_URL", "")
	t.Setenv("BLOG_COOKIE_NAME", "")
	t.Setenv("BLOG_TRUSTED_PROXY_CIDRS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "./data")
	}
	if cfg.PublicURL != "http://localhost:8080" {
		t.Fatalf("PublicURL = %q", cfg.PublicURL)
	}
	if cfg.CookieName != "diary_blog_session" {
		t.Fatalf("CookieName = %q", cfg.CookieName)
	}
	if len(cfg.TrustedProxyCIDRs) != 0 {
		t.Fatalf("TrustedProxyCIDRs = %v, want none", cfg.TrustedProxyCIDRs)
	}
}

func TestLoadUsesEnvironment(t *testing.T) {
	t.Setenv("BLOG_ADDR", "127.0.0.1:9090")
	t.Setenv("BLOG_DATA_DIR", "/srv/diary-blog")
	t.Setenv("BLOG_PUBLIC_URL", "https://blog.example.com")
	t.Setenv("BLOG_COOKIE_NAME", "blog_session")
	t.Setenv("BLOG_TRUSTED_PROXY_CIDRS", "172.30.0.10/32, 10.0.0.0/8")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != "127.0.0.1:9090" || cfg.DataDir != "/srv/diary-blog" {
		t.Fatalf("unexpected server config: %+v", cfg)
	}
	if cfg.PublicURL != "https://blog.example.com" || cfg.CookieName != "blog_session" {
		t.Fatalf("unexpected public config: %+v", cfg)
	}
	if len(cfg.TrustedProxyCIDRs) != 2 || cfg.TrustedProxyCIDRs[0] != "172.30.0.10/32" || cfg.TrustedProxyCIDRs[1] != "10.0.0.0/8" {
		t.Fatalf("TrustedProxyCIDRs = %#v", cfg.TrustedProxyCIDRs)
	}
}
