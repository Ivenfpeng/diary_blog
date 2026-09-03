# Diary Blog System Design

Date: 2026-09-03
Status: Approved

## 1. Summary

Diary Blog is a single-author technical blog focused on fast reading, reliable publishing, and low-maintenance deployment. It uses a Go monolith for public server-rendered pages and JSON APIs, a Vue 3 single-page application for administration, SQLite for persistence and full-text search, and Markdown as the canonical article format.

The system is deployed with Docker Compose. Caddy terminates HTTPS and proxies to one Blog application container. SQLite, uploaded media, and backups live in a persistent data volume.

## 2. Goals

- Publish technical articles through a browser-based Markdown editor.
- Provide a fast, SEO-friendly public site rendered with Go `html/template`.
- Make categories, tags, archives, and full-text search first-class navigation tools.
- Preserve a lightweight operational model without Redis, message queues, or a separate search service.
- Keep published content available if a new publish attempt fails.
- Support reliable backup and restore of the entire site.
- Deliver a responsive, accessible knowledge-base visual design.

## 3. Non-Goals

- Public registration or multiple authors.
- Reader comments in the first release.
- Email newsletters, subscriptions, or password recovery email.
- Multi-node deployment or horizontal application scaling.
- Real-time collaborative editing.
- Plugin or theme marketplaces.
- A fully client-rendered public website.

## 4. Selected Architecture

### 4.1 Public Site

The public site is rendered on the server with Go `html/template`. Requests return complete HTML for predictable first paint, search-engine indexing, and graceful operation when JavaScript is unavailable.

Public JavaScript is limited to progressive enhancements such as search suggestions, mobile navigation, copy-link actions, and reading preferences. Core navigation and article reading must not depend on JavaScript.

### 4.2 Administration

The administration interface is a Vue 3 + TypeScript application built with Vite. The compiled assets are embedded into the Go binary and served under `/admin`.

The SPA communicates with JSON endpoints under `/api`. It includes login, dashboard, article management, Markdown editing and preview, revision restore, category and tag management, media management, and site settings.

### 4.3 Go Application

One Go process owns:

- Public HTTP routes and template rendering.
- Administration JSON APIs.
- Authentication, authorization, and session management.
- Markdown parsing, sanitization, syntax highlighting, and table-of-contents generation.
- Article publishing and revision management.
- SQLite repositories, migrations, and full-text indexing.
- RSS and sitemap generation.
- In-process public-page caching and invalidation.
- Health checks, structured logging, backup, and restore commands.

The implementation remains a modular monolith. HTTP handlers depend on application services, and application services depend on repository interfaces. SQLite details do not leak into handlers or templates.

### 4.4 Rejected Alternatives

- A fully client-rendered public SPA was rejected because it weakens the default SEO and first-load behavior of a content site.
- Nuxt or Next server rendering was rejected because it adds a second production runtime and deployment surface.
- A file-only Markdown repository was rejected because the selected workflow requires browser-based editing, drafts, revisions, and structured metadata.

## 5. Proposed Repository Structure

```text
diary_blog/
  cmd/blog/                 Application and CLI entry point
  internal/auth/            Passwords, sessions, CSRF, authorization
  internal/content/         Markdown rendering and article rules
  internal/http/            Public handlers, API handlers, middleware
  internal/platform/        Configuration, logging, clock, identifiers
  internal/repository/      Repository interfaces and SQLite implementation
  internal/search/          FTS indexing and query behavior
  internal/site/            RSS, sitemap, settings, public view models
  migrations/               Embedded SQLite migrations
  web/templates/            Go templates and partials
  web/static/               Public CSS, icons, and generated assets
  admin/                    Vue 3 + TypeScript + Vite source
  deploy/                   Caddy and deployment support files
  tests/e2e/                Playwright workflows
  Dockerfile
  compose.yaml
```

Packages should remain small and capability-oriented. Shared abstractions are introduced only when two or more concrete flows need them.

## 6. Product Experience

### 6.1 Visual Direction

The approved direction is a knowledge-base interface with forest green, neutral white, muted gray-green, gold accents, and limited blue or red for status and emphasis. The design is text-led and avoids decorative card-heavy composition.

Public pages use restrained sans-serif interface type and a highly readable article typeface. Typography uses fixed responsive breakpoints rather than viewport-scaled font sizes. Interactive controls use familiar icons with accessible labels and tooltips.

### 6.2 Public Pages

The first release contains:

- Home and article index.
- Article detail.
- Category detail.
- Tag detail.
- Date archive.
- Full-text search results.
- About page.
- RSS feed.
- XML sitemap.
- Health and readiness endpoints.

The desktop home page uses a stable category sidebar and a dense article list. Mobile layouts move taxonomy into a menu and preserve a single-column reading flow.

Article pages contain title, taxonomy, publication metadata, estimated reading time, sanitized Markdown content, syntax-highlighted code, heading anchors, and a sticky table of contents on sufficiently wide screens.

### 6.3 Administration Pages

The administration application contains:

- Login.
- Dashboard with draft and publication summaries.
- Article list with status, category, filters, and search.
- Markdown editor with title, slug, summary, taxonomy, cover, autosave status, preview, save, and publish actions.
- Revision history and restore.
- Category and tag management.
- Media library and upload.
- Site identity, navigation, social links, and SEO defaults.

The editor uses a workbench layout: navigation on the left, writing surface in the center, and publication settings on the right. Narrow screens collapse supporting panels without changing editor state.

## 7. Data Model

### 7.1 `posts`

Stores the canonical Markdown and current rendered representation.

Key fields:

- `id`
- `slug`, unique
- `title`
- `summary`
- `content_md`
- `content_html`
- `content_plain`
- `status`: `draft`, `published`, or `archived`
- `category_id`, nullable
- `cover_media_id`, nullable
- `published_at`, nullable
- `created_at`
- `updated_at`

A published slug does not change automatically when the title changes. Explicit slug changes require confirmation because they can invalidate inbound links.

### 7.2 `post_revisions`

Stores prior article content and metadata for recovery. A revision is created when meaningful content changes are saved. The system retains the latest 30 revisions per article.

### 7.3 Taxonomy

- `categories`: `id`, `name`, `slug`, timestamps.
- `tags`: `id`, `name`, `slug`, timestamps.
- `post_tags`: composite relation between posts and tags.

An article has zero or one category and zero or more tags.

### 7.4 Media

`media` stores path, MIME type, dimensions, size, alternative text, and creation time. File bytes live under the persistent media directory, not in SQLite BLOB columns.

### 7.5 Administration

- `admins`: username, Argon2id password hash, timestamps.
- `sessions`: token hash, administrator ID, expiration, last-seen time, and creation time.

Only one administrator is expected, but the schema does not rely on a hard-coded numeric ID.

### 7.6 Search

`posts_fts` is an SQLite FTS5 virtual table indexing title, summary, and plain text. Only published articles are indexed. Search data is derived and can be fully rebuilt from `posts`.

The first release uses the FTS5 trigram tokenizer so Chinese text and technical substrings can be matched without a separate segmentation service. Search queries are normalized and bounded before reaching SQLite. Integration tests must cover Chinese terms, Latin identifiers, and mixed-language article content. The search service owns tokenization and query construction so the implementation can change without affecting public handlers.

### 7.7 Settings and Migrations

`site_settings` stores validated site identity and presentation settings. `schema_migrations` records applied database migrations. Migration files are embedded into the Go binary and applied during startup before readiness succeeds.

## 8. Publishing Flow

1. The administration app autosaves Markdown and metadata as a draft.
2. The API validates title, slug, taxonomy, content size, and optimistic concurrency metadata.
3. Preview uses the same server-side Markdown renderer as publication.
4. A changed article creates a revision before replacing its current draft.
5. Publishing renders and sanitizes HTML, extracts plain text and headings, updates the published post, and updates its FTS row in one database transaction.
6. After commit, the application invalidates affected article, index, taxonomy, RSS, and sitemap cache entries.
7. If validation, rendering, persistence, or indexing fails, the transaction rolls back and the previously published version remains public.

Autosave requests include the last known `updated_at` or revision value. Conflicts return a stable conflict response so the SPA can preserve local content and offer reload or overwrite choices.

## 9. HTTP Boundary

### 9.1 Public Routes

```text
GET /                              Home and article index
GET /posts/{slug}                  Article detail
GET /categories/{slug}             Category detail
GET /tags/{slug}                   Tag detail
GET /archive                       Date archive
GET /search?q=                     Search results
GET /about                         About page
GET /rss.xml                       RSS feed
GET /sitemap.xml                   XML sitemap
GET /media/{path}                  Uploaded media
GET /healthz                       Process liveness
GET /readyz                        Database and storage readiness
```

### 9.2 Administration API

```text
POST   /api/auth/login
DELETE /api/auth/session
GET    /api/auth/session

/api/admin/posts
/api/admin/posts/{id}/preview
/api/admin/posts/{id}/publish
/api/admin/posts/{id}/revisions
/api/admin/categories
/api/admin/tags
/api/admin/media
/api/admin/settings
```

Exact REST methods and request schemas are defined in the implementation plan and handler tests. API errors use stable machine-readable codes, a request ID, and field-level validation details where applicable.

## 10. Security

- There is no public registration endpoint.
- The administrator is created or reset through an explicit CLI command.
- Passwords use Argon2id with parameters stored alongside the hash.
- Session cookies are `Secure`, `HttpOnly`, and `SameSite`; the default session lifetime is 12 hours.
- Raw session tokens are never stored in SQLite. Only token hashes are persisted.
- State-changing administration requests require CSRF validation and same-origin checks.
- Login attempts are rate-limited by normalized username and client IP.
- Markdown raw HTML is disabled by default. Rendered links and generated HTML are sanitized.
- Uploads accept an explicit image MIME allowlist, verify decoded content, enforce a 10 MB file limit, and use generated storage names.
- Caddy sets HTTPS, compression, request-size limits, and baseline security headers.
- Logs never include passwords, raw cookies, session tokens, or full article bodies.

## 11. Performance and SQLite Behavior

- Enable WAL mode, foreign keys, and a bounded busy timeout.
- Keep write transactions short and avoid performing file I/O inside database transactions.
- Use pagination with a default size of 20 and a maximum of 100.
- Cache rendered public pages and generated feeds in process memory.
- Invalidate cache entries after successful publication or settings changes.
- Do not introduce Redis in the first release.
- Serve immutable fingerprinted static assets with long-lived cache headers.

## 12. Deployment and Configuration

Docker Compose runs two services:

- `caddy`: the only service exposing ports 80 and 443.
- `blog`: the Go application on an internal network.

The Blog image is multi-stage:

1. Node builds the Vue administration assets.
2. Go builds the application with templates, migrations, public assets, and administration assets embedded.
3. The final image contains only runtime certificates, the non-root application user, and the application binary.

The persistent Blog data directory contains:

```text
/data/blog.db
/data/media/
/data/backups/
```

Configuration is supplied through environment variables or a mounted configuration file. Secrets are not built into the image or committed to Git.

Readiness verifies that migrations completed, SQLite can be queried, and required data directories are writable. The application shuts down gracefully and completes in-flight requests within a bounded timeout.

## 13. Backup and Restore

The application provides explicit CLI subcommands:

- `blog backup`: create a consistent SQLite online backup and package it with the media directory and a manifest.
- `blog restore`: validate a backup manifest and restore into an empty or explicitly confirmed data directory.
- `blog migrate`: apply pending schema migrations.
- `blog admin reset-password`: initialize or reset the administrator password.

Backups can be scheduled from the Docker host. A release is not considered operationally complete until a backup can be restored into an empty directory and the restored application passes a smoke test.

## 14. Error Handling and Observability

- Every request receives a request ID.
- Logs are structured and include route, method, status, latency, and classified error type.
- Public errors render accessible HTML pages without internal details.
- API errors preserve stable codes and do not expose SQL or stack traces.
- The SPA retains unsaved editor text when network or validation errors occur.
- Publication errors identify the failed stage and leave the last successful public version unchanged.
- `/healthz` reports process liveness only.
- `/readyz` reports migration, database, and data-directory readiness.

## 15. Testing Strategy

### 15.1 Go Unit Tests

- Slug generation and validation.
- Markdown rendering, sanitization, heading extraction, and reading time.
- Publishing state transitions and cache invalidation.
- Authentication, session expiration, authorization, and CSRF rules.
- API error mapping and validation.

### 15.2 SQLite Integration Tests

- Migrations from an empty database.
- Repository CRUD and transaction rollback.
- FTS synchronization and rebuild.
- Chinese, Latin, and mixed-language trigram search behavior.
- Revision retention and restore.
- Optimistic update conflicts.
- WAL and busy-timeout behavior under representative concurrent access.

### 15.3 HTTP Tests

- Public page status, metadata, canonical URLs, RSS, and sitemap.
- Authentication and administration API contracts.
- Security headers, CSRF rejection, upload validation, and request limits.
- Draft content never appearing in public routes or search.

### 15.4 Vue Tests

- Editor autosave and dirty-state behavior.
- Field validation and server error presentation.
- Conflict handling without loss of local text.
- Publish controls and revision restore.

### 15.5 End-to-End Tests

Playwright covers:

- Login.
- Create draft.
- Upload an image.
- Preview Markdown.
- Publish an article.
- Find the article through public search and taxonomy pages.
- Edit and republish.
- Restore a revision.
- Read core pages at desktop and mobile viewports without overlap.

### 15.6 Operational Tests

- Build the production container image.
- Start the Compose stack and pass liveness/readiness checks.
- Back up a populated site.
- Restore into an empty data directory.
- Verify the restored public article and media.

## 16. First-Release Acceptance Criteria

- An administrator can log in, create, preview, save, publish, edit, archive, and restore an article.
- Public users can browse articles by index, category, tag, archive, and search.
- Public pages render complete HTML and remain usable without client-side JavaScript.
- Draft and archived content never appears in public lists, feeds, sitemap, or search.
- Markdown output is sanitized and code blocks are readable on desktop and mobile.
- The application starts from Docker Compose with persistent SQLite and media data.
- Backup and restore reproduce the database and uploaded media.
- Automated tests cover the critical authoring-to-publication workflow.
- Desktop and mobile screenshots show no incoherent overlap or clipped controls.

## 17. Implementation Constraints

- Prefer established Go and Vue libraries over custom implementations for routing, Markdown parsing, sanitization, editor behavior, and end-to-end testing.
- Keep the final production runtime to Caddy and one Blog application process.
- Do not add infrastructure components without a demonstrated first-release requirement.
- Preserve Markdown as the canonical content source; rendered HTML and search indexes must remain rebuildable.
- Use test-driven development for behavior and migration work.
