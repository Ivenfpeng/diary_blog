package web

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/go-chi/chi/v5"
)

const (
	SessionCookieName = "diary_session"
	CSRFCookieName    = "diary_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
	loginWindow       = 15 * time.Minute
	loginFailureLimit = 5
	loginBodyLimit    = 16 * 1024
)

type authHandler struct {
	repository auth.Repository
	origin     *url.URL
	clock      func() time.Time
	dummyHash  string
	throttle   *loginThrottle
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginThrottle struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	inFlight map[string]int
}

type throttleOutcome uint8

const (
	throttleNeutral throttleOutcome = iota
	throttleFailure
	throttleSuccess
)

func newAuthHandler(repository auth.Repository, origin *url.URL, clock func() time.Time) (*authHandler, error) {
	dummyHash, err := auth.HashPassword("invalid-administrator-password")
	if err != nil {
		return nil, err
	}
	return &authHandler{
		repository: repository, origin: origin, clock: clock, dummyHash: dummyHash,
		throttle: &loginThrottle{failures: make(map[string][]time.Time), inFlight: make(map[string]int)},
	}, nil
}

func (h *authHandler) routes(router chi.Router) {
	router.Post("/api/auth/login", h.login)
	router.With(h.requireSession).Get("/api/auth/session", h.currentSession)
	router.With(h.requireSession, h.requireCSRF).Delete("/api/auth/session", h.logout)
	router.With(h.requireSession, h.requireCSRF).Handle("/api/admin/*", http.NotFoundHandler())
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var input loginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, loginBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_request", "A username and password are required.")
		return
	}
	username := auth.NormalizeUsername(input.Username)
	if username == "" || input.Password == "" {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_request", "A username and password are required.")
		return
	}
	key := username + "\x00" + clientIP(r.RemoteAddr)
	now := h.clock()
	if !h.throttle.begin(key, now) {
		writeAPIError(w, r, http.StatusTooManyRequests, "login_throttled", "Too many failed login attempts. Try again later.")
		return
	}
	outcome := throttleNeutral
	defer func() { h.throttle.complete(key, h.clock(), outcome) }()

	admin, err := h.repository.FindAdminByUsername(r.Context(), username)
	if errors.Is(err, auth.ErrAdminNotFound) {
		_, _ = auth.VerifyPassword(input.Password, h.dummyHash)
		outcome = throttleFailure
		writeAPIError(w, r, http.StatusUnauthorized, "invalid_credentials", "The username or password is incorrect.")
		return
	}
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	matched, err := auth.VerifyPassword(input.Password, admin.PasswordHash)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	if !matched {
		outcome = throttleFailure
		writeAPIError(w, r, http.StatusUnauthorized, "invalid_credentials", "The username or password is incorrect.")
		return
	}
	session, credentials, err := auth.NewSession(admin.ID, now)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	if err := h.repository.CreateSession(r.Context(), session); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	outcome = throttleSuccess
	setAuthCookies(w, credentials, session.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": admin.Username})
}

func (h *authHandler) currentSession(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(authSessionKey).(auth.Session)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true, "username": session.Username, "expires_at": session.ExpiresAt,
	})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(authSessionKey).(auth.Session)
	if err := h.repository.DeleteSession(r.Context(), session.TokenHash); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
		return
	}
	clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func setAuthCookies(w http.ResponseWriter, credentials auth.Credentials, expiresAt time.Time) {
	maxAge := int(auth.SessionLifetime.Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Value: credentials.SessionToken, Path: "/", Expires: expiresAt,
		MaxAge: maxAge, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookieName, Value: credentials.CSRFToken, Path: "/", Expires: expiresAt,
		MaxAge: maxAge, Secure: true, HttpOnly: false, SameSite: http.SameSiteStrictMode,
	})
}

func clearAuthCookies(w http.ResponseWriter) {
	expired := time.Unix(1, 0).UTC()
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Path: "/", Expires: expired, MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: CSRFCookieName, Path: "/", Expires: expired, MaxAge: -1, Secure: true, HttpOnly: false, SameSite: http.SameSiteStrictMode})
}

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "request_id": requestID}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func (l *loginThrottle) begin(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-loginWindow)
	failures := l.failures[key]
	firstCurrent := 0
	for firstCurrent < len(failures) && failures[firstCurrent].Before(cutoff) {
		firstCurrent++
	}
	if firstCurrent > 0 {
		failures = append([]time.Time(nil), failures[firstCurrent:]...)
		l.failures[key] = failures
	}
	if len(failures)+l.inFlight[key] >= loginFailureLimit {
		return false
	}
	l.inFlight[key]++
	return true
}

func (l *loginThrottle) complete(key string, now time.Time, outcome throttleOutcome) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight[key] > 1 {
		l.inFlight[key]--
	} else {
		delete(l.inFlight, key)
	}
	switch outcome {
	case throttleFailure:
		l.failures[key] = append(l.failures[key], now)
	case throttleSuccess:
		delete(l.failures, key)
	}
}
