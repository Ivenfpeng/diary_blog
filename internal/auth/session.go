package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const SessionLifetime = 12 * time.Hour

var (
	ErrAdminNotFound      = errors.New("administrator not found")
	ErrAdminAlreadyExists = errors.New("an administrator already exists")
	ErrSessionNotFound    = errors.New("session not found")
)

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	TokenHash  [sha256.Size]byte
	CSRFHash   [sha256.Size]byte
	AdminID    int64
	Username   string
	ExpiresAt  time.Time
	LastSeenAt time.Time
	CreatedAt  time.Time
}

type Credentials struct {
	SessionToken string
	CSRFToken    string
}

type Repository interface {
	FindAdminByUsername(context.Context, string) (Admin, error)
	CreateSession(context.Context, Session) error
	FindSession(context.Context, [sha256.Size]byte, time.Time) (Session, error)
	DeleteSession(context.Context, [sha256.Size]byte) error
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func NewSession(adminID int64, now time.Time) (Session, Credentials, error) {
	sessionToken, err := randomToken()
	if err != nil {
		return Session{}, Credentials{}, err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return Session{}, Credentials{}, err
	}
	credentials := Credentials{SessionToken: sessionToken, CSRFToken: csrfToken}
	return Session{
		TokenHash: HashToken(sessionToken), CSRFHash: HashToken(csrfToken), AdminID: adminID,
		ExpiresAt: now.Add(SessionLifetime), LastSeenAt: now, CreatedAt: now,
	}, credentials, nil
}

func HashToken(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func (s Session) Active(now time.Time) bool {
	return now.Before(s.ExpiresAt)
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate authentication token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
