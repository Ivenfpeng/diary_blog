package operations_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
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

func TestBackupRejectsOutputInsideMediaDirectory(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "media"), 0o750); err != nil {
		t.Fatal(err)
	}
	err := operations.Backup(context.Background(), source, filepath.Join(source, "media", "site.tar.gz"))
	if err == nil {
		t.Fatal("backup accepted output inside media directory")
	}
}

func TestBackupRejectsSymlinkedOutputInsideMediaDirectory(t *testing.T) {
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
	mediaRoot := filepath.Join(source, "media")
	if err := os.MkdirAll(mediaRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "media-link")
	if err := os.Symlink(mediaRoot, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	output := filepath.Join(link, "site.tar.gz")
	if err := operations.Backup(context.Background(), source, output); err == nil {
		t.Fatal("backup accepted symlinked output inside media directory")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("backup archive exists through media symlink: %v", err)
	}
}

func TestBackupRejectsMissingSourceDatabase(t *testing.T) {
	output := filepath.Join(t.TempDir(), "site.tar.gz")
	if err := operations.Backup(context.Background(), filepath.Join(t.TempDir(), "missing"), output); err == nil {
		t.Fatal("backup accepted a missing source database")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("backup archive exists after missing source error: %v", err)
	}
}

func TestBackupRejectsInvalidSourceDatabase(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "blog.db"), []byte("not sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "site.tar.gz")
	if err := operations.Backup(context.Background(), source, output); err == nil {
		t.Fatal("backup accepted an invalid source database")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("backup archive exists after invalid source error: %v", err)
	}
}

func TestBackupRejectsOversizedMediaFile(t *testing.T) {
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
	largeMedia := filepath.Join(source, "media", "large.bin")
	if err := os.MkdirAll(filepath.Dir(largeMedia), 0o750); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(largeMedia)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(file, zeroReader{}, 65<<20); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "site.tar.gz")
	if err := operations.Backup(context.Background(), source, output); err == nil {
		t.Fatal("backup accepted media larger than restore can accept")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("backup archive exists after oversized media error: %v", err)
	}
}

func TestBackupAndRestoreSupportURIMetacharactersInPaths(t *testing.T) {
	source := filepath.Join(t.TempDir(), "site#hash")
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
	archive := filepath.Join(t.TempDir(), "site.tar.gz")
	if err := operations.Backup(context.Background(), source, archive); err != nil {
		t.Fatalf("backup with URI metacharacter path: %v", err)
	}
	destination := filepath.Join(t.TempDir(), "restored#hash")
	if err := operations.Restore(context.Background(), archive, destination, false); err != nil {
		t.Fatalf("restore with URI metacharacter path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "blog.db")); err != nil {
		t.Fatalf("restored database missing: %v", err)
	}
}

func TestRestoreRejectsArbitraryTopLevelEntry(t *testing.T) {
	archive := writeTestArchive(t, testArchiveEntry{name: "unexpected.txt", body: []byte("no")})
	err := operations.Restore(context.Background(), archive, filepath.Join(t.TempDir(), "restored"), false)
	if err == nil {
		t.Fatal("restore accepted arbitrary top-level archive entry")
	}
}

func TestRestoreRejectsInvalidDatabase(t *testing.T) {
	archive := writeTestArchive(t,
		testArchiveEntry{name: "blog.db", body: []byte("not sqlite")},
		testArchiveEntry{name: "media/", directory: true},
	)
	err := operations.Restore(context.Background(), archive, filepath.Join(t.TempDir(), "restored"), false)
	if err == nil {
		t.Fatal("restore accepted invalid database")
	}
}

func TestRestoreRejectsOversizedEntry(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "oversized.tar.gz")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "blog.db", Mode: 0o600, Size: 65 << 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(tw, zeroReader{}, 65<<20); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err = operations.Restore(context.Background(), archive, filepath.Join(t.TempDir(), "restored"), false)
	if err == nil {
		t.Fatal("restore accepted oversized archive entry")
	}
}

type testArchiveEntry struct {
	name      string
	body      []byte
	directory bool
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func writeTestArchive(t *testing.T, entries ...testArchiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	checksums := map[string]string{}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o600}
		if entry.directory {
			header.Typeflag = tar.TypeDir
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = int64(len(entry.body))
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(entry.body) != 0 {
			if _, err := tw.Write(entry.body); err != nil {
				t.Fatal(err)
			}
		}
		checksums[entry.name] = checksum(entry.body)
	}
	manifest, err := json.Marshal(struct {
		Version   int               `json:"version"`
		Checksums map[string]string `json:"checksums"`
	}{Version: 1, Checksums: checksums})
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifest)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(manifest); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
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
