package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
)

type saturatedAuthRepository struct {
	lookups atomic.Int32
}

func (r *saturatedAuthRepository) FindAdminByUsername(context.Context, string) (auth.Admin, error) {
	r.lookups.Add(1)
	return auth.Admin{}, auth.ErrAdminNotFound
}

func (*saturatedAuthRepository) CreateSession(context.Context, auth.Session) error { return nil }

func (*saturatedAuthRepository) FindSession(context.Context, [sha256.Size]byte, time.Time) (auth.Session, error) {
	return auth.Session{}, auth.ErrSessionNotFound
}

func (*saturatedAuthRepository) DeleteSession(context.Context, [sha256.Size]byte) error { return nil }

func TestLoginGlobalWorkSaturationFailsIdenticallyBeforeAccountLookup(t *testing.T) {
	repository := &saturatedAuthRepository{}
	origin, err := url.Parse("https://diary.example")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newAuthHandler(repository, origin, time.Now, AuthOptions{MaxConcurrentAuthWork: 1})
	if err != nil {
		t.Fatal(err)
	}
	handler.authWork <- struct{}{}

	var firstBody string
	for index, username := range []string{"admin", "rotated-attacker-name"} {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"wrong"}`))
		request.RemoteAddr = "203.0.113.20:4567"
		response := httptest.NewRecorder()
		handler.login(response, request)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("username %q status = %d, want 503", username, response.Code)
		}
		if index == 0 {
			firstBody = response.Body.String()
		} else if response.Body.String() != firstBody {
			t.Fatalf("saturation response varied by username: first=%q second=%q", firstBody, response.Body.String())
		}
	}
	if calls := repository.lookups.Load(); calls != 0 {
		t.Fatalf("repository lookups under saturation = %d, want 0", calls)
	}
	handler.throttle.mu.Lock()
	remaining := len(handler.throttle.entries)
	handler.throttle.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("neutral saturation retained %d empty throttle entries", remaining)
	}
}

func TestLoginAlreadyThrottledRejectsBeforeSaturatedAuthWork(t *testing.T) {
	repository := &saturatedAuthRepository{}
	origin, err := url.Parse("https://diary.example")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	handler, err := newAuthHandler(repository, origin, func() time.Time { return now }, AuthOptions{MaxConcurrentAuthWork: 1})
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < loginFailureLimit; attempt++ {
		if !handler.throttle.begin("admin", "203.0.113.20", now) {
			t.Fatalf("seed attempt %d was rejected", attempt+1)
		}
		handler.throttle.complete("admin", "203.0.113.20", now, throttleFailure)
	}
	before := throttleFailureCounts(handler.throttle)
	handler.authWork <- struct{}{}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
	request.RemoteAddr = "203.0.113.20:4567"
	response := httptest.NewRecorder()
	handler.login(response, request)

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("already-throttled request status = %d, want 429: %s", response.Code, response.Body.String())
	}
	if calls := repository.lookups.Load(); calls != 0 {
		t.Fatalf("repository lookups for already-throttled request = %d, want 0", calls)
	}
	if occupied := len(handler.authWork); occupied != 1 {
		t.Fatalf("already-throttled request changed auth-work occupancy to %d, want 1", occupied)
	}
	after := throttleFailureCounts(handler.throttle)
	if len(after) != len(before) {
		t.Fatalf("already-throttled request changed throttle entry count from %d to %d", len(before), len(after))
	}
	for key, want := range before {
		if got, ok := after[key]; !ok || got != want {
			t.Fatalf("already-throttled request changed throttle history %q from %d to %d (present=%v)", key, want, got, ok)
		}
	}
}

func TestLoginGlobalWorkSaturationCannotEvictThrottleHistory(t *testing.T) {
	repository := &saturatedAuthRepository{}
	origin, err := url.Parse("https://diary.example")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newAuthHandler(repository, origin, time.Now, AuthOptions{MaxConcurrentAuthWork: 1, MaxThrottleEntries: 4})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	seedThrottleFailures(t, handler.throttle, now,
		[2]string{"a", "203.0.113.1"},
		[2]string{"b", "203.0.113.1"},
		[2]string{"c", "203.0.113.2"},
		[2]string{"d", "203.0.113.2"},
	)
	before := throttleFailureCounts(handler.throttle)
	handler.authWork <- struct{}{}

	for attempt := 0; attempt < 20; attempt++ {
		body := fmt.Sprintf(`{"username":"rotated-%d","password":"wrong"}`, attempt)
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
		request.RemoteAddr = fmt.Sprintf("198.51.100.%d:4567", attempt+20)
		response := httptest.NewRecorder()
		handler.login(response, request)
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("saturated attempt %d status = %d, want 503", attempt+1, response.Code)
		}
	}
	if calls := repository.lookups.Load(); calls != 0 {
		t.Fatalf("repository lookups under saturation = %d, want 0", calls)
	}
	after := throttleFailureCounts(handler.throttle)
	if len(after) != len(before) {
		t.Fatalf("saturated traffic changed throttle entry count from %d to %d", len(before), len(after))
	}
	for key, want := range before {
		if got, ok := after[key]; !ok || got != want {
			t.Fatalf("saturated traffic changed throttle history %q from %d to %d (present=%v)", key, want, got, ok)
		}
	}
}

func TestClientIPTrustsForwardingOnlyFromConfiguredImmediateProxy(t *testing.T) {
	trusted, err := parseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		proxies    trustedProxySet
		want       string
	}{
		{name: "trust none by default", remoteAddr: "198.51.100.9:1234", forwarded: "203.0.113.8", want: "198.51.100.9"},
		{name: "untrusted peer cannot spoof", remoteAddr: "198.51.100.9:1234", forwarded: "203.0.113.8", proxies: trusted, want: "198.51.100.9"},
		{name: "trusted immediate peer", remoteAddr: "10.0.0.4:1234", forwarded: "203.0.113.8", proxies: trusted, want: "203.0.113.8"},
		{name: "trusted proxy chain", remoteAddr: "10.0.0.4:1234", forwarded: "192.0.2.66, 203.0.113.8, 10.0.0.3", proxies: trusted, want: "203.0.113.8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := clientIP(test.remoteAddr, test.forwarded, test.proxies); got != test.want {
				t.Fatalf("clientIP(%q, %q) = %q, want %q", test.remoteAddr, test.forwarded, got, test.want)
			}
		})
	}
}

func TestAuthOptionsRejectInvalidTrustedProxyCIDR(t *testing.T) {
	origin, err := url.Parse("https://diary.example")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newAuthHandler(&saturatedAuthRepository{}, origin, time.Now, AuthOptions{TrustedProxyCIDRs: []string{"not-a-cidr"}}); err == nil {
		t.Fatal("expected invalid trusted proxy CIDR to be rejected")
	}
}

func TestLoginRejectsOversizedUsernameBeforeAuthenticationWork(t *testing.T) {
	repository := &saturatedAuthRepository{}
	origin, err := url.Parse("https://diary.example")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newAuthHandler(repository, origin, time.Now, AuthOptions{MaxConcurrentAuthWork: 1})
	if err != nil {
		t.Fatal(err)
	}
	for name, username := range map[string]string{
		"byte limit": strings.Repeat("a", maxUsernameBytes+1),
		"rune limit": strings.Repeat("界", maxUsernameRunes+1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"wrong"}`))
			response := httptest.NewRecorder()
			handler.login(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
	if calls := repository.lookups.Load(); calls != 0 {
		t.Fatalf("repository lookups for oversized usernames = %d, want 0", calls)
	}
}

func TestLoginThrottleStopsRotatingNamesByIPAndRecoversAfterWindow(t *testing.T) {
	throttle := newLoginThrottle(64)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	ip := "203.0.113.44"
	for attempt := 0; attempt < loginIPFailureLimit; attempt++ {
		username := "rotated-" + string(rune('a'+attempt))
		if !throttle.begin(username, ip, now) {
			t.Fatalf("rotating attempt %d was rejected before IP limit", attempt+1)
		}
		throttle.complete(username, ip, now, throttleFailure)
	}
	if throttle.begin("another-name", ip, now) {
		t.Fatal("rotating username bypassed IP-only admission limit")
	}

	recoveredAt := now.Add(loginWindow + time.Nanosecond)
	if !throttle.begin("admin", ip, recoveredAt) {
		t.Fatal("legitimate login did not recover after throttle TTL")
	}
	throttle.complete("admin", ip, recoveredAt, throttleSuccess)
	throttle.mu.Lock()
	remaining := len(throttle.entries)
	throttle.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("throttle retained %d empty/expired entries after recovery", remaining)
	}
}

func TestLoginThrottleStateIsGloballyCappedWithCrossKeyEviction(t *testing.T) {
	const capacity = 4
	throttle := newLoginThrottle(capacity)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	for attempt := 0; attempt < 10; attempt++ {
		username := "user-" + string(rune('a'+attempt))
		ip := "203.0.113." + string(rune('1'+attempt))
		if !throttle.begin(username, ip, now.Add(time.Duration(attempt)*time.Second)) {
			t.Fatalf("attempt %d unexpectedly rejected", attempt+1)
		}
		throttle.complete(username, ip, now.Add(time.Duration(attempt)*time.Second), throttleFailure)
		throttle.mu.Lock()
		count := len(throttle.entries)
		throttle.mu.Unlock()
		if count > capacity {
			t.Fatalf("throttle entry count = %d, exceeds cap %d", count, capacity)
		}
	}
	throttle.mu.Lock()
	_, retainedOldest := throttle.entries[usernameIPThrottleKey("user-a", "203.0.113.1")]
	throttle.mu.Unlock()
	if retainedOldest {
		t.Fatal("least-recently-used throttle key was not evicted")
	}
}

func TestLoginThrottleProtectsRequestedKeysWhileMakingRoom(t *testing.T) {
	const capacity = 4
	throttle := newLoginThrottle(capacity)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	seedThrottleFailures(t, throttle, now,
		[2]string{"a", "203.0.113.1"},
		[2]string{"b", "203.0.113.1"},
		[2]string{"c", "203.0.113.2"},
		[2]string{"d", "203.0.113.2"},
	)
	ipKey := ipThrottleKey("203.0.113.1")
	before := throttleFailureCounts(throttle)
	wantIPFailures := before[ipKey]
	if wantIPFailures == 0 {
		t.Fatal("review reproduction did not retain the requested IP history")
	}

	if !throttle.begin("a", "203.0.113.1", now.Add(5*time.Second)) {
		t.Fatal("request was rejected despite an evictable unrequested entry")
	}
	after := throttleFailureCounts(throttle)
	if len(after) > capacity {
		t.Errorf("throttle entry count = %d after requested-key eviction, exceeds cap %d", len(after), capacity)
	}
	if got := after[ipKey]; got != wantIPFailures {
		t.Errorf("requested IP failure count = %d, want preserved count %d", got, wantIPFailures)
	}
	throttle.complete("a", "203.0.113.1", now.Add(5*time.Second), throttleNeutral)
}

func seedThrottleFailures(t *testing.T, throttle *loginThrottle, now time.Time, attempts ...[2]string) {
	t.Helper()
	for index, attempt := range attempts {
		at := now.Add(time.Duration(index) * time.Second)
		if !throttle.begin(attempt[0], attempt[1], at) {
			t.Fatalf("seed throttle attempt %d (%s, %s) was rejected", index+1, attempt[0], attempt[1])
		}
		throttle.complete(attempt[0], attempt[1], at, throttleFailure)
	}
}

func throttleFailureCounts(throttle *loginThrottle) map[string]int {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	counts := make(map[string]int, len(throttle.entries))
	for key, entry := range throttle.entries {
		counts[key] = len(entry.failures)
	}
	return counts
}
