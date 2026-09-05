// Package operations contains offline administration operations for a blog site.
package operations

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	blogmigrations "github.com/Ivenfpeng/diary_blog/migrations"
	"modernc.org/sqlite"
)

const (
	databaseName      = "blog.db"
	manifestName      = "manifest.json"
	maxArchiveEntries = 10_000
	maxManifestBytes  = 1 << 20   // 1 MiB
	maxEntryBytes     = 64 << 20  // 64 MiB
	maxArchiveBytes   = 512 << 20 // 512 MiB
)

var renamePath = os.Rename

// Manifest describes the payload entries in a backup. The manifest is written
// last, so its own digest cannot be included without a circular checksum.
type Manifest struct {
	Version   int               `json:"version"`
	CreatedAt time.Time         `json:"created_at"`
	Checksums map[string]string `json:"checksums"`
}

// Backup creates a durable tar.gz snapshot of a site's database and media.
func Backup(ctx context.Context, dataDir, output string) (err error) {
	if strings.TrimSpace(output) == "" {
		return errors.New("backup output path is required")
	}
	if pathWithin(filepath.Join(dataDir, "media"), output) {
		return fmt.Errorf("backup output must not be inside media directory: %s", output)
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("backup output already exists: %s", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup output: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	tempDB, err := os.CreateTemp(filepath.Dir(output), ".blog-backup-*.db")
	if err != nil {
		return fmt.Errorf("create temporary database path: %w", err)
	}
	tempDBPath := tempDB.Name()
	if err := tempDB.Close(); err != nil {
		_ = os.Remove(tempDBPath)
		return err
	}
	if err := os.Remove(tempDBPath); err != nil {
		return fmt.Errorf("prepare temporary database path: %w", err)
	}
	defer func() { _ = os.Remove(tempDBPath) }()
	if err := snapshotDatabase(ctx, dataDir, tempDBPath); err != nil {
		return err
	}

	tempArchive, err := os.CreateTemp(filepath.Dir(output), ".blog-backup-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temporary archive: %w", err)
	}
	tempArchivePath := tempArchive.Name()
	defer func() { _ = os.Remove(tempArchivePath) }()
	if err := writeArchive(tempArchive, tempDBPath, filepath.Join(dataDir, "media")); err != nil {
		_ = tempArchive.Close()
		return err
	}
	if err := tempArchive.Sync(); err != nil {
		_ = tempArchive.Close()
		return fmt.Errorf("sync backup archive: %w", err)
	}
	if err := tempArchive.Close(); err != nil {
		return fmt.Errorf("close backup archive: %w", err)
	}
	if err := os.Rename(tempArchivePath, output); err != nil {
		return fmt.Errorf("publish backup archive: %w", err)
	}
	return nil
}

type onlineBackupConn interface {
	NewBackup(string) (*sqlite.Backup, error)
}

// snapshotDatabase uses the driver's SQLite online-backup API to create a
// transactionally consistent temporary database while the source remains open.
func snapshotDatabase(ctx context.Context, dataDir, destination string) error {
	db, err := appdb.Open(ctx, dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire SQLite backup connection: %w", err)
	}
	defer conn.Close()
	return conn.Raw(func(raw any) (result error) {
		source, ok := raw.(onlineBackupConn)
		if !ok {
			return errors.New("SQLite driver connection does not support online backup")
		}
		backup, err := source.NewBackup(destination)
		if err != nil {
			return fmt.Errorf("start SQLite online backup: %w", err)
		}
		defer func() {
			if finishErr := backup.Finish(); finishErr != nil && result == nil {
				result = fmt.Errorf("finish SQLite online backup: %w", finishErr)
			}
		}()
		for more := true; more; {
			more, err = backup.Step(-1)
			if err != nil {
				return fmt.Errorf("copy SQLite online backup: %w", err)
			}
		}
		return nil
	})
}

func writeArchive(file *os.File, databasePath, mediaRoot string) error {
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	checksums := map[string]string{}
	closeWriters := func() error {
		if err := tw.Close(); err != nil {
			_ = gz.Close()
			return err
		}
		return gz.Close()
	}
	if err := addFile(tw, databasePath, databaseName, checksums); err != nil {
		_ = closeWriters()
		return err
	}
	if err := addMedia(tw, mediaRoot, checksums); err != nil {
		_ = closeWriters()
		return err
	}
	manifest, err := json.Marshal(Manifest{Version: 1, CreatedAt: time.Now().UTC(), Checksums: checksums})
	if err != nil {
		_ = closeWriters()
		return fmt.Errorf("encode backup manifest: %w", err)
	}
	if err := writeEntry(tw, manifestName, 0o600, manifest); err != nil {
		_ = closeWriters()
		return err
	}
	return closeWriters()
}

func addMedia(tw *tar.Writer, root string, checksums map[string]string) error {
	if err := writeEntry(tw, "media/", 0o750, nil); err != nil {
		return err
	}
	checksums["media/"] = checksum(nil)
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect media directory: %w", err)
	}
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("media symlink is not supported: %s", path)
		}
		if entry.IsDir() || entry.Type().IsRegular() {
			paths = append(paths, path)
			return nil
		}
		return fmt.Errorf("unsupported media file: %s", path)
	})
	if err != nil {
		return fmt.Errorf("walk media directory: %w", err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := "media/" + filepath.ToSlash(rel)
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := writeEntry(tw, name+"/", info.Mode().Perm(), nil); err != nil {
				return err
			}
			checksums[name+"/"] = checksum(nil)
			continue
		}
		if err := addFile(tw, path, name, checksums); err != nil {
			return err
		}
	}
	return nil
}

func addFile(tw *tar.Writer, path, name string, checksums map[string]string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer file.Close()
	if err := writeFileHeader(tw, name, info.Mode().Perm(), info.Size()); err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tw, hash), file); err != nil {
		return fmt.Errorf("write archive entry %s: %w", name, err)
	}
	checksums[name] = hex.EncodeToString(hash.Sum(nil))
	return nil
}

func writeFileHeader(tw *tar.Writer, name string, mode fs.FileMode, size int64) error {
	h := &tar.Header{Name: name, Mode: int64(mode.Perm()), ModTime: time.Now().UTC(), Format: tar.FormatPAX, Typeflag: tar.TypeReg, Size: size}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("write archive header %s: %w", name, err)
	}
	return nil
}

func writeEntry(tw *tar.Writer, name string, mode fs.FileMode, contents []byte) error {
	h := &tar.Header{Name: name, Mode: int64(mode.Perm()), ModTime: time.Now().UTC(), Format: tar.FormatPAX}
	if strings.HasSuffix(name, "/") {
		h.Typeflag = tar.TypeDir
		h.Size = 0
	} else {
		h.Typeflag = tar.TypeReg
		h.Size = int64(len(contents))
	}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("write archive header %s: %w", name, err)
	}
	if len(contents) > 0 {
		if _, err := tw.Write(contents); err != nil {
			return fmt.Errorf("write archive entry %s: %w", name, err)
		}
	}
	return nil
}

func checksum(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

// Restore validates and atomically installs a backup in dataDir. A non-empty
// destination is refused unless force is true.
func Restore(ctx context.Context, input, dataDir string, force bool) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(input) == "" {
		return errors.New("restore input path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dataDir), 0o750); err != nil {
		return fmt.Errorf("create restore parent: %w", err)
	}
	if !force {
		nonEmpty, err := directoryNonEmpty(dataDir)
		if err != nil {
			return err
		}
		if nonEmpty {
			return fmt.Errorf("restore destination is not empty: %s (use --force to replace it)", dataDir)
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(dataDir), "."+filepath.Base(dataDir)+"-restore-*")
	if err != nil {
		return fmt.Errorf("create restore staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err := extractAndValidate(ctx, input, stage); err != nil {
		return err
	}
	if err := validateStagedDatabase(ctx, filepath.Join(stage, databaseName)); err != nil {
		return err
	}
	if err := installStagedSite(stage, dataDir); err != nil {
		return err
	}
	return nil
}

func installStagedSite(stage, destination string) error {
	parent := filepath.Dir(destination)
	old := filepath.Join(parent, "."+filepath.Base(destination)+"-restore-old-"+fmt.Sprint(time.Now().UnixNano()))
	_, err := os.Lstat(destination)
	destinationExists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect restore destination: %w", err)
	}
	if destinationExists {
		if err := renamePath(destination, old); err != nil {
			return fmt.Errorf("move existing restore destination aside: %w", err)
		}
		if err := syncDirectory(parent); err != nil {
			if rollbackErr := renamePath(old, destination); rollbackErr != nil {
				return fmt.Errorf("%w; restore previous site after sync failure: %v", err, rollbackErr)
			}
			return err
		}
	}
	if err := renamePath(stage, destination); err != nil {
		installErr := fmt.Errorf("install restored site: %w", err)
		if destinationExists {
			if rollbackErr := renamePath(old, destination); rollbackErr != nil {
				return fmt.Errorf("%w; restore previous site: %v", installErr, rollbackErr)
			}
			if syncErr := syncDirectory(parent); syncErr != nil {
				return fmt.Errorf("%w; sync restored previous site: %v", installErr, syncErr)
			}
		}
		return installErr
	}
	if err := syncDirectory(parent); err != nil {
		return err
	}
	if destinationExists {
		if err := os.RemoveAll(old); err != nil {
			return fmt.Errorf("remove previous restored site: %w", err)
		}
		if err := syncDirectory(parent); err != nil {
			return err
		}
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open restore parent for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOTSUP) {
		return fmt.Errorf("sync restore parent: %w", err)
	}
	return nil
}

func directoryNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect restore destination: %w", err)
	}
	return len(entries) > 0, nil
}

func pathWithin(root, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateStagedDatabase(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open restored database: %w", err)
	}
	defer db.Close()
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return fmt.Errorf("check restored database: %w", err)
	}
	if check != "ok" {
		return fmt.Errorf("restored database quick_check failed: %s", check)
	}
	versions, err := fs.Glob(blogmigrations.Files, "*.sql")
	if err != nil {
		return fmt.Errorf("list expected migrations: %w", err)
	}
	sort.Strings(versions)
	for _, version := range versions {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
			return fmt.Errorf("check restored migration %s: %w", version, err)
		}
		if count != 1 {
			return fmt.Errorf("restored database is missing migration %s", version)
		}
	}
	return nil
}

func extractAndValidate(ctx context.Context, input, stage string) error {
	f, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read backup gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	actual := map[string]string{}
	var manifest Manifest
	seenManifest := false
	var entries, totalBytes int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup archive: %w", err)
		}
		if seenManifest {
			return errors.New("backup manifest must be the final archive entry")
		}
		entries++
		if entries > maxArchiveEntries {
			return fmt.Errorf("backup archive has more than %d entries", maxArchiveEntries)
		}
		if !safeArchiveName(h.Name) {
			return fmt.Errorf("unsafe archive path: %s", h.Name)
		}
		if !allowedArchiveName(h.Name) {
			return fmt.Errorf("unsupported archive entry: %s", h.Name)
		}
		if h.Name == manifestName {
			if h.Typeflag != tar.TypeReg {
				return errors.New("backup manifest is not a file")
			}
			if h.Size < 0 || h.Size > maxManifestBytes {
				return fmt.Errorf("backup manifest exceeds %d bytes", maxManifestBytes)
			}
			if err := json.NewDecoder(io.LimitReader(tr, h.Size)).Decode(&manifest); err != nil {
				return fmt.Errorf("decode backup manifest: %w", err)
			}
			seenManifest = true
			continue
		}
		if _, exists := actual[h.Name]; exists {
			return fmt.Errorf("duplicate archive entry: %s", h.Name)
		}
		destination := filepath.Join(stage, filepath.FromSlash(strings.TrimSuffix(h.Name, "/")))
		switch h.Typeflag {
		case tar.TypeDir:
			if h.Size != 0 {
				return fmt.Errorf("directory archive entry has content: %s", h.Name)
			}
			if err := os.MkdirAll(destination, fs.FileMode(h.Mode).Perm()); err != nil {
				return err
			}
			actual[h.Name] = checksum(nil)
		case tar.TypeReg, tar.TypeRegA:
			if h.Size < 0 || h.Size > maxEntryBytes {
				return fmt.Errorf("backup entry %s exceeds %d bytes", h.Name, maxEntryBytes)
			}
			totalBytes += h.Size
			if totalBytes > maxArchiveBytes {
				return fmt.Errorf("backup archive exceeds %d bytes", maxArchiveBytes)
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
				return err
			}
			actual[h.Name], err = extractRegularFile(tr, destination, h)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %s", h.Name)
		}
	}
	if !seenManifest {
		return errors.New("backup manifest is missing")
	}
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported backup manifest version: %d", manifest.Version)
	}
	if manifest.Checksums == nil || len(manifest.Checksums) != len(actual) {
		return errors.New("backup manifest checksums do not match archive entries")
	}
	for name, digest := range actual {
		if manifest.Checksums[name] != digest {
			return fmt.Errorf("backup checksum mismatch for %s", name)
		}
	}
	if _, ok := actual[databaseName]; !ok {
		return errors.New("backup database is missing")
	}
	if _, ok := actual["media/"]; !ok {
		return errors.New("backup media directory is missing")
	}
	if info, err := os.Stat(filepath.Join(stage, databaseName)); err != nil || !info.Mode().IsRegular() {
		return errors.New("backup database is not a regular file")
	}
	if info, err := os.Stat(filepath.Join(stage, "media")); err != nil || !info.IsDir() {
		return errors.New("backup media path is not a directory")
	}
	return nil
}

func extractRegularFile(source io.Reader, destination string, header *tar.Header) (string, error) {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fs.FileMode(header.Mode).Perm())
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(source, header.Size))
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("extract archive entry %s: %w", header.Name, copyErr)
	}
	if n != header.Size {
		return "", fmt.Errorf("truncated archive entry: %s", header.Name)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close archive entry %s: %w", header.Name, closeErr)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func allowedArchiveName(name string) bool {
	return name == databaseName || name == "media/" || strings.HasPrefix(name, "media/") || name == manifestName
}

func safeArchiveName(name string) bool {
	clean := strings.TrimSuffix(name, "/")
	return clean != "" && !strings.HasPrefix(name, "/") && filepath.ToSlash(filepath.Clean(clean)) == clean && !strings.HasPrefix(clean, "../") && clean != ".."
}
