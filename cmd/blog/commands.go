package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/Ivenfpeng/diary_blog/internal/config"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/operations"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func runCommand(ctx context.Context, cfg config.Config, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 || args[0] == "serve" {
		return errors.New("serve must be started by main")
	}
	switch args[0] {
	case "migrate":
		if len(args) != 1 {
			return errors.New("usage: blog migrate")
		}
		return migrateCommand(ctx, cfg, output)
	case "backup":
		flags := flag.NewFlagSet("backup", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("output", "", "")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("usage: blog backup --output PATH: %w", err)
		}
		if flags.NArg() != 0 || strings.TrimSpace(*path) == "" {
			return errors.New("usage: blog backup --output PATH")
		}
		if err := operations.Backup(ctx, cfg.DataDir, *path); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "backup created: %s\n", *path)
		return err
	case "restore":
		flags := flag.NewFlagSet("restore", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		path := flags.String("input", "", "")
		force := flags.Bool("force", false, "")
		if err := flags.Parse(args[1:]); err != nil {
			return fmt.Errorf("usage: blog restore --input PATH [--force]: %w", err)
		}
		if flags.NArg() != 0 || strings.TrimSpace(*path) == "" {
			return errors.New("usage: blog restore --input PATH [--force]")
		}
		if err := operations.Restore(ctx, *path, cfg.DataDir, *force); err != nil {
			return err
		}
		_, err := fmt.Fprintf(output, "backup restored into: %s\n", cfg.DataDir)
		return err
	case "admin":
		return adminCommand(ctx, cfg, args[1:], input, output)
	case "search":
		if len(args) != 2 || args[1] != "rebuild" {
			return errors.New("usage: blog search rebuild")
		}
		db, err := openMigrated(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := sqliterepo.NewPostRepository(db).RebuildSearch(ctx); err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, "search index rebuilt")
		return err
	default:
		return fmt.Errorf("unknown command %q (use serve, migrate, backup, restore, admin, or search)", args[0])
	}
}

func migrateCommand(ctx context.Context, cfg config.Config, output io.Writer) error {
	db, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = fmt.Fprintln(output, "migrations complete")
	return err
}

func adminCommand(ctx context.Context, cfg config.Config, args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 || args[0] != "reset-password" {
		return errors.New("usage: blog admin reset-password --username NAME")
	}
	flags := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	username := flags.String("username", "", "")
	if err := flags.Parse(args[1:]); err != nil {
		return fmt.Errorf("usage: blog admin reset-password --username NAME: %w", err)
	}
	if flags.NArg() != 0 || strings.TrimSpace(*username) == "" {
		return errors.New("usage: blog admin reset-password --username NAME")
	}
	password, err := readPassword(input)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash administrator password: %w", err)
	}
	db, err := openMigrated(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := sqliterepo.NewPostRepository(db).UpsertAdmin(ctx, *username, hash, time.Now().UTC()); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "administrator password reset for %s\n", auth.NormalizeUsername(*username))
	return err
}

func readPassword(input io.Reader) (string, error) {
	contents, err := io.ReadAll(io.LimitReader(input, 4097))
	if err != nil {
		return "", fmt.Errorf("read administrator password: %w", err)
	}
	password := strings.TrimRight(string(contents), "\r\n")
	if password == "" {
		return "", errors.New("administrator password is required on standard input")
	}
	if len(password) > 4096 {
		return "", errors.New("administrator password is too long")
	}
	return password, nil
}

func openMigrated(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := appdb.Open(ctx, cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := appdb.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
