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
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admins (username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash, updated_at = excluded.updated_at`,
		username, passwordHash, formatTime(now), formatTime(now))
	if err != nil {
		return auth.Admin{}, fmt.Errorf("upsert administrator: %w", err)
	}
	return r.FindAdminByUsername(ctx, username)
}

func (r *PostRepository) FindAdminByUsername(ctx context.Context, username string) (auth.Admin, error) {
	var admin auth.Admin
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at, updated_at
		FROM admins WHERE username = ?`, auth.NormalizeUsername(username)).Scan(
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
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, csrf_hash, admin_id, expires_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, session.TokenHash[:], session.CSRFHash[:], session.AdminID,
		formatTime(session.ExpiresAt), formatTime(session.LastSeenAt), formatTime(session.CreatedAt))
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *PostRepository) FindSession(ctx context.Context, tokenHash [sha256.Size]byte, now time.Time) (auth.Session, error) {
	var session auth.Session
	var storedToken, storedCSRF []byte
	var expiresAt, lastSeenAt, createdAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT s.token_hash, s.csrf_hash, s.admin_id, a.username, s.expires_at, s.last_seen_at, s.created_at
		FROM sessions s JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = ?`, tokenHash[:]).Scan(
		&storedToken, &storedCSRF, &session.AdminID, &session.Username, &expiresAt, &lastSeenAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
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
	session.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return auth.Session{}, err
	}
	session.LastSeenAt, err = parseTime(lastSeenAt)
	if err != nil {
		return auth.Session{}, err
	}
	session.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return auth.Session{}, err
	}
	if !session.Active(now) {
		_, _ = r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash[:])
		return auth.Session{}, auth.ErrSessionNotFound
	}
	if _, err := r.db.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?", formatTime(now), tokenHash[:]); err != nil {
		return auth.Session{}, fmt.Errorf("update session last seen: %w", err)
	}
	session.LastSeenAt = now
	return session, nil
}

func (r *PostRepository) DeleteSession(ctx context.Context, tokenHash [sha256.Size]byte) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash[:]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}
