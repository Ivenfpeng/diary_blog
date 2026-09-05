package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadyFailsWhenStorageIsNotReady(t *testing.T) {
	handler := NewServer(nil, ServerOptions{Ready: func() error { return errors.New("data directory is not writable") }})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestRecoveryReturnsRequestIDWithoutPanicDetails(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := recoverPanics(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret stack detail") }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.WithValue(request.Context(), requestIDKey, "request-123"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("recovery status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if got := recorder.Header().Get("X-Request-ID"); got != "request-123" {
		t.Fatalf("recovery request id = %q, want request-123", got)
	}
	if strings.Contains(recorder.Body.String(), "secret stack detail") {
		t.Fatalf("recovery response leaked panic details: %q", recorder.Body.String())
	}
}

func TestAccessLogOmitsCookies(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := accessLog(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Cookie", "session=secret")

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if strings.Contains(output.String(), "secret") || strings.Contains(output.String(), "Cookie") {
		t.Fatalf("access log leaked cookie: %s", output.String())
	}
}
