package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/go-chi/chi/v5"
)

const adminPostBodyLimit = 2 * 1024 * 1024

type adminPostAPI struct {
	posts *posts.Service
}

type postInputRequest struct {
	Slug         string  `json:"slug"`
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	ContentMD    string  `json:"content_md"`
	CategoryID   *int64  `json:"category_id"`
	CoverMediaID *int64  `json:"cover_media_id"`
	TagIDs       []int64 `json:"tag_ids"`
}

type savePostRequest struct {
	postInputRequest
	ExpectedRevision int64 `json:"expected_revision"`
}

type revisionRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type previewRequest struct {
	ContentMD string `json:"content_md"`
}

type postResponse struct {
	ID           int64              `json:"id"`
	Slug         string             `json:"slug"`
	Title        string             `json:"title"`
	Summary      string             `json:"summary"`
	ContentMD    string             `json:"content_md"`
	ContentHTML  string             `json:"content_html"`
	ContentPlain string             `json:"content_plain"`
	Status       content.PostStatus `json:"status"`
	CategoryID   *int64             `json:"category_id"`
	CoverMediaID *int64             `json:"cover_media_id"`
	TagIDs       []int64            `json:"tag_ids"`
	Revision     int64              `json:"revision"`
	PublishedAt  *time.Time         `json:"published_at"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type revisionResponse struct {
	ID         int64     `json:"id"`
	PostID     int64     `json:"post_id"`
	Revision   int64     `json:"revision"`
	Title      string    `json:"title"`
	Slug       string    `json:"slug"`
	Summary    string    `json:"summary"`
	ContentMD  string    `json:"content_md"`
	CategoryID *int64    `json:"category_id"`
	TagIDs     []int64   `json:"tag_ids"`
	CreatedAt  time.Time `json:"created_at"`
}

func newAdminPostAPI(service *posts.Service) *adminPostAPI {
	return &adminPostAPI{posts: service}
}

func (a *adminPostAPI) routes(router chi.Router, requireSession, requireCSRF func(http.Handler) http.Handler) {
	router.Group(func(router chi.Router) {
		router.Use(requireSession, requireCSRF)
		router.Get("/api/admin/posts", a.list)
		router.Post("/api/admin/posts", a.create)
		router.Get("/api/admin/posts/{id}", a.get)
		router.Put("/api/admin/posts/{id}", a.save)
		router.Post("/api/admin/posts/{id}/preview", a.preview)
		router.Post("/api/admin/posts/{id}/publish", a.publish)
		router.Post("/api/admin/posts/{id}/archive", a.archive)
		router.Get("/api/admin/posts/{id}/revisions", a.listRevisions)
		router.Post("/api/admin/posts/{id}/revisions/{revisionID}/restore", a.restoreRevision)
	})
}

func (a *adminPostAPI) create(w http.ResponseWriter, r *http.Request) {
	var input postInputRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	post, err := a.posts.CreateDraft(r.Context(), input.postInput())
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"post": newPostResponse(post)})
}

func (a *adminPostAPI) get(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	post, err := a.posts.Get(r.Context(), id)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": newPostResponse(post)})
}

func (a *adminPostAPI) list(w http.ResponseWriter, r *http.Request) {
	filter, ok := adminFilter(w, r)
	if !ok {
		return
	}
	items, total, err := a.posts.ListAdmin(r.Context(), filter)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	response := make([]postResponse, 0, len(items))
	for _, item := range items {
		response = append(response, newPostResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"posts": response, "total": total, "page": filter.Page, "page_size": filter.PageSize,
	})
}

func (a *adminPostAPI) save(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	var input savePostRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	post, err := a.posts.SaveDraft(r.Context(), id, input.postInput(), input.ExpectedRevision)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": newPostResponse(post)})
}

func (a *adminPostAPI) preview(w http.ResponseWriter, r *http.Request) {
	if _, ok := postID(w, r); !ok {
		return
	}
	var input previewRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	rendered, err := a.posts.Preview(input.ContentMD)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"html": rendered.HTML, "plain_text": rendered.PlainText,
		"headings": rendered.Headings, "reading_minutes": rendered.ReadingMinutes,
	})
}

func (a *adminPostAPI) publish(w http.ResponseWriter, r *http.Request) {
	a.changeRevision(w, r, a.posts.Publish)
}

func (a *adminPostAPI) archive(w http.ResponseWriter, r *http.Request) {
	a.changeRevision(w, r, a.posts.Archive)
}

func (a *adminPostAPI) changeRevision(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int64) (content.Post, error)) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	var input revisionRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	post, err := action(r.Context(), id, input.ExpectedRevision)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": newPostResponse(post)})
}

func (a *adminPostAPI) listRevisions(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	revisions, err := a.posts.ListRevisions(r.Context(), id)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	response := make([]revisionResponse, 0, len(revisions))
	for _, revision := range revisions {
		response = append(response, newRevisionResponse(revision))
	}
	writeJSON(w, http.StatusOK, map[string]any{"revisions": response})
}

func (a *adminPostAPI) restoreRevision(w http.ResponseWriter, r *http.Request) {
	id, ok := postID(w, r)
	if !ok {
		return
	}
	revisionID, err := strconv.ParseInt(chi.URLParam(r, "revisionID"), 10, 64)
	if err != nil || revisionID <= 0 {
		writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
		return
	}
	var input revisionRequest
	if !decodeAdminJSON(w, r, &input) {
		return
	}
	post, err := a.posts.RestoreRevision(r.Context(), id, revisionID, input.ExpectedRevision)
	if err != nil {
		a.writePostError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": newPostResponse(post)})
}

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, adminPostBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		writeAPIError(w, r, http.StatusBadRequest, "invalid_request", "The request body is invalid.")
		return false
	}
	return true
}

func adminFilter(w http.ResponseWriter, r *http.Request) (posts.AdminFilter, bool) {
	query := r.URL.Query()
	filter := posts.AdminFilter{Query: query.Get("q"), Page: 1, PageSize: 20}
	if status := query.Get("status"); status != "" {
		filter.Status = content.PostStatus(status)
		if filter.Status != content.StatusDraft && filter.Status != content.StatusPublished && filter.Status != content.StatusArchived {
			writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
			return posts.AdminFilter{}, false
		}
	}
	var err error
	if value := query.Get("page"); value != "" {
		filter.Page, err = strconv.Atoi(value)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
			return posts.AdminFilter{}, false
		}
	}
	if value := query.Get("page_size"); value != "" {
		filter.PageSize, err = strconv.Atoi(value)
		if err != nil {
			writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
			return posts.AdminFilter{}, false
		}
	}
	return filter, true
}

func postID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
		return 0, false
	}
	return id, true
}

func (input postInputRequest) postInput() content.PostInput {
	return content.PostInput{
		Slug: input.Slug, Title: input.Title, Summary: input.Summary, ContentMD: input.ContentMD,
		CategoryID: input.CategoryID, CoverMediaID: input.CoverMediaID, TagIDs: input.TagIDs,
	}
}

func newPostResponse(post content.Post) postResponse {
	return postResponse{
		ID: post.ID, Slug: post.Slug, Title: post.Title, Summary: post.Summary, ContentMD: post.ContentMD,
		ContentHTML: post.ContentHTML, ContentPlain: post.ContentPlain, Status: post.Status,
		CategoryID: post.CategoryID, CoverMediaID: post.CoverMediaID, TagIDs: post.TagIDs,
		Revision: post.Revision, PublishedAt: post.PublishedAt, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt,
	}
}

func newRevisionResponse(revision content.PostRevision) revisionResponse {
	return revisionResponse{
		ID: revision.ID, PostID: revision.PostID, Revision: revision.Revision, Title: revision.Title,
		Slug: revision.Slug, Summary: revision.Summary, ContentMD: revision.ContentMD,
		CategoryID: revision.CategoryID, TagIDs: revision.TagIDs, CreatedAt: revision.CreatedAt,
	}
}

func (a *adminPostAPI) writePostError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, posts.ErrValidation):
		writeAPIError(w, r, http.StatusBadRequest, "post_validation", "The article data is invalid.")
	case errors.Is(err, posts.ErrNotFound):
		writeAPIError(w, r, http.StatusNotFound, "post_not_found", "The article was not found.")
	case errors.Is(err, posts.ErrConflict):
		writeAPIError(w, r, http.StatusConflict, "post_conflict", "The article changed on the server.")
	default:
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
	}
}
