package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashVerifiesCorrectAndIncorrectPasswords(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Fatalf("password hash did not encode the required parameters: %q", hash)
	}

	matched, err := VerifyPassword("correct horse battery staple", hash)
	if err != nil || !matched {
		t.Fatalf("correct password matched=%v err=%v", matched, err)
	}
	matched, err = VerifyPassword("incorrect", hash)
	if err != nil {
		t.Fatal(err)
	}
	if matched {
		t.Fatal("incorrect password matched")
	}
}

func TestPasswordRejectsMalformedAndExcessiveHashes(t *testing.T) {
	validSalt := base64.RawStdEncoding.EncodeToString(make([]byte, 16))
	validKey := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	tests := []string{
		"not-an-argon-hash",
		"$argon2i$v=19$m=65536,t=3,p=2$" + validSalt + "$" + validKey,
		"$argon2id$v=20$m=65536,t=3,p=2$" + validSalt + "$" + validKey,
		"$argon2id$v=19$m=65537,t=3,p=2$" + validSalt + "$" + validKey,
		"$argon2id$v=19$m=65536,t=4,p=2$" + validSalt + "$" + validKey,
		"$argon2id$v=19$m=65536,t=3,p=3$" + validSalt + "$" + validKey,
		"$argon2id$v=19$m=65536,t=3,p=2$%%%$" + validKey,
	}
	for _, encoded := range tests {
		if matched, err := VerifyPassword("password", encoded); err == nil || matched {
			t.Fatalf("VerifyPassword(%q) matched=%v err=%v, want a parse error", encoded, matched, err)
		}
	}
}

func TestSessionGeneratesIndependentTokensHashesAndAbsoluteExpiry(t *testing.T) {
	now := time.Date(2026, 9, 4, 9, 30, 0, 0, time.UTC)
	session, credentials, err := NewSession(42, now)
	if err != nil {
		t.Fatal(err)
	}
	if session.AdminID != 42 || !session.CreatedAt.Equal(now) || !session.LastSeenAt.Equal(now) {
		t.Fatalf("unexpected session metadata: %+v", session)
	}
	if !session.ExpiresAt.Equal(now.Add(12 * time.Hour)) {
		t.Fatalf("expires at = %v, want %v", session.ExpiresAt, now.Add(12*time.Hour))
	}
	if credentials.SessionToken == credentials.CSRFToken {
		t.Fatal("session and CSRF tokens must be independently generated")
	}
	for name, token := range map[string]string{"session": credentials.SessionToken, "csrf": credentials.CSRFToken} {
		decoded, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatalf("decode %s token: %v", name, err)
		}
		if len(decoded) != 32 {
			t.Fatalf("%s token contains %d random bytes, want 32", name, len(decoded))
		}
	}
	wantSessionHash := sha256.Sum256([]byte(credentials.SessionToken))
	wantCSRFHash := sha256.Sum256([]byte(credentials.CSRFToken))
	if session.TokenHash != wantSessionHash || session.CSRFHash != wantCSRFHash {
		t.Fatalf("session did not retain only the expected token hashes: %+v", session)
	}
	if session.Active(now.Add(12 * time.Hour)) {
		t.Fatal("session remained active at its absolute expiry")
	}
	if !session.Active(now.Add(12*time.Hour - time.Nanosecond)) {
		t.Fatal("session expired before its absolute expiry")
	}
}
