package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/auth"
	appdb "github.com/Ivenfpeng/diary_blog/internal/database"
	sqliterepo "github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/web"
)

func TestAdminPostAPICreateGetListAndSaveDraft(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	post := createAdminDraft(t, server, session, csrf, map[string]any{
		"slug": "first-draft", "title": "First draft", "summary": "A short summary", "content_md": "# Hello", "tag_ids": []int64{},
	})
	if post["status"] != "draft" || post["revision"].(float64) != 1 {
		t.Fatalf("created post = %#v, want draft revision 1", post)
	}
	id := int64(post["id"].(float64))

	response := adminRequest(t, server, session, csrf, http.MethodGet, "/api/admin/posts/"+strconv.FormatInt(id, 10), nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	got := decodeObject(t, response)["post"].(map[string]any)
	if got["title"] != "First draft" || got["content_md"] != "# Hello" {
		t.Fatalf("get post = %#v", got)
	}

	response = adminRequest(t, server, session, csrf, http.MethodGet, "/api/admin/posts?status=draft&page=1&page_size=10&q=First", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	listing := decodeObject(t, response)
	if listing["total"].(float64) != 1 || len(listing["posts"].([]any)) != 1 {
		t.Fatalf("list response = %#v", listing)
	}

	response = adminRequest(t, server, session, csrf, http.MethodPut, "/api/admin/posts/"+strconv.FormatInt(id, 10), map[string]any{
		"slug": "first-draft", "title": "Edited draft", "summary": "Updated", "content_md": "# Updated", "tag_ids": []int64{}, "expected_revision": 1,
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("save status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	saved := decodeObject(t, response)["post"].(map[string]any)
	if saved["title"] != "Edited draft" || saved["revision"].(float64) != 2 {
		t.Fatalf("saved post = %#v", saved)
	}
}

func TestAdminPostAPIPreviewPublishArchiveAndRevisions(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	post := createAdminDraft(t, server, session, csrf, map[string]any{
		"slug": "publish-me", "title": "Publish me", "content_md": "# Heading", "tag_ids": []int64{},
	})
	id := int64(post["id"].(float64))

	response := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts/"+strconv.FormatInt(id, 10)+"/preview", map[string]any{"content_md": "# Preview"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("preview status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	preview := decodeObject(t, response)
	if preview["html"] == "" || preview["plain_text"] != "Preview" {
		t.Fatalf("preview response = %#v", preview)
	}

	response = adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts/"+strconv.FormatInt(id, 10)+"/publish", map[string]any{"expected_revision": 1})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	published := decodeObject(t, response)["post"].(map[string]any)
	if published["status"] != "published" || published["revision"].(float64) != 2 {
		t.Fatalf("published post = %#v", published)
	}

	response = adminRequest(t, server, session, csrf, http.MethodGet, "/api/admin/posts/"+strconv.FormatInt(id, 10)+"/revisions", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("revisions status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	revisions := decodeObject(t, response)["revisions"].([]any)
	if len(revisions) != 1 {
		t.Fatalf("revisions = %#v, want one revision", revisions)
	}
	revisionID := int64(revisions[0].(map[string]any)["id"].(float64))

	response = adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts/"+strconv.FormatInt(id, 10)+"/revisions/"+strconv.FormatInt(revisionID, 10)+"/restore", map[string]any{"expected_revision": 2})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("restore status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	restored := decodeObject(t, response)["post"].(map[string]any)
	if restored["status"] != "published" || restored["revision"].(float64) != 3 {
		t.Fatalf("restored post = %#v", restored)
	}

	response = adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts/"+strconv.FormatInt(id, 10)+"/archive", map[string]any{"expected_revision": 3})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("archive status = %d, want 200: %s", response.StatusCode, readBody(t, response))
	}
	archived := decodeObject(t, response)["post"].(map[string]any)
	if archived["status"] != "archived" || archived["revision"].(float64) != 4 {
		t.Fatalf("archived post = %#v", archived)
	}
}

func TestAdminPostAPIRejectsUnauthenticatedAndMalformedRequests(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	response := adminRequest(t, server, nil, nil, http.MethodGet, "/api/admin/posts?page=1&page_size=10", nil)
	assertAPIError(t, response, http.StatusUnauthorized, "authentication_required", "Administrator authentication is required.")

	response = adminRequest(t, server, nil, nil, http.MethodGet, "/api/admin/unknown", nil)
	assertAPIError(t, response, http.StatusUnauthorized, "authentication_required", "Administrator authentication is required.")

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/admin/posts", bytes.NewBufferString(`{"title":"draft","unknown":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", authOrigin)
	request.Header.Set(web.CSRFHeaderName, csrf.Value)
	request.AddCookie(session)
	request.AddCookie(csrf)
	response = do(t, server, request)
	assertAPIError(t, response, http.StatusBadRequest, "invalid_request", "The request body is invalid.")

	request, err = http.NewRequest(http.MethodPost, server.URL+"/api/admin/posts", bytes.NewBufferString(`{"title":`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", authOrigin)
	request.Header.Set(web.CSRFHeaderName, csrf.Value)
	request.AddCookie(session)
	request.AddCookie(csrf)
	response = do(t, server, request)
	assertAPIError(t, response, http.StatusBadRequest, "invalid_request", "The request body is invalid.")

	request, err = http.NewRequest(http.MethodPost, server.URL+"/api/admin/posts", strings.NewReader(`{"content_md":"`+strings.Repeat("x", 2*1024*1024)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", authOrigin)
	request.Header.Set(web.CSRFHeaderName, csrf.Value)
	request.AddCookie(session)
	request.AddCookie(csrf)
	response = do(t, server, request)
	assertAPIError(t, response, http.StatusBadRequest, "invalid_request", "The request body is invalid.")

	response = adminRequest(t, server, session, nil, http.MethodPost, "/api/admin/posts", map[string]any{"tag_ids": []int64{}})
	assertAPIError(t, response, http.StatusForbidden, "csrf_rejected", "The request origin or CSRF token is invalid.")
}

func TestAdminPostAPIMapsValidationNotFoundAndConflictErrors(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	response := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts", map[string]any{"slug": "Bad Slug", "tag_ids": []int64{}})
	assertAPIError(t, response, http.StatusBadRequest, "post_validation", "The article data is invalid.")

	response = adminRequest(t, server, session, csrf, http.MethodGet, "/api/admin/posts/999", nil)
	assertAPIError(t, response, http.StatusNotFound, "post_not_found", "The article was not found.")

	post := createAdminDraft(t, server, session, csrf, map[string]any{"slug": "conflict", "title": "Conflict", "content_md": "text", "tag_ids": []int64{}})
	id := int64(post["id"].(float64))
	response = adminRequest(t, server, session, csrf, http.MethodPut, "/api/admin/posts/"+strconv.FormatInt(id, 10), map[string]any{
		"slug": "conflict", "title": "Stale", "content_md": "text", "tag_ids": []int64{}, "expected_revision": 99,
	})
	assertAPIError(t, response, http.StatusConflict, "post_conflict", "The article changed on the server.")
}

func TestAdminPostAPIRejectsMissingParentResources(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	for _, test := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "preview", method: http.MethodPost, path: "/api/admin/posts/999/preview", body: map[string]any{"content_md": "# Preview"}},
		{name: "revisions", method: http.MethodGet, path: "/api/admin/posts/999/revisions"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := adminRequest(t, server, session, csrf, test.method, test.path, test.body)
			assertAPIError(t, response, http.StatusNotFound, "post_not_found", "The article was not found.")
		})
	}
}

func TestTaxonomyManagementAPIRejectsSameTableDuplicateSlug(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	first := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/categories", map[string]any{"name": "Engineering", "slug": "engineering"})
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("create category = %d: %s", first.StatusCode, readBody(t, first))
	}
	firstCategory := decodeObject(t, first)["category"].(map[string]any)
	duplicate := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/categories", map[string]any{"name": "Again", "slug": "engineering"})
	assertAPIError(t, duplicate, http.StatusBadRequest, "management_validation", "The management data is invalid.")

	second := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/categories", map[string]any{"name": "Writing", "slug": "writing"})
	if second.StatusCode != http.StatusCreated {
		t.Fatalf("create second category = %d: %s", second.StatusCode, readBody(t, second))
	}
	secondCategory := decodeObject(t, second)["category"].(map[string]any)
	update := adminRequest(t, server, session, csrf, http.MethodPut, "/api/admin/categories/"+strconv.Itoa(int(secondCategory["id"].(float64))), map[string]any{"name": "Writing", "slug": firstCategory["slug"]})
	assertAPIError(t, update, http.StatusBadRequest, "management_validation", "The management data is invalid.")
}

func TestMediaManagementAPIStoresValidatedUploadInConfiguredDirectory(t *testing.T) {
	mediaDir := t.TempDir()
	server, _, session, csrf := newAdminServerWithMediaDir(t, mediaDir)
	response := multipartAdminRequest(t, server, session, csrf, "/api/admin/media", "image.png", tinyPNG(t), "A green pixel")
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d: %s", response.StatusCode, readBody(t, response))
	}
	media := decodeObject(t, response)["media"].(map[string]any)
	path := media["path"].(string)
	if !strings.HasPrefix(path, "2026/09/") || strings.Contains(path, "media/") {
		t.Fatalf("media path = %q, want public relative year/month path", path)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, filepath.FromSlash(path))); err != nil {
		t.Fatalf("stored media missing: %v", err)
	}
	if media["mime_type"] != "image/png" || media["width"] != float64(1) || media["height"] != float64(1) {
		t.Fatalf("uploaded metadata = %#v", media)
	}

	invalid := multipartAdminRequest(t, server, session, csrf, "/api/admin/media", "not-image.txt", []byte("not an image"), "Not an image")
	assertAPIError(t, invalid, http.StatusBadRequest, "media_validation", "The uploaded image is invalid.")
}

func TestMediaManagementAPIRejectsOversizedUpload(t *testing.T) {
	server, _, session, csrf := newAdminServerWithMediaDir(t, t.TempDir())
	response := multipartAdminRequest(t, server, session, csrf, "/api/admin/media", "large.png", bytes.Repeat([]byte{'x'}, int(10*1024*1024+1)), "Oversized")
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized upload status = %d: %s", response.StatusCode, readBody(t, response))
	}
}

func TestSettingsManagementAPIValidatesInput(t *testing.T) {
	server, _, session, csrf := newAdminServer(t)
	invalid := adminRequest(t, server, session, csrf, http.MethodPut, "/api/admin/settings", map[string]any{"site_title": "", "description": "Notes"})
	assertAPIError(t, invalid, http.StatusBadRequest, "management_validation", "The management data is invalid.")
	saved := adminRequest(t, server, session, csrf, http.MethodPut, "/api/admin/settings", map[string]any{"site_title": "Diary", "description": "Notes", "author": "Iven"})
	if saved.StatusCode != http.StatusOK {
		t.Fatalf("save settings = %d: %s", saved.StatusCode, readBody(t, saved))
	}
}

func newAdminServer(t *testing.T) (*httptest.Server, *sql.DB, *http.Cookie, *http.Cookie) {
	return newAdminServerWithMediaDir(t, "")
}

func newAdminServerWithMediaDir(t *testing.T, mediaDir string) (*httptest.Server, *sql.DB, *http.Cookie, *http.Cookie) {
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
	repo := sqliterepo.NewPostRepository(db)
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertAdmin(ctx, "admin", passwordHash, now); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(web.NewServer(repo, web.ServerOptions{AuthRepository: repo, MediaDir: mediaDir, PublicURL: authOrigin, Clock: func() time.Time { return now }}))
	t.Cleanup(server.Close)
	response := login(t, server, "admin", "secret")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d: %s", response.StatusCode, readBody(t, response))
	}
	cookies := cookieMap(response.Cookies())
	_ = response.Body.Close()
	return server, db, cookies[web.SessionCookieName], cookies[web.CSRFCookieName]
}

func multipartAdminRequest(t *testing.T, server *httptest.Server, session, csrf *http.Cookie, path, filename string, contents []byte, altText string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("alt_text", altText); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+path, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", authOrigin)
	request.Header.Set(web.CSRFHeaderName, csrf.Value)
	request.AddCookie(session)
	request.AddCookie(csrf)
	return do(t, server, request)
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	image := image.NewRGBA(image.Rect(0, 0, 1, 1))
	image.Set(0, 0, color.RGBA{G: 255, A: 255})
	if err := png.Encode(&body, image); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func createAdminDraft(t *testing.T, server *httptest.Server, session, csrf *http.Cookie, input map[string]any) map[string]any {
	t.Helper()
	response := adminRequest(t, server, session, csrf, http.MethodPost, "/api/admin/posts", input)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", response.StatusCode, readBody(t, response))
	}
	return decodeObject(t, response)["post"].(map[string]any)
}

func adminRequest(t *testing.T, server *httptest.Server, session, csrf *http.Cookie, method, path string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		request.Header.Set("Origin", authOrigin)
		if csrf != nil {
			request.Header.Set(web.CSRFHeaderName, csrf.Value)
		}
	}
	if session != nil {
		request.AddCookie(session)
	}
	if csrf != nil {
		request.AddCookie(csrf)
	}
	return do(t, server, request)
}

func decodeObject(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	defer response.Body.Close()
	var value map[string]any
	if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func assertAPIError(t *testing.T, response *http.Response, status int, code, message string) {
	t.Helper()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, status, readBody(t, response))
	}
	value := decodeObject(t, response)["error"].(map[string]any)
	if value["code"] != code || value["message"] != message {
		t.Fatalf("error = %#v, want code=%q message=%q", value, code, message)
	}
	if _, ok := value["fields"].(map[string]any); !ok {
		t.Fatalf("error fields = %#v, want object", value["fields"])
	}
	if value["request_id"] == "" {
		t.Fatalf("error request_id = %#v, want non-empty", value["request_id"])
	}
}
