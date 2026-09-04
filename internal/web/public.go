package web

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	webassets "github.com/Ivenfpeng/diary_blog/web"
	"github.com/go-chi/chi/v5"
)

const publicPageSize = 20

type publicHandler struct {
	repository posts.Repository
	templates  *template.Template
	renderer   *content.Renderer
}

type pageData struct {
	Title           string
	Heading         string
	Description     string
	CanonicalURL    string
	Posts           []content.Post
	Post            content.Post
	Body            template.HTML
	Headings        []content.Heading
	ReadingMinutes  int
	CurrentCategory string
	CurrentTag      string
	Status          int
}

func newPublicHandler(repository posts.Repository) (*publicHandler, error) {
	base, err := template.New("base.html").Funcs(template.FuncMap{
		"formatDate": func(value *time.Time) string {
			if value == nil {
				return "Unpublished"
			}
			return value.Format("02 Jan 2006")
		},
		"formatTime": func(value *time.Time) string {
			if value == nil {
				return ""
			}
			return value.Format(time.RFC3339)
		},
	}).ParseFS(webassets.Assets, "templates/base.html")
	if err != nil {
		return nil, err
	}
	return &publicHandler{repository: repository, templates: base, renderer: content.NewRenderer()}, nil
}

func (h *publicHandler) routes(router chi.Router) {
	router.Get("/", h.home)
	router.Get("/posts/{slug}", h.article)
	router.Get("/categories/{slug}", h.category)
	router.Get("/tags/{slug}", h.tag)
	router.Get("/archive", h.archive)
}

func (h *publicHandler) home(w http.ResponseWriter, r *http.Request) {
	h.renderListing(w, r, "home.html", pageData{Title: "Articles", Heading: "Latest writing", Description: "Technical notes, field reports, and durable explanations."}, posts.PublishedFilter{Page: 1, PageSize: publicPageSize})
}

func (h *publicHandler) category(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	h.renderListing(w, r, "list.html", pageData{Title: "Category: " + slug, Heading: "Category: " + slug, Description: "Published articles in this category.", CurrentCategory: slug}, posts.PublishedFilter{CategorySlug: slug, Page: 1, PageSize: publicPageSize})
}

func (h *publicHandler) tag(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	h.renderListing(w, r, "list.html", pageData{Title: "Tag: " + slug, Heading: "Tag: " + slug, Description: "Published articles with this tag.", CurrentTag: slug}, posts.PublishedFilter{TagSlug: slug, Page: 1, PageSize: publicPageSize})
}

func (h *publicHandler) archive(w http.ResponseWriter, r *http.Request) {
	h.renderListing(w, r, "list.html", pageData{Title: "Archive", Heading: "Archive", Description: "All published writing, ordered by publication date."}, posts.PublishedFilter{Page: 1, PageSize: publicPageSize})
}

func (h *publicHandler) renderListing(w http.ResponseWriter, r *http.Request, templateName string, data pageData, filter posts.PublishedFilter) {
	items, _, err := h.repository.ListPublished(r.Context(), filter)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load these articles.")
		return
	}
	data.Posts = items
	data.CanonicalURL = canonicalURL(r)
	h.render(w, http.StatusOK, templateName, data)
}

func (h *publicHandler) article(w http.ResponseWriter, r *http.Request) {
	post, err := h.repository.GetPublishedBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err == posts.ErrNotFound {
		h.error(w, r, http.StatusNotFound, "This article is not published or no longer exists.")
		return
	}
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load this article.")
		return
	}
	rendered, err := h.renderer.Render(post.ContentMD)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare this article.")
		return
	}
	h.render(w, http.StatusOK, "article.html", pageData{
		Title: post.Title, Description: post.Summary, CanonicalURL: canonicalURL(r), Post: post,
		Body: template.HTML(post.ContentHTML), Headings: rendered.Headings, ReadingMinutes: rendered.ReadingMinutes,
	})
}

func (h *publicHandler) error(w http.ResponseWriter, r *http.Request, status int, description string) {
	h.render(w, status, "error.html", pageData{Title: "Not found", Heading: "Nothing here", Description: description, Status: status})
}

func (h *publicHandler) render(w http.ResponseWriter, status int, pageTemplate string, data pageData) {
	file, err := fs.ReadFile(webassets.Assets, "templates/"+pageTemplate)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	clone, err := h.templates.Clone()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if _, err := clone.Parse(string(file)); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var output bytes.Buffer
	if err := clone.ExecuteTemplate(&output, "base", data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(output.Bytes())
}

func canonicalURL(r *http.Request) string {
	scheme := "http"
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "https" || forwarded == "http" {
		scheme = forwarded
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path}).String()
}
