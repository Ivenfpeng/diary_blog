package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
)

func NewAuthRepository(db *sql.DB) *PostRepository {
	return NewPostRepository(db)
}

func (r *PostRepository) UpsertAdmin(ctx context.Context, username, passwordHash string, now time.Time) (auth.Admin, error) {
	username = auth.NormalizeUsername(username)
	if username == "" || passwordHash == "" {
		return auth.Admin{}, errors.New("administrator username and password hash are required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Admin{}, fmt.Errorf("begin administrator upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingUsername string
	err = tx.QueryRowContext(ctx, "SELECT username FROM admins ORDER BY id LIMIT 1").Scan(&existingUsername)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `
			INSERT INTO admins (username, password_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?)`, username, passwordHash, formatTime(now), formatTime(now))
	case err != nil:
		return auth.Admin{}, fmt.Errorf("inspect administrator identity: %w", err)
	case existingUsername != username:
		return auth.Admin{}, auth.ErrAdminAlreadyExists
	default:
		_, err = tx.ExecContext(ctx, "UPDATE admins SET password_hash = ?, updated_at = ? WHERE username = ?", passwordHash, formatTime(now), username)
	}
	if err != nil {
		return auth.Admin{}, fmt.Errorf("upsert administrator: %w", err)
	}
	admin, err := findAdminByUsername(ctx, tx, username)
	if err != nil {
		return auth.Admin{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.Admin{}, fmt.Errorf("commit administrator upsert: %w", err)
	}
	return admin, nil
}

func (r *PostRepository) FindAdminByUsername(ctx context.Context, username string) (auth.Admin, error) {
	return findAdminByUsername(ctx, r.db, auth.NormalizeUsername(username))
}

func findAdminByUsername(ctx context.Context, q queryer, username string) (auth.Admin, error) {
	var admin auth.Admin
	var createdAt, updatedAt string
	err := q.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at, updated_at
		FROM admins WHERE username = ?`, username).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Admin{}, auth.ErrAdminNotFound
	}
	if err != nil {
		return auth.Admin{}, fmt.Errorf("find administrator: %w", err)
	}
	admin.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return auth.Admin{}, err
	}
	admin.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return auth.Admin{}, err
	}
	return admin, nil
}

func (r *PostRepository) CreateSession(ctx context.Context, session auth.Session) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneExpiredSessions(ctx, tx, session.CreatedAt); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, csrf_hash, admin_id, expires_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, session.TokenHash[:], session.CSRFHash[:], session.AdminID,
		session.ExpiresAt.UnixNano(), session.LastSeenAt.UnixNano(), session.CreatedAt.UnixNano())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create session: %w", err)
	}
	return nil
}

func (r *PostRepository) FindSession(ctx context.Context, tokenHash [sha256.Size]byte, now time.Time) (auth.Session, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Session{}, fmt.Errorf("begin find session: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := pruneExpiredSessions(ctx, tx, now); err != nil {
		return auth.Session{}, err
	}
	var session auth.Session
	var storedToken, storedCSRF []byte
	var expiresAt, lastSeenAt, createdAt int64
	err = tx.QueryRowContext(ctx, `
		SELECT s.token_hash, s.csrf_hash, s.admin_id, a.username, s.expires_at, s.last_seen_at, s.created_at
		FROM sessions s JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = ?`, tokenHash[:]).Scan(
		&storedToken, &storedCSRF, &session.AdminID, &session.Username, &expiresAt, &lastSeenAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return auth.Session{}, fmt.Errorf("commit expired session prune: %w", err)
		}
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if err != nil {
		return auth.Session{}, fmt.Errorf("find session: %w", err)
	}
	if len(storedToken) != sha256.Size || len(storedCSRF) != sha256.Size {
		return auth.Session{}, errors.New("stored session hash has invalid length")
	}
	copy(session.TokenHash[:], storedToken)
	copy(session.CSRFHash[:], storedCSRF)
	session.ExpiresAt = time.Unix(0, expiresAt).UTC()
	session.LastSeenAt = time.Unix(0, lastSeenAt).UTC()
	session.CreatedAt = time.Unix(0, createdAt).UTC()
	if !session.Active(now) {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash[:]); err != nil {
			return auth.Session{}, fmt.Errorf("delete expired session: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return auth.Session{}, fmt.Errorf("commit expired session deletion: %w", err)
		}
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if _, err := tx.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?", now.UnixNano(), tokenHash[:]); err != nil {
		return auth.Session{}, fmt.Errorf("update session last seen: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return auth.Session{}, fmt.Errorf("commit find session: %w", err)
	}
	session.LastSeenAt = now
	return session, nil
}

type contextExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func pruneExpiredSessions(ctx context.Context, execer contextExecer, now time.Time) error {
	if _, err := execer.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", now.UnixNano()); err != nil {
		return fmt.Errorf("prune expired sessions: %w", err)
	}
	return nil
}

func (r *PostRepository) DeleteSession(ctx context.Context, tokenHash [sha256.Size]byte) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
