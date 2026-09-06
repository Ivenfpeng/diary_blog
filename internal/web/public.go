package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
	"github.com/Ivenfpeng/diary_blog/internal/posts"
	"github.com/Ivenfpeng/diary_blog/internal/repository/sqlite"
	"github.com/Ivenfpeng/diary_blog/internal/site"
	webassets "github.com/Ivenfpeng/diary_blog/web"
	"github.com/go-chi/chi/v5"
)

const publicPageSize = 20

type publicSettingsRepository interface {
	GetSettings(context.Context) (sqlite.Settings, error)
}

type publicSiteSettings struct {
	SiteTitle   string
	Description string
	Author      string
	Navigation  []sqlite.NavigationItem
	SocialLinks []sqlite.SocialLink
	SEO         sqlite.SEODefaults
}

type publicHandler struct {
	repository    posts.Repository
	templates     *template.Template
	renderer      *content.Renderer
	publicBaseURL *url.URL
	clock         func() time.Time
	cache         *site.Cache
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
	SiteTitle            string
	SiteAuthor           string
	TitleSuffix          string
	Navigation           []sqlite.NavigationItem
	SocialLinks          []sqlite.SocialLink
	CurrentPage          int
	TotalPages           int
	PreviousURL          string
	NextURL              string
	Status               int
}

func newPublicHandler(repository posts.Repository, publicURL string, clock func() time.Time, cache *site.Cache) (*publicHandler, error) {
	publicBaseURL, err := normalizePublicURL(publicURL)
	if err != nil {
		return nil, err
	}
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
	return &publicHandler{repository: repository, templates: base, renderer: content.NewRenderer(), publicBaseURL: publicBaseURL, clock: clock, cache: cache}, nil
}

func (h *publicHandler) routes(router chi.Router) {
	router.Get("/", h.cached(func(r *http.Request) string { return site.HomeCacheKeyForPage(pageNumber(r)) }, "text/html; charset=utf-8", h.home))
	router.Get("/posts/{slug}", h.cached(func(r *http.Request) string { return site.ArticleCacheKey(chi.URLParam(r, "slug")) }, "text/html; charset=utf-8", h.article))
	router.Get("/categories/{slug}", h.cached(func(r *http.Request) string {
		return site.CategoryCacheKeyForPage(chi.URLParam(r, "slug"), pageNumber(r))
	}, "text/html; charset=utf-8", h.category))
	router.Get("/tags/{slug}", h.cached(func(r *http.Request) string { return site.TagCacheKeyForPage(chi.URLParam(r, "slug"), pageNumber(r)) }, "text/html; charset=utf-8", h.tag))
	router.Get("/archive", h.cached(func(r *http.Request) string { return site.ArchiveCacheKeyForPage(pageNumber(r)) }, "text/html; charset=utf-8", h.archive))
	router.Get("/search", h.search)
	router.Get("/rss.xml", h.cached(func(*http.Request) string { return site.RSSCacheKey() }, "application/xml; charset=utf-8", h.rss))
	router.Get("/sitemap.xml", h.cached(func(*http.Request) string { return site.SitemapCacheKey() }, "application/xml; charset=utf-8", h.sitemap))
}

func (h *publicHandler) cached(key func(*http.Request) string, contentType string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cache == nil {
			next(w, r)
			return
		}
		cacheKey := key(r)
		if value, ok := h.cache.Get(cacheKey); ok {
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(value)
			return
		}
		generation := h.cache.Generation()
		capture := &cachedResponseWriter{header: make(http.Header), status: http.StatusOK}
		next(capture, r)
		for name, values := range capture.header {
			w.Header()[name] = append([]string(nil), values...)
		}
		w.WriteHeader(capture.status)
		_, _ = w.Write(capture.body.Bytes())
		if capture.status == http.StatusOK {
			h.cache.SetIfGeneration(cacheKey, capture.body.Bytes(), generation)
		}
	}
}

type cachedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *cachedResponseWriter) Header() http.Header             { return w.header }
func (w *cachedResponseWriter) WriteHeader(status int)          { w.status = status }
func (w *cachedResponseWriter) Write(value []byte) (int, error) { return w.body.Write(value) }

func (h *publicHandler) home(w http.ResponseWriter, r *http.Request) {
	page, ok := h.validPage(w, r)
	if !ok {
		return
	}
	h.renderListing(w, r, "home.html", pageData{Title: "Articles", Heading: "Latest writing"}, posts.PublishedFilter{Page: page, PageSize: publicPageSize})
}

func (h *publicHandler) category(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page, ok := h.validPage(w, r)
	if !ok {
		return
	}
	h.renderListing(w, r, "list.html", pageData{Title: "Category: " + slug, Heading: "Category: " + slug, Description: "Published articles in this category.", CurrentCategory: slug}, posts.PublishedFilter{CategorySlug: slug, Page: page, PageSize: publicPageSize})
}

func (h *publicHandler) tag(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page, ok := h.validPage(w, r)
	if !ok {
		return
	}
	h.renderListing(w, r, "list.html", pageData{Title: "Tag: " + slug, Heading: "Tag: " + slug, Description: "Published articles with this tag.", CurrentTag: slug}, posts.PublishedFilter{TagSlug: slug, Page: page, PageSize: publicPageSize})
}

func (h *publicHandler) archive(w http.ResponseWriter, r *http.Request) {
	page, ok := h.validPage(w, r)
	if !ok {
		return
	}
	h.renderListing(w, r, "list.html", pageData{Title: "Archive", Heading: "Archive", Description: "All published writing, ordered by publication date."}, posts.PublishedFilter{Page: page, PageSize: publicPageSize})
}

func (h *publicHandler) search(w http.ResponseWriter, r *http.Request) {
	page, ok := h.validPage(w, r)
	if !ok {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	taxonomy, err := h.repository.ListPublishedTaxonomy(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load the site navigation.")
		return
	}
	data := pageData{
		Title: "Search", Heading: "Search", Description: "Search published technical notes.",
		CanonicalURL: h.publicURLForRequest(r, page, false), Robots: "noindex,follow", SearchQuery: query,
		Categories: taxonomy.Categories, Tags: taxonomy.Tags,
	}
	if query != "" {
		items, total, err := h.repository.SearchPublished(r.Context(), query, page, publicPageSize)
		if err != nil {
			h.error(w, r, http.StatusBadRequest, "We could not search for that phrase.")
			return
		}
		data.Posts, data.SearchTotal = items, total
		h.applyPagination(r, &data, page, total, true)
	}
	h.render(w, r, http.StatusOK, "search.html", data)
}

func (h *publicHandler) rss(w http.ResponseWriter, r *http.Request) {
	items, err := h.allPublished(r)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the RSS feed.")
		return
	}
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not prepare the RSS feed.")
		return
	}
	feed, err := site.NewRSSWithIdentity(h.publicBaseURL.String(), h.clock, settings.SiteTitle, settings.Description, settings.Author)
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
	sitemap, err := site.NewSitemap(h.publicBaseURL.String(), h.clock)
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
	taxonomy, err := h.repository.ListPublishedTaxonomy(r.Context())
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load the site navigation.")
		return
	}
	categoryMissing := data.CurrentCategory != "" && !taxonomyContains(taxonomy.Categories, data.CurrentCategory)
	tagMissing := data.CurrentTag != "" && !taxonomyContains(taxonomy.Tags, data.CurrentTag)
	if categoryMissing || tagMissing {
		h.error(w, r, http.StatusNotFound, "This topic does not exist or has no published articles.")
		return
	}
	items, total, err := h.repository.ListPublished(r.Context(), filter)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, "We could not load these articles.")
		return
	}
	data.Posts = items
	data.Categories = taxonomy.Categories
	data.Tags = taxonomy.Tags
	data.CanonicalURL = h.publicURLForRequest(r, filter.Page, false)
	h.applyPagination(r, &data, filter.Page, total, false)
	h.render(w, r, http.StatusOK, templateName, data)
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
		Title: post.Title, Description: post.Summary, CanonicalURL: h.publicURLFor(r.URL.Path), Post: post,
		Body: template.HTML(post.ContentHTML), Headings: rendered.Headings, ReadingMinutes: rendered.ReadingMinutes,
		Categories: taxonomy.Categories, Tags: taxonomy.Tags,
	}
	if post.CoverMediaPath != "" {
		data.OpenGraphImage = h.mediaURL(post.CoverMediaPath)
	}
	h.render(w, r, http.StatusOK, "article.html", data)
}

func (h *publicHandler) error(w http.ResponseWriter, r *http.Request, status int, description string) {
	h.render(w, r, status, "error.html", pageData{Title: "Not found", Heading: "Nothing here", Description: description, CanonicalURL: h.publicURLFor(r.URL.Path), Status: status})
}

func (h *publicHandler) render(w http.ResponseWriter, r *http.Request, status int, pageTemplate string, data pageData) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	data.applySEO(status, settings, h)
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

func pageNumber(r *http.Request) int {
	raw, present := r.URL.Query()["page"]
	if !present {
		return 1
	}
	if len(raw) != 1 {
		return -1
	}
	page, err := strconv.Atoi(raw[0])
	if err != nil || page < 1 || page > 100000 {
		return -1
	}
	return page
}

func (h *publicHandler) validPage(w http.ResponseWriter, r *http.Request) (int, bool) {
	page := pageNumber(r)
	if page < 1 {
		h.error(w, r, http.StatusBadRequest, "The requested page number is invalid.")
		return 0, false
	}
	return page, true
}

func (h *publicHandler) applyPagination(r *http.Request, data *pageData, page, total int, preserveQuery bool) {
	data.CurrentPage = page
	data.TotalPages = (total + publicPageSize - 1) / publicPageSize
	if page > 1 {
		data.PreviousURL = pageLink(r, page-1, preserveQuery)
	}
	if page < data.TotalPages {
		data.NextURL = pageLink(r, page+1, preserveQuery)
	}
}

func pageLink(r *http.Request, page int, preserveQuery bool) string {
	query := make(url.Values)
	if preserveQuery {
		for key, values := range r.URL.Query() {
			if key != "page" {
				query[key] = append([]string(nil), values...)
			}
		}
	}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}
	result := &url.URL{Path: r.URL.Path, RawQuery: query.Encode()}
	return result.String()
}

func (h *publicHandler) publicURLForRequest(r *http.Request, page int, preserveQuery bool) string {
	result, err := url.Parse(h.publicURLFor(r.URL.Path))
	if err != nil {
		return h.publicURLFor(r.URL.Path)
	}
	query := make(url.Values)
	if preserveQuery {
		for key, values := range r.URL.Query() {
			if key != "page" {
				query[key] = append([]string(nil), values...)
			}
		}
	}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}
	result.RawQuery = query.Encode()
	return result.String()
}

func taxonomyContains(values []posts.Taxonomy, slug string) bool {
	for _, value := range values {
		if value.Slug == slug {
			return true
		}
	}
	return false
}

func (h *publicHandler) loadSettings(ctx context.Context) (publicSiteSettings, error) {
	settings := publicSiteSettings{
		SiteTitle:   "Diary Blog",
		Description: "Technical notes, field reports, and durable explanations.",
	}
	repository, ok := h.repository.(publicSettingsRepository)
	if !ok {
		return settings, nil
	}
	stored, err := repository.GetSettings(ctx)
	if err != nil {
		return publicSiteSettings{}, err
	}
	if stored.SiteTitle == "" {
		return settings, nil
	}
	settings.SiteTitle = stored.SiteTitle
	settings.Description = stored.Description
	settings.Author = stored.Author
	settings.Navigation, err = sqlite.ParseNavigation(stored.Navigation)
	if err != nil {
		return publicSiteSettings{}, err
	}
	settings.SocialLinks, err = sqlite.ParseSocialLinks(stored.SocialLinks)
	if err != nil {
		return publicSiteSettings{}, err
	}
	settings.SEO, err = sqlite.ParseSEODefaults(stored.SEODefaults)
	if err != nil {
		return publicSiteSettings{}, err
	}
	return settings, nil
}

func (h *publicHandler) writeXML(w http.ResponseWriter, output []byte) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output)
}

func (data *pageData) applySEO(status int, settings publicSiteSettings, handler *publicHandler) {
	data.SiteTitle = settings.SiteTitle
	data.SiteAuthor = settings.Author
	data.Navigation = settings.Navigation
	data.SocialLinks = settings.SocialLinks
	data.TitleSuffix = settings.SEO.TitleSuffix
	if data.TitleSuffix == "" {
		data.TitleSuffix = settings.SiteTitle
	}
	if data.Title == "" {
		data.Title = settings.SiteTitle
	}
	if data.Description == "" {
		data.Description = settings.SEO.Description
		if data.Description == "" {
			data.Description = settings.Description
		}
	}
	if data.Robots == "" {
		if status >= http.StatusBadRequest {
			data.Robots = "noindex,follow"
		} else if settings.SEO.Robots != "" {
			data.Robots = settings.SEO.Robots
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
	if data.OpenGraphImage == "" && settings.SEO.OpenGraphImage != "" {
		if strings.HasPrefix(settings.SEO.OpenGraphImage, "/") {
			data.OpenGraphImage = handler.publicURLFor(settings.SEO.OpenGraphImage)
		} else {
			data.OpenGraphImage = settings.SEO.OpenGraphImage
		}
	}
}

func normalizePublicURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return nil, fmt.Errorf("invalid public URL %q", value)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid public URL scheme %q", parsed.Scheme)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed, nil
}

func (h *publicHandler) publicURLFor(requestPath string) string {
	result := *h.publicBaseURL
	basePath := strings.TrimSuffix(result.Path, "/")
	suffix := strings.TrimPrefix(requestPath, "/")
	if suffix == "" {
		result.Path = basePath + "/"
	} else {
		result.Path = basePath + "/" + suffix
	}
	result.RawPath = ""
	return result.String()
}

func (h *publicHandler) mediaURL(path string) string {
	return h.publicURLFor("/media/" + strings.TrimPrefix(path, "/"))
}
