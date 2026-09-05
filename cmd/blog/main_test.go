package main

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/config"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

type shutdownSpy struct {
	deadline time.Time
}

func (s *shutdownSpy) Shutdown(ctx context.Context) error {
	s.deadline, _ = ctx.Deadline()
	return nil
}

func TestShutdownUsesTenSecondTimeout(t *testing.T) {
	before := time.Now()
	server := &shutdownSpy{}
	if err := shutdownServer(server); err != nil {
		t.Fatalf("shutdown server: %v", err)
	}
	if server.deadline.Before(before.Add(9*time.Second)) || server.deadline.After(before.Add(11*time.Second)) {
		t.Fatalf("shutdown deadline = %v, want approximately ten seconds after %v", server.deadline, before)
	}
}

var _ interface{ Shutdown(context.Context) error } = (*http.Server)(nil)

func TestAdminCommandResetsPasswordWithoutPrintingIt(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Config{DataDir: dataDir}
	var output bytes.Buffer
	if err := runCommand(context.Background(), cfg, []string{"admin", "reset-password", "--username", "admin"}, bytes.NewBufferString("secret-password\n"), &output); err != nil {
		t.Fatalf("run admin command: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte("secret-password")) {
		t.Fatalf("command output leaked password: %q", output.String())
	}
	db, err := appdb.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	admin, err := sqliterepo.NewPostRepository(db).FindAdminByUsername(context.Background(), "admin")
	if err != nil || admin.PasswordHash == "" {
		t.Fatalf("stored administrator = %+v err=%v", admin, err)
	}
}

func TestAdminCommandRequiresUsername(t *testing.T) {
	var output bytes.Buffer
	err := runCommand(context.Background(), config.Config{DataDir: t.TempDir()}, []string{"admin", "reset-password"}, bytes.NewBufferString("secret\n"), &output)
	if err == nil {
		t.Fatal("admin reset-password succeeded without username")
	}
}

func TestRestoreCommandPassesForceFlag(t *testing.T) {
	dataDir := t.TempDir()
	archive := filepath.Join(t.TempDir(), "missing.tar.gz")
	if err := os.WriteFile(filepath.Join(dataDir, "occupied"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runCommand(context.Background(), config.Config{DataDir: dataDir}, []string{"restore", "--input", archive}, bytes.NewReader(nil), &output)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("not empty")) {
		t.Fatalf("restore error = %v, want non-empty destination refusal", err)
	}
}
