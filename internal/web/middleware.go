package web

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	requestIDKey   contextKey = "request_id"
	authSessionKey contextKey = "auth_session"
)

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func recoverPanics(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic while serving request", "request_id", r.Context().Value(requestIDKey))
					if requestID, ok := r.Context().Value(requestIDKey).(string); ok {
						w.Header().Set("X-Request-ID", requestID)
					}
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func accessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			response := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				route := r.URL.Path
				if routeContext := chi.RouteContext(r.Context()); routeContext != nil && routeContext.RoutePattern() != "" {
					route = routeContext.RoutePattern()
				}
				logger.Info("http request", "request_id", r.Context().Value(requestIDKey), "method", r.Method, "route", route, "status", response.status, "bytes", response.bytes, "duration", time.Since(started))
			}()
			next.ServeHTTP(response, r)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(value []byte) (int, error) {
	count, err := w.ResponseWriter.Write(value)
	w.bytes += count
	return count, err
}

func (h *authHandler) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := h.sessionFromRequest(r)
		if !ok {
			writeAPIError(w, r, http.StatusUnauthorized, "authentication_required", "Administrator authentication is required.")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authSessionKey, session)))
	})
}

func (h *authHandler) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if !sameOrigin(r.Header.Get("Origin"), h.origin) {
			writeAPIError(w, r, http.StatusForbidden, "csrf_rejected", "The request origin or CSRF token is invalid.")
			return
		}
		cookie, err := r.Cookie(CSRFCookieName)
		header := r.Header.Get(CSRFHeaderName)
		if err != nil || header == "" || cookie.Value == "" || !constantTimeEqual(header, cookie.Value) {
			writeAPIError(w, r, http.StatusForbidden, "csrf_rejected", "The request origin or CSRF token is invalid.")
			return
		}
		session, ok := r.Context().Value(authSessionKey).(auth.Session)
		csrfHash := auth.HashToken(header)
		if !ok || subtle.ConstantTimeCompare(csrfHash[:], session.CSRFHash[:]) != 1 {
			writeAPIError(w, r, http.StatusForbidden, "csrf_rejected", "The request origin or CSRF token is invalid.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *authHandler) sessionFromRequest(r *http.Request) (auth.Session, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return auth.Session{}, false
	}
	session, err := h.repository.FindSession(r.Context(), auth.HashToken(cookie.Value), h.clock())
	return session, err == nil
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func sameOrigin(value string, expected *url.URL) bool {
	origin, err := url.Parse(value)
	if err != nil || origin.Scheme == "" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return false
	}
	return strings.EqualFold(origin.Scheme, expected.Scheme) &&
		strings.EqualFold(origin.Hostname(), expected.Hostname()) &&
		effectivePort(origin) == effectivePort(expected)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
