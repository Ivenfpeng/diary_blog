package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/config"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/platform"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/site"
	"github.com/Ivenfpeng/diary_blog/internal/web"
)

const shutdownTimeout = 10 * time.Second

type shutdowner interface{ Shutdown(context.Context) error }

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logger := platform.NewLogger(os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := appdb.Open(ctx, cfg.DataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := appdb.Migrate(ctx, db); err != nil {
		return err
	}
	repository := sqliterepo.NewPostRepository(db)
	server := &http.Server{Addr: cfg.Addr, Handler: web.NewServer(repository, web.ServerOptions{
		MediaDir: filepath.Join(cfg.DataDir, "media"), Logger: logger, PublicURL: cfg.PublicURL,
		AuthRepository: repository, Ready: func() error { return writable(cfg.DataDir) }, Cache: site.NewCache(time.Minute),
	})}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		return shutdownServer(server)
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func shutdownServer(server shutdowner) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return server.Shutdown(ctx)
}

func writable(directory string) error {
	file, err := os.CreateTemp(directory, ".ready-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}
