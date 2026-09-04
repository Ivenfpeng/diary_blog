package sqlite_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
)

func TestSessionRepositoryStoresHashesExpiresAbsolutelyAndDeletesOnLogout(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	admin, err := repo.UpsertAdmin(ctx, "Admin", "encoded-password", now)
	if err != nil {
		t.Fatal(err)
	}
	loadedAdmin, err := repo.FindAdminByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if loadedAdmin.ID != admin.ID || loadedAdmin.Username != "admin" || loadedAdmin.PasswordHash != "encoded-password" {
		t.Fatalf("loaded administrator = %+v, want normalized persisted administrator", loadedAdmin)
	}

	session, credentials, err := auth.NewSession(admin.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	var storedToken, storedCSRF []byte
	if err := db.QueryRowContext(ctx, "SELECT token_hash, csrf_hash FROM sessions").Scan(&storedToken, &storedCSRF); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedToken, session.TokenHash[:]) || !bytes.Equal(storedCSRF, session.CSRFHash[:]) {
		t.Fatalf("stored hashes token=%x csrf=%x", storedToken, storedCSRF)
	}
	if bytes.Contains(storedToken, []byte(credentials.SessionToken)) || bytes.Contains(storedCSRF, []byte(credentials.CSRFToken)) {
		t.Fatal("repository stored a raw credential")
	}

	loaded, err := repo.FindSession(ctx, session.TokenHash, now.Add(11*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TokenHash != session.TokenHash || loaded.CSRFHash != session.CSRFHash || !loaded.ExpiresAt.Equal(session.ExpiresAt) {
		t.Fatalf("loaded session = %+v, want %+v", loaded, session)
	}
	if _, err := repo.FindSession(ctx, session.TokenHash, session.ExpiresAt); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("expired session error = %v, want ErrSessionNotFound", err)
	}
	if err := repo.DeleteSession(ctx, session.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindSession(ctx, session.TokenHash, now); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("deleted session error = %v, want ErrSessionNotFound", err)
	}
}

func newAuthRepository(t *testing.T) (*sqliterepo.PostRepository, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db, err := appdb.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return sqliterepo.NewAuthRepository(db), db
}
