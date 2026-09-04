// Package web exposes the HTTP server for the public diary.
package web

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/posts"
	webassets "github.com/Ivenfpeng/diary_blog/web"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// ServerOptions configures filesystem resources that are intentionally kept
// outside the executable image.
type ServerOptions struct {
	MediaDir string
	Logger   *slog.Logger
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

	public, err := newPublicHandler(repository)
	if err != nil {
		panic(fmt.Sprintf("load public templates: %v", err))
	}
	router := chi.NewRouter()
	router.Use(requestID, recoverPanics(config.Logger), accessLog(config.Logger))
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

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

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func recoverPanics(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic while serving request", "request_id", r.Context().Value(requestIDKey), "panic", recovered, "stack", string(debug.Stack()))
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func accessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("http request", "request_id", r.Context().Value(requestIDKey), "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
		})
	}
}
