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
	"github.com/Ivenfpeng/diary_blog/internal/site"
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
	Title                string
	Heading              string
	Description          string
	CanonicalURL         string
	Robots               string
	OpenGraphTitle       string
	OpenGraphDescription string
	OpenGraphURL         string
	OpenGraphType        string
	OpenGraphImage       string
	Posts                []posts.PublishedPost
	Post                 posts.PublishedPost
	Body                 template.HTML
	Headings             []content.Heading
	ReadingMinutes       int
	Categories           []posts.Taxonomy
	Tags                 []posts.Taxonomy
	CurrentCategory      string
	CurrentTag           string
	SearchQuery          string
	SearchTotal          int
	Status               int
}

func newPublicHandler(repository posts.Repository) (*publicHandler, error) {
	base, err := template.New("base.html").Funcs(template.FuncMap{
		"formatDate": func(value time.Time) string {
			return value.Format("02 Jan 2006")
		},
		"formatTime": func(value time.Time) string {
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
	router.Get("/search", h.search)
	router.Get("/rss.xml", h.rss)
	router.Get("/sitemap.xml", h.sitemap)
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

func (h *publicHandler) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	taxonomy, err := h.repository.ListPublishedTaxonomy(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load the site navigation.")
		return
	}
	data := pageData{
		Title: "Search", Heading: "Search", Description: "Search published technical notes.",
		CanonicalURL: canonicalURL(r), Robots: "noindex,follow", SearchQuery: query,
		Categories: taxonomy.Categories, Tags: taxonomy.Tags,
	}
	if query != "" {
		items, total, err := h.repository.SearchPublished(r.Context(), query, 1, publicPageSize)
		if err != nil {
			h.error(w, r, http.StatusBadRequest, "We could not search for that phrase.")
			return
		}
		data.Posts, data.SearchTotal = items, total
	}
	h.render(w, http.StatusOK, "search.html", data)
}

func (h *publicHandler) rss(w http.ResponseWriter, r *http.Request) {
	items, err := h.allPublished(r)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the RSS feed.")
		return
	}
	feed, err := site.NewRSS(publicBaseURL(r), time.Now)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the RSS feed.")
		return
	}
	output, err := feed.Generate(items)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the RSS feed.")
		return
	}
	h.writeXML(w, output)
}

func (h *publicHandler) sitemap(w http.ResponseWriter, r *http.Request) {
	items, err := h.allPublished(r)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the sitemap.")
		return
	}
	sitemap, err := site.NewSitemap(publicBaseURL(r), time.Now)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the sitemap.")
		return
	}
	output, err := sitemap.Generate(items)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the sitemap.")
		return
	}
	h.writeXML(w, output)
}

func (h *publicHandler) renderListing(w http.ResponseWriter, r *http.Request, templateName string, data pageData, filter posts.PublishedFilter) {
	items, _, err := h.repository.ListPublished(r.Context(), filter)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load these articles.")
		return
	}
	taxonomy, err := h.repository.ListPublishedTaxonomy(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load the site navigation.")
		return
	}
	data.Posts = items
	data.Categories = taxonomy.Categories
	data.Tags = taxonomy.Tags
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
	taxonomy, err := h.repository.ListPublishedTaxonomy(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load the site navigation.")
		return
	}
	data := pageData{
		Title: post.Title, Description: post.Summary, CanonicalURL: canonicalURL(r), Post: post,
		Body: template.HTML(post.ContentHTML), Headings: rendered.Headings, ReadingMinutes: rendered.ReadingMinutes,
		Categories: taxonomy.Categories, Tags: taxonomy.Tags,
	}
	if post.CoverMediaPath != "" {
		data.OpenGraphImage = mediaURL(r, post.CoverMediaPath)
	}
	h.render(w, http.StatusOK, "article.html", data)
}

func (h *publicHandler) error(w http.ResponseWriter, r *http.Request, status int, description string) {
	h.render(w, status, "error.html", pageData{Title: "Not found", Heading: "Nothing here", Description: description, CanonicalURL: canonicalURL(r), Status: status})
}

func (h *publicHandler) render(w http.ResponseWriter, status int, pageTemplate string, data pageData) {
	data.applySEO(status)
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

func (h *publicHandler) allPublished(r *http.Request) ([]posts.PublishedPost, error) {
	result := make([]posts.PublishedPost, 0)
	for page := 1; ; page++ {
		items, total, err := h.repository.ListPublished(r.Context(), posts.PublishedFilter{Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		if len(result) >= total || len(items) == 0 {
			return result, nil
		}
	}
}

func (h *publicHandler) writeXML(w http.ResponseWriter, output []byte) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

func (data *pageData) applySEO(status int) {
	if data.Title == "" {
		data.Title = "Diary Blog"
	}
	if data.Description == "" {
		data.Description = "Technical notes, field reports, and durable explanations."
	}
	if data.Robots == "" {
		if status >= http.StatusBadRequest {
			data.Robots = "noindex,follow"
		} else {
			data.Robots = "index,follow"
		}
	}
	if data.OpenGraphTitle == "" {
		data.OpenGraphTitle = data.Title
	}
	if data.OpenGraphDescription == "" {
		data.OpenGraphDescription = data.Description
	}
	if data.OpenGraphURL == "" {
		data.OpenGraphURL = data.CanonicalURL
	}
	if data.OpenGraphType == "" {
		data.OpenGraphType = "website"
	}
}

func canonicalURL(r *http.Request) string {
	return (&url.URL{Scheme: requestScheme(r), Host: r.Host, Path: r.URL.Path}).String()
}

func publicBaseURL(r *http.Request) string {
	return (&url.URL{Scheme: requestScheme(r), Host: r.Host}).String()
}

func requestScheme(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "https" || forwarded == "http" {
		scheme = forwarded
	}
	return scheme
}

func mediaURL(r *http.Request, path string) string {
	parts := append([]string{"media"}, strings.Split(path, "/")...)
	return (&url.URL{Scheme: requestScheme(r), Host: r.Host, Path: "/" + strings.Join(parts, "/")}).String()
}
