package operations_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	"github.com/Ivenfpeng/diary_blog/internal/operations"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func TestBackupAndRestorePreservesSiteData(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	db, err := appdb.Open(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := sqliterepo.NewPostRepository(db)
	now := time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)
	post, err := repo.CreateDraft(ctx, content.PostInput{Slug: "backup-post", Title: "Backup search", ContentMD: "reliable restore"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(ctx, post.ID, content.RenderedContent{HTML: "<p>reliable restore</p>", PlainText: "reliable restore"}, post.Revision, now); err != nil {
		t.Fatal(err)
	}
	mediaPath := filepath.Join(source, "media", "2026", "09", "picture.png")
	if err := os.MkdirAll(filepath.Dir(mediaPath), 0o750); err != nil {
		t.Fatal(err)
	}
	mediaBytes := []byte("media contents")
	if err := os.WriteFile(mediaPath, mediaBytes, 0o640); err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	archive := filepath.Join(t.TempDir(), "site.tar.gz")
	if err := operations.Backup(ctx, source, archive); err != nil {
		t.Fatalf("backup: %v", err)
	}
	checksums := archiveChecksums(t, archive)
	if _, ok := checksums["blog.db"]; !ok {
		t.Fatal("manifest omitted blog.db checksum")
	}
	if got := checksums["media/2026/09/picture.png"]; got != checksum(mediaBytes) {
		t.Fatalf("media checksum = %q", got)
	}

	destination := filepath.Join(t.TempDir(), "restored")
	if err := operations.Restore(ctx, archive, destination, false); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, err := appdb.Open(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	restoredRepo := sqliterepo.NewPostRepository(restored)
	result, total, err := restoredRepo.SearchPublished(ctx, "backup", 1, 10)
	if err != nil || total != 1 || len(result) != 1 || result[0].Slug != "backup-post" {
		t.Fatalf("restored search = %+v total=%d err=%v", result, total, err)
	}
	gotMedia, err := os.ReadFile(filepath.Join(destination, "media", "2026", "09", "picture.png"))
	if err != nil || string(gotMedia) != string(mediaBytes) {
		t.Fatalf("restored media = %q err=%v", gotMedia, err)
	}
}

func TestRestoreRefusesNonEmptyDestinationWithoutForce(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "site.tar.gz")
	source := t.TempDir()
	db, err := appdb.Open(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if err := appdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := operations.Backup(context.Background(), source, archive); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := os.WriteFile(filepath.Join(destination, "keep"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := operations.Restore(context.Background(), archive, destination, false); err == nil {
		t.Fatal("restore into non-empty destination succeeded without force")
	}
}

func archiveChecksums(t *testing.T, archive string) map[string]string {
	t.Helper()
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		if h.Name != "manifest.json" {
			continue
		}
		var manifest struct {
			Checksums map[string]string `json:"checksums"`
		}
		if err := json.NewDecoder(tr).Decode(&manifest); err != nil {
			t.Fatal(err)
		}
		return manifest.Checksums
	}
	t.Fatal("manifest.json absent")
	return nil
}

func checksum(b []byte) string { value := sha256.Sum256(b); return hex.EncodeToString(value[:]) }
