package web

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/go-chi/chi/v5"
)

const (
	SessionCookieName      = "diary_session"
	CSRFCookieName         = "diary_csrf"
	CSRFHeaderName         = "X-CSRF-Token"
	loginWindow            = 15 * time.Minute
	loginFailureLimit      = 5
	loginBodyLimit         = 16 * 1024
	defaultAuthWork        = 4
	maxUsernameBytes       = 256
	maxUsernameRunes       = 64
	loginIPFailureLimit    = 20
	defaultThrottleEntries = 4096
)

type authHandler struct {
	repository     auth.Repository
	origin         *url.URL
	clock          func() time.Time
	dummyHash      string
	throttle       *loginThrottle
	authWork       chan struct{}
	trustedProxies trustedProxySet
}

type trustedProxySet []*net.IPNet

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginThrottle struct {
	mu         sync.Mutex
	entries    map[string]*loginThrottleEntry
	recency    *list.List
	maxEntries int
}

type loginThrottleEntry struct {
	failures     []time.Time
	inFlight     int
	lastActivity time.Time
	element      *list.Element
}

type throttleOutcome uint8

const (
	throttleNeutral throttleOutcome = iota
	throttleFailure
	throttleSuccess
)

func newAuthHandler(repository auth.Repository, origin *url.URL, clock func() time.Time, options AuthOptions) (*authHandler, error) {
	trustedProxies, err := parseTrustedProxyCIDRs(options.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	maxAuthWork := options.MaxConcurrentAuthWork
	if maxAuthWork < 0 {
		return nil, errors.New("maximum concurrent authentication work cannot be negative")
	}
	if maxAuthWork == 0 {
		maxAuthWork = defaultAuthWork
	}
	maxThrottleEntries := options.MaxThrottleEntries
	if maxThrottleEntries < 0 || maxThrottleEntries == 1 {
		return nil, errors.New("maximum throttle entries must be zero or at least two")
	}
	if maxThrottleEntries == 0 {
		maxThrottleEntries = defaultThrottleEntries
	}
	dummyHash, err := auth.HashPassword("invalid-administrator-password")
	if err != nil {
		return nil, err
	}
	return &authHandler{
		repository: repository, origin: origin, clock: clock, dummyHash: dummyHash,
		throttle: newLoginThrottle(maxThrottleEntries),
		authWork: make(chan struct{}, maxAuthWork), trustedProxies: trustedProxies,
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
	if username == "" || input.Password == "" || len(username) > maxUsernameBytes || utf8.RuneCountInString(username) > maxUsernameRunes {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_request", "A username and password are required.")
		return
	}
	ip := clientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), h.trustedProxies)
	now := h.clock()
	if !h.acquireAuthWork() {
		w.Header().Set("Retry-After", "1")
		writeAPIError(w, r, http.StatusServiceUnavailable, "authentication_busy", "Authentication is temporarily busy. Try again shortly.")
		return
	}
	defer h.releaseAuthWork()
	if !h.throttle.begin(username, ip, now) {
		writeAPIError(w, r, http.StatusTooManyRequests, "login_throttled", "Too many failed login attempts. Try again later.")
		return
	}
	outcome := throttleNeutral
	defer func() { h.throttle.complete(username, ip, h.clock(), outcome) }()

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

func (h *authHandler) acquireAuthWork() bool {
	select {
	case h.authWork <- struct{}{}:
		return true
	default:
		return false
	}
}

func (h *authHandler) releaseAuthWork() {
	<-h.authWork
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

func clientIP(remoteAddr, forwardedFor string, trusted trustedProxySet) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return remoteAddr
	}
	if !trusted.contains(peer) || forwardedFor == "" {
		return peer.String()
	}
	parts := strings.Split(forwardedFor, ",")
	if len(parts) > 32 {
		return peer.String()
	}
	chain := make([]net.IP, 0, len(parts))
	for _, part := range parts {
		address := net.ParseIP(strings.TrimSpace(part))
		if address == nil {
			return peer.String()
		}
		chain = append(chain, address)
	}
	candidate := peer
	for index := len(chain) - 1; index >= 0; index-- {
		candidate = chain[index]
		if !trusted.contains(candidate) {
			return candidate.String()
		}
	}
	return candidate.String()
}

func parseTrustedProxyCIDRs(values []string) (trustedProxySet, error) {
	trusted := make(trustedProxySet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR: %w", err)
		}
		trusted = append(trusted, network)
	}
	return trusted, nil
}

func (trusted trustedProxySet) contains(address net.IP) bool {
	for _, network := range trusted {
		if network.Contains(address) {
			return true
		}
	}
	return false
}

func newLoginThrottle(maxEntries int) *loginThrottle {
	return &loginThrottle{entries: make(map[string]*loginThrottleEntry), recency: list.New(), maxEntries: maxEntries}
}

func usernameIPThrottleKey(username, ip string) string {
	return "username-ip\x00" + username + "\x00" + ip
}

func ipThrottleKey(ip string) string {
	return "ip\x00" + ip
}

func (l *loginThrottle) begin(username, ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.evictExpired(now)
	specs := []struct {
		key   string
		limit int
	}{
		{key: usernameIPThrottleKey(username, ip), limit: loginFailureLimit},
		{key: ipThrottleKey(ip), limit: loginIPFailureLimit},
	}
	for _, spec := range specs {
		entry := l.currentEntry(spec.key, now)
		if entry != nil && len(entry.failures)+entry.inFlight >= spec.limit {
			return false
		}
	}
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, spec.key)
	}
	if !l.makeRoom(keys) {
		return false
	}
	for _, spec := range specs {
		entry := l.entries[spec.key]
		if entry == nil {
			entry = &loginThrottleEntry{lastActivity: now}
			entry.element = l.recency.PushFront(spec.key)
			l.entries[spec.key] = entry
		}
		entry.inFlight++
		entry.lastActivity = now
		l.recency.MoveToFront(entry.element)
	}
	return true
}

func (l *loginThrottle) complete(username, ip string, now time.Time, outcome throttleOutcome) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range []string{usernameIPThrottleKey(username, ip), ipThrottleKey(ip)} {
		entry := l.entries[key]
		if entry == nil {
			continue
		}
		if entry.inFlight > 0 {
			entry.inFlight--
		}
		switch outcome {
		case throttleFailure:
			entry.failures = append(entry.failures, now)
		case throttleSuccess:
			entry.failures = nil
		}
		entry.lastActivity = now
		l.recency.MoveToFront(entry.element)
		if entry.inFlight == 0 && len(entry.failures) == 0 {
			l.remove(key, entry)
		}
	}
}

func (l *loginThrottle) currentEntry(key string, now time.Time) *loginThrottleEntry {
	entry := l.entries[key]
	if entry == nil {
		return nil
	}
	cutoff := now.Add(-loginWindow)
	firstCurrent := 0
	for firstCurrent < len(entry.failures) && entry.failures[firstCurrent].Before(cutoff) {
		firstCurrent++
	}
	if firstCurrent > 0 {
		entry.failures = append([]time.Time(nil), entry.failures[firstCurrent:]...)
	}
	if entry.inFlight == 0 && len(entry.failures) == 0 {
		l.remove(key, entry)
		return nil
	}
	return entry
}

func (l *loginThrottle) evictExpired(now time.Time) {
	cutoff := now.Add(-loginWindow)
	for element := l.recency.Back(); element != nil; {
		previous := element.Prev()
		key := element.Value.(string)
		entry := l.entries[key]
		if !entry.lastActivity.Before(cutoff) {
			break
		}
		if entry.inFlight == 0 {
			l.remove(key, entry)
		}
		element = previous
	}
}

func (l *loginThrottle) makeRoom(requestedKeys []string) bool {
	protected := make(map[string]struct{}, len(requestedKeys))
	for _, key := range requestedKeys {
		protected[key] = struct{}{}
	}
	for {
		missing := 0
		for key := range protected {
			if l.entries[key] == nil {
				missing++
			}
		}
		if len(l.entries)+missing <= l.maxEntries {
			return true
		}
		var candidate *list.Element
		for element := l.recency.Back(); element != nil; element = element.Prev() {
			key := element.Value.(string)
			if _, requested := protected[key]; !requested && l.entries[key].inFlight == 0 {
				candidate = element
				break
			}
		}
		if candidate == nil {
			return false
		}
		key := candidate.Value.(string)
		l.remove(key, l.entries[key])
	}
}

func (l *loginThrottle) remove(key string, entry *loginThrottleEntry) {
	delete(l.entries, key)
	l.recency.Remove(entry.element)
}
