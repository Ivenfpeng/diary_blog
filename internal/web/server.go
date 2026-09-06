// Package web exposes the HTTP server for the public diary.
package web

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/site"
	webassets "github.com/Ivenfpeng/diary_blog/web"
	"github.com/go-chi/chi/v5"
)

// ServerOptions configures runtime dependencies, public URL handling, and
// authentication boundaries.
type ServerOptions struct {
	MediaDir       string
	Logger         *slog.Logger
	PublicURL      string
	Clock          func() time.Time
	AuthRepository auth.Repository
	Auth           AuthOptions
	Cache          *site.Cache
	// Ready verifies storage dependencies after startup migrations finish.
	Ready func() error
}

// AuthOptions bounds authentication work and throttle state, and configures trusted network peers.
// Forwarded client addresses are ignored unless their immediate peer matches a
// configured trusted proxy CIDR.
type AuthOptions struct {
	Repository            auth.Repository
	MaxConcurrentAuthWork int
	MaxThrottleEntries    int
	TrustedProxyCIDRs     []string
}

// NewServer builds the public HTTP surface. Options are optional so the
// server remains convenient to embed in tests and command entry points.
func NewServer(repository posts.Repository, options ...ServerOptions) http.Handler {
	config := ServerOptions{}
	if len(options) > 0 {
		config = options[0]
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.PublicURL == "" {
		config.PublicURL = "http://localhost:8080"
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}

	public, err := newPublicHandler(repository, config.PublicURL, config.Clock, config.Cache)
	if err != nil {
		panic(fmt.Sprintf("load public templates: %v", err))
	}
	router := chi.NewRouter()
	router.Use(requestID, accessLog(config.Logger), recoverPanics(config.Logger))
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if config.Ready != nil && config.Ready() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	authRepository := config.Auth.Repository
	if authRepository == nil {
		authRepository = config.AuthRepository
	}
	if authRepository == nil {
		authRepository, _ = repository.(auth.Repository)
	}
	if authRepository != nil {
		authAPI, err := newAuthHandler(authRepository, public.publicBaseURL, config.Clock, config.Auth)
		if err != nil {
			panic(fmt.Sprintf("initialize authentication: %v", err))
		}
		authAPI.routes(router)
		newAdminPostAPI(posts.NewService(repository, nil, config.Clock), config.Cache).routes(router, authAPI.requireSession, authAPI.requireCSRF)
		if managementRepository, ok := repository.(*sqlite.PostRepository); ok {
			newAdminManagementAPI(managementRepository, config.MediaDir, config.Clock, config.Cache).routes(router, authAPI.requireSession, authAPI.requireCSRF)
		}
		authAPI.adminNotFound(router)
	}

	static, err := fs.Sub(webassets.Assets, "static")
	if err != nil {
		panic(fmt.Sprintf("open static assets: %v", err))
	}
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	admin, err := fs.Sub(webassets.Assets, "admin")
	if err != nil {
		panic(fmt.Sprintf("open admin assets: %v", err))
	}
	adminIndex, err := fs.ReadFile(webassets.Assets, "admin/index.html")
	if err != nil {
		panic(fmt.Sprintf("open admin index: %v", err))
	}
	router.Get("/admin", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(adminIndex)
	})
	adminStatic := http.StripPrefix("/admin/", http.FileServer(http.FS(admin)))
	router.Handle("/admin/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(filepath.Base(r.URL.Path), ".") {
			adminStatic.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(adminIndex)
	}))
	if config.MediaDir == "" {
		router.Handle("/media/*", http.NotFoundHandler())
	} else {
		router.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir(filepath.Clean(config.MediaDir)))))
	}
	public.routes(router)
	return router
}
