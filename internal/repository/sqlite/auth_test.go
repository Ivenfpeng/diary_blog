package sqlite_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sync"
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

func TestAdminRepositoryEnforcesSingleAdministratorTransactionally(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	admin, err := repo.UpsertAdmin(ctx, "Admin", "first-hash", now)
	if err != nil {
		t.Fatal(err)
	}
	reset, err := repo.UpsertAdmin(ctx, " admin ", "second-hash", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if reset.ID != admin.ID || reset.PasswordHash != "second-hash" {
		t.Fatalf("password reset administrator = %+v, want id %d with updated hash", reset, admin.ID)
	}
	if _, err := repo.UpsertAdmin(ctx, "other", "other-hash", now); !errors.Is(err, auth.ErrAdminAlreadyExists) {
		t.Fatalf("second identity error = %v, want ErrAdminAlreadyExists", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO admins (username, password_hash, created_at, updated_at)
		VALUES ('direct-second', 'hash', '2026-09-04T10:00:00Z', '2026-09-04T10:00:00Z')`); err == nil {
		t.Fatal("schema permitted a second administrator")
	}
}

func TestAdminRepositoryConcurrentCreationAllowsOneIdentity(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, username := range []string{"first", "second"} {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := repo.UpsertAdmin(ctx, username, "hash", time.Now())
			results <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	var succeeded, rejected int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, auth.ErrAdminAlreadyExists):
			rejected++
		default:
			t.Fatalf("unexpected concurrent creation error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent results succeeded=%d rejected=%d, want 1 and 1", succeeded, rejected)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("administrator count = %d, want 1", count)
	}
}

func TestSessionRepositoryPrunesExpiredRowsDuringCreation(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	admin, err := repo.UpsertAdmin(ctx, "admin", "hash", now)
	if err != nil {
		t.Fatal(err)
	}
	expired, _, err := auth.NewSession(admin.ID, now.Add(-13*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, expired); err != nil {
		t.Fatal(err)
	}
	active, _, err := auth.NewSession(admin.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, active); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("session count after bounded prune = %d, want 1", count)
	}
}

func TestSessionRepositorySurfacesExpiredPruneFailure(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	admin, err := repo.UpsertAdmin(ctx, "admin", "hash", now)
	if err != nil {
		t.Fatal(err)
	}
	active, _, err := auth.NewSession(admin.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, active); err != nil {
		t.Fatal(err)
	}
	expired, _, err := auth.NewSession(admin.ID, now.Add(-13*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER reject_expired_session_cleanup BEFORE DELETE ON sessions
		BEGIN SELECT RAISE(ABORT, 'cleanup rejected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindSession(ctx, active.TokenHash, now); err == nil {
		t.Fatal("expected expired-session cleanup failure to be returned")
	}
}

func TestSessionRepositoryOrdersFractionalExpiryNumerically(t *testing.T) {
	ctx := context.Background()
	repo, db := newAuthRepository(t)
	base := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	admin, err := repo.UpsertAdmin(ctx, "admin", "hash", base.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expired, _, err := auth.NewSession(admin.ID, base.Add(-13*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	expired.ExpiresAt = base.Add(100 * time.Millisecond)
	future, _, err := auth.NewSession(admin.ID, base.Add(-13*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	future.ExpiresAt = base.Add(120 * time.Millisecond)
	if err := repo.CreateSession(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateSession(ctx, future); err != nil {
		t.Fatal(err)
	}

	lookupAt := base.Add(110 * time.Millisecond)
	loaded, err := repo.FindSession(ctx, future.TokenHash, lookupAt)
	if err != nil {
		t.Fatalf("future fractional session lookup: %v", err)
	}
	if !loaded.ExpiresAt.Equal(future.ExpiresAt) {
		t.Fatalf("loaded expiry = %v, want %v", loaded.ExpiresAt, future.ExpiresAt)
	}
	var expiredCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE token_hash = ?", expired.TokenHash[:]).Scan(&expiredCount); err != nil {
		t.Fatal(err)
	}
	if expiredCount != 0 {
		t.Fatal("numerically expired .1 session was not pruned before .12 session lookup")
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
