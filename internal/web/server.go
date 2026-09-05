// Package web exposes the HTTP server for the public diary.
package web

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
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

	public, err := newPublicHandler(repository, config.PublicURL, config.Clock)
	if err != nil {
		panic(fmt.Sprintf("load public templates: %v", err))
	}
	router := chi.NewRouter()
	router.Use(requestID, recoverPanics(config.Logger), accessLog(config.Logger))
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
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
		newAdminPostAPI(posts.NewService(repository, nil, config.Clock)).routes(router, authAPI.requireSession, authAPI.requireCSRF)
		if managementRepository, ok := repository.(*sqlite.PostRepository); ok {
			newAdminManagementAPI(managementRepository, config.MediaDir, config.Clock).routes(router, authAPI.requireSession, authAPI.requireCSRF)
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
	router.Handle("/admin/*", http.StripPrefix("/admin/", http.FileServer(http.FS(admin))))
	if config.MediaDir == "" {
		router.Handle("/media/*", http.NotFoundHandler())
	} else {
		router.Handle("/media/*", http.StripPrefix("/media/", http.FileServer(http.Dir(filepath.Clean(config.MediaDir)))))
	}
	public.routes(router)
	return router
}
