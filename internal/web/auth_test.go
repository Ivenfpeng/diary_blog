package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/web"
)

const authOrigin = "https://diary.example"

func TestLoginSetsSecureCookiesExposesSessionAndLogoutDeletesIt(t *testing.T) {
	server, db, clock := newAuthServer(t)
	now := *clock
	response := login(t, server, " Admin ", "secret")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	cookies := cookieMap(response.Cookies())
	_ = response.Body.Close()
	sessionCookie := cookies[web.SessionCookieName]
	csrfCookie := cookies[web.CSRFCookieName]
	if sessionCookie == nil || !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode || sessionCookie.Value == "" {
		t.Fatalf("invalid session cookie: %#v", sessionCookie)
	}
	if csrfCookie == nil || !csrfCookie.Secure || csrfCookie.HttpOnly || csrfCookie.SameSite != http.SameSiteStrictMode || csrfCookie.Value == "" {
		t.Fatalf("invalid CSRF cookie: %#v", csrfCookie)
	}
	if !sessionCookie.Expires.Equal(now.Add(12*time.Hour)) || !csrfCookie.Expires.Equal(now.Add(12*time.Hour)) {
		t.Fatalf("cookie expiries session=%v csrf=%v, want %v", sessionCookie.Expires, csrfCookie.Expires, now.Add(12*time.Hour))
	}

	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/session", nil)
	request.AddCookie(sessionCookie)
	response = do(t, server, request)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	_ = response.Body.Close()

	request, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/auth/session", nil)
	request.Header.Set("Origin", authOrigin)
	request.Header.Set(web.CSRFHeaderName, csrfCookie.Value)
	request.AddCookie(sessionCookie)
	request.AddCookie(csrfCookie)
	response = do(t, server, request)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204: %s", response.StatusCode, readBody(t, response))
	}
	_ = response.Body.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("session count after logout = %d, want 0", count)
	}
}

func TestLoginThrottlesNormalizedUsernameAndClientIPAfterFiveFailures(t *testing.T) {
	server, _, clock := newAuthServer(t)
	for attempt := 1; attempt <= 5; attempt++ {
		username := "ADMIN"
		if attempt%2 == 0 {
			username = " admin "
		}
		response := login(t, server, username, "wrong")
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401: %s", attempt, response.StatusCode, readBody(t, response))
		}
		_ = response.Body.Close()
	}
	response := login(t, server, "admin", "secret")
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("sixth attempt status = %d, want 429: %s", response.StatusCode, readBody(t, response))
	}
	_ = response.Body.Close()
	*clock = clock.Add(15*time.Minute + time.Nanosecond)
	response = login(t, server, "admin", "secret")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("attempt after throttle window status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	_ = response.Body.Close()
}

func TestCSRFRejectsUnsafeAdminRequestsUnlessOriginDoubleSubmitAndStoredHashMatch(t *testing.T) {
	server, _, _ := newAuthServer(t)
	response := login(t, server, "admin", "secret")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d: %s", response.StatusCode, readBody(t, response))
	}
	cookies := cookieMap(response.Cookies())
	_ = response.Body.Close()

	tests := []struct {
		name       string
		origin     string
		csrfHeader string
		csrfCookie string
		want       int
	}{
		{name: "missing origin", csrfHeader: cookies[web.CSRFCookieName].Value, csrfCookie: cookies[web.CSRFCookieName].Value, want: http.StatusForbidden},
		{name: "foreign origin", origin: "https://attacker.example", csrfHeader: cookies[web.CSRFCookieName].Value, csrfCookie: cookies[web.CSRFCookieName].Value, want: http.StatusForbidden},
		{name: "double submit mismatch", origin: authOrigin, csrfHeader: "different", csrfCookie: cookies[web.CSRFCookieName].Value, want: http.StatusForbidden},
		{name: "stored hash mismatch", origin: authOrigin, csrfHeader: "tampered", csrfCookie: "tampered", want: http.StatusForbidden},
		{name: "valid protection", origin: authOrigin, csrfHeader: cookies[web.CSRFCookieName].Value, csrfCookie: cookies[web.CSRFCookieName].Value, want: http.StatusNotFound},
		{name: "equivalent default port origin", origin: "https://DIARY.EXAMPLE:443", csrfHeader: cookies[web.CSRFCookieName].Value, csrfCookie: cookies[web.CSRFCookieName].Value, want: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/admin/probe", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.csrfHeader != "" {
				request.Header.Set(web.CSRFHeaderName, test.csrfHeader)
			}
			request.AddCookie(cookies[web.SessionCookieName])
			request.AddCookie(&http.Cookie{Name: web.CSRFCookieName, Value: test.csrfCookie})
			response := do(t, server, request)
			if response.StatusCode != test.want {
				t.Fatalf("status = %d, want %d: %s", response.StatusCode, test.want, readBody(t, response))
			}
			_ = response.Body.Close()
		})
	}
}

func newAuthServer(t *testing.T) (*httptest.Server, *sql.DB, *time.Time) {
	t.Helper()
	ctx := context.Background()
	db, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := appdb.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := sqliterepo.NewAuthRepository(db)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertAdmin(ctx, "admin", passwordHash, now); err != nil {
		t.Fatal(err)
	}
	clock := now
	handler := web.NewServer(repo, web.ServerOptions{AuthRepository: repo, PublicURL: authOrigin, Clock: func() time.Time { return clock }})
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server, db, &clock
}

func login(t *testing.T, server *httptest.Server, username, password string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/login", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	return do(t, server, request)
}

func do(t *testing.T, server *httptest.Server, request *http.Request) *http.Response {
	t.Helper()
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func cookieMap(cookies []*http.Cookie) map[string]*http.Cookie {
	result := make(map[string]*http.Cookie, len(cookies))
	for _, cookie := range cookies {
		result[cookie.Name] = cookie
	}
	return result
}
