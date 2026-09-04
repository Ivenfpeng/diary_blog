package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
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
