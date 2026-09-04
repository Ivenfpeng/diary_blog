# Diary Blog MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/Users/iven/work/program/diary_blog` 中交付一个可通过 Docker Compose 运行的单作者技术博客，包含 Go 服务端渲染公开站点、Vue 管理后台、Markdown 发布、SQLite 全文搜索、备份恢复和端到端测试。

**Architecture:** Go 模块化单体同时提供公开 HTML、管理 JSON API 和 CLI；公开页面使用 `html/template`，管理后台使用 Vue 3 SPA。SQLite 保存内容、会话和 FTS5 trigram 索引，Caddy 负责 HTTPS，所有持久数据进入 `/data` 卷。

**Tech Stack:** Go 1.27、chi v5、modernc SQLite、goldmark、bluemonday、Argon2id、Vue 3、TypeScript、Vite、Vue Router、CodeMirror 6、Vitest、Vue Test Utils、Playwright、Caddy 2、Docker Compose。

---

## Execution Rules

- 在用户指定的现有目录 `/Users/iven/work/program/diary_blog` 中执行，不迁移到其他工作树。
- 开始实现时从当前 `main` 创建 `codex/diary-blog-mvp` 分支。
- 保留当前未提交的 `.gitignore` 和 `.idea/` 内容；提交时只暂存本任务列出的文件。
- 每个行为变更遵循红、绿、重构顺序。
- 每个任务结束前运行该任务测试，并创建独立提交。
- 不在首版引入 Redis、消息队列、外部对象存储或前台 SPA。

## Milestones

1. **Core Foundation:** Task 1-5，得到可迁移数据库、渲染 Markdown、保存并发布文章的 Go 核心。
2. **Public Product:** Task 6-7，得到可浏览、搜索、订阅的服务端渲染公开博客。
3. **Authoring Product:** Task 8-12，得到安全登录、管理 API 和完整 Vue 写作后台。
4. **Operational Release:** Task 13-16，得到健康检查、备份恢复、容器部署和 E2E 验收。

## File Map

```text
cmd/blog/main.go                         进程与 CLI 入口
cmd/blog/main_test.go                    CLI 和启动行为测试
internal/config/config.go                环境配置解析
internal/config/config_test.go           配置默认值和失败行为
internal/database/database.go            SQLite 连接和 pragma
internal/database/database_test.go       连接与 pragma 集成测试
internal/database/migrate.go             嵌入式迁移执行器
internal/database/migrate_test.go        空库和重复迁移测试
internal/content/model.go                文章领域类型
internal/content/render.go               Markdown 渲染
internal/content/render_test.go          安全过滤、目录和阅读时间
internal/posts/repository.go              文章仓储接口
internal/posts/service.go                 草稿和发布服务
internal/posts/service_test.go            发布状态机单元测试
internal/repository/sqlite/posts.go       SQLite 文章仓储
internal/repository/sqlite/posts_test.go  草稿、版本、发布和搜索集成测试
internal/auth/password.go                 Argon2id 密码处理
internal/auth/session.go                  会话服务
internal/auth/auth_test.go                密码和会话测试
internal/web/server.go                    chi 路由装配
internal/web/public.go                    公开页面处理器
internal/web/admin_api.go                 管理 API
internal/web/auth_api.go                  登录与退出 API
internal/web/middleware.go                请求 ID、日志、恢复、CSRF
internal/web/http_test.go                 HTTP 契约测试
internal/operations/backup.go             备份与恢复
internal/operations/backup_test.go        恢复一致性测试
migrations/001_initial.sql                初始数据表
migrations/002_search.sql                 FTS5 trigram 索引
web/assets.go                             模板与静态资源嵌入
web/templates/*.html                      公开页面模板
web/static/site.css                       公开站点样式
web/static/site.js                        渐进增强脚本
web/admin/index.html                      未构建后台时的嵌入占位页
admin/src/*                               Vue 管理后台
admin/src/**/*.test.ts                    Vue 组件和状态测试
tests/e2e/blog.spec.ts                    Playwright 主流程
Dockerfile                                多阶段生产镜像
compose.yaml                              Blog 与 Caddy 服务
deploy/Caddyfile                          HTTPS 和反向代理
Makefile                                  开发、测试和构建命令
.nvmrc                                    Node 24 工具链约束
README.md                                 开发和部署说明
```

### Task 1: Toolchain, Branch, and Build Skeleton

**Files:**
- Create: `.nvmrc`
- Create: `go.mod`
- Create: `Makefile`
- Create: `cmd/blog/main.go`
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Create: `admin/package.json`
- Create: `admin/tsconfig.json`
- Create: `admin/vite.config.ts`
- Create: `admin/src/main.ts`
- Create: `admin/src/App.vue`
- Create: `admin/index.html`
- Create: `admin/.gitignore`

- [ ] **Step 1: Create the feature branch and verify user changes remain untouched**

Run:

```bash
git switch -c codex/diary-blog-mvp
git status --short
```

Expected: branch is `codex/diary-blog-mvp`; existing `.gitignore` and `.idea/` remain visible and unchanged.

- [ ] **Step 2: Install and verify the supported toolchain**

Run:

```bash
brew install go node@24
PATH="$(brew --prefix node@24)/bin:$PATH" go version
PATH="$(brew --prefix node@24)/bin:$PATH" node --version
PATH="$(brew --prefix node@24)/bin:$PATH" npm --version
```

Expected: Go reports `go1.27.x`; Node reports `v24.x`.

- [ ] **Step 3: Write the failing configuration test**

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BLOG_ADDR", "")
	t.Setenv("BLOG_DATA_DIR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.DataDir != "./data" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
```

- [ ] **Step 4: Run the test and confirm the red state**

Run: `go test ./internal/config -run TestLoadDefaults -v`

Expected: FAIL because `Load` is undefined.

- [ ] **Step 5: Create the minimal Go module and configuration implementation**

Run: `go mod init github.com/Ivenfpeng/diary_blog`

Create `internal/config/config.go`:

```go
package config

import "os"

type Config struct {
	Addr       string
	DataDir    string
	PublicURL  string
	CookieName string
}

func Load() (Config, error) {
	return Config{
		Addr:       value("BLOG_ADDR", ":8080"),
		DataDir:    value("BLOG_DATA_DIR", "./data"),
		PublicURL:  value("BLOG_PUBLIC_URL", "http://localhost:8080"),
		CookieName: value("BLOG_COOKIE_NAME", "diary_blog_session"),
	}, nil
}

func value(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

Create `cmd/blog/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/Ivenfpeng/diary_blog/internal/config"
)

func main() {
	if _, err := config.Load(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("diary-blog bootstrap ready")
}
```

- [ ] **Step 6: Create the frontend build skeleton**

Create `.nvmrc` containing `24`. Create `admin/.gitignore` containing:

```text
node_modules/
dist/
coverage/
```

Create `admin/package.json`:

```json
{
  "name": "diary-blog-admin",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "@codemirror/lang-markdown": "latest",
    "@codemirror/state": "latest",
    "@codemirror/view": "latest",
    "codemirror": "latest",
    "lucide-vue-next": "latest",
    "vue": "latest",
    "vue-router": "latest"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "latest",
    "@vue/test-utils": "latest",
    "jsdom": "latest",
    "typescript": "latest",
    "vite": "latest",
    "vitest": "latest",
    "vue-tsc": "latest"
  }
}
```

Create `admin/src/main.ts`:

```ts
import { createApp } from 'vue'
import App from './App.vue'
import './styles.css'

createApp(App).mount('#app')
```

Create `admin/src/App.vue`:

```vue
<template><main>Diary Blog Admin</main></template>
```

Create `admin/index.html`:

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Diary Blog Admin</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

Create `admin/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "types": ["vite/client", "vitest/globals"]
  },
  "include": ["src/**/*.ts", "src/**/*.vue", "vite.config.ts"]
}
```

Create `admin/vite.config.ts`:

```ts
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  test: { environment: 'jsdom' },
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' },
  },
})
```

Create `admin/src/styles.css`:

```css
:root { font-family: Inter, ui-sans-serif, system-ui, sans-serif; color: #18211b; }
* { box-sizing: border-box; }
body { margin: 0; background: #f4f7f4; }
button, input, textarea, select { font: inherit; }
```

- [ ] **Step 7: Add repeatable build commands and verify green state**

Create `Makefile`:

```make
NODE_BIN := $(shell brew --prefix node@24 2>/dev/null)/bin
NODE_ENV := PATH="$(NODE_BIN):$(PATH)"

.PHONY: test-go test-admin test admin-build build dev

test-go:
	go test ./...

test-admin:
	$(NODE_ENV) npm --prefix admin test

test: test-go test-admin

admin-build:
	$(NODE_ENV) npm --prefix admin run build

build: admin-build
	go build -o bin/blog ./cmd/blog

dev:
	go run ./cmd/blog serve
```

Run:

```bash
npm --prefix admin install
go test ./...
npm --prefix admin run build
```

Expected: Go test PASS; Vite creates `admin/dist`.

- [ ] **Step 8: Commit the foundation**

```bash
git add .nvmrc go.mod Makefile cmd internal/config admin
git commit -m "build: bootstrap Go and Vue toolchains"
```

### Task 2: SQLite Connection and Embedded Migrations

**Files:**
- Create: `internal/database/database.go`
- Create: `internal/database/database_test.go`
- Create: `internal/database/migrate.go`
- Create: `internal/database/migrate_test.go`
- Create: `migrations/001_initial.sql`
- Create: `migrations/002_search.sql`

- [ ] **Step 1: Add the SQLite driver and write failing database tests**

Run: `go get modernc.org/sqlite`

Create tests that open a temporary database, call `Open`, and assert:

```go
var journalMode string
if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
	t.Fatal(err)
}
if journalMode != "wal" {
	t.Fatalf("journal_mode=%s", journalMode)
}
```

The migration test must call `Migrate` twice and assert all expected tables exist without duplicate-migration errors.

- [ ] **Step 2: Run tests and confirm they fail**

Run: `go test ./internal/database -v`

Expected: FAIL because `Open` and `Migrate` are undefined.

- [ ] **Step 3: Create the initial schema**

Create `migrations/001_initial.sql`:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE admins (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  csrf_hash BLOB NOT NULL,
  admin_id INTEGER NOT NULL,
  expires_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (admin_id) REFERENCES admins(id) ON DELETE CASCADE
);

CREATE TABLE categories (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  slug TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE tags (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  slug TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE media (
  id INTEGER PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  mime_type TEXT NOT NULL,
  width INTEGER NOT NULL,
  height INTEGER NOT NULL,
  size INTEGER NOT NULL,
  alt_text TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE posts (
  id INTEGER PRIMARY KEY,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  content_md TEXT NOT NULL DEFAULT '',
  content_html TEXT NOT NULL DEFAULT '',
  content_plain TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('draft', 'published', 'archived')),
  category_id INTEGER,
  cover_media_id INTEGER,
  revision INTEGER NOT NULL DEFAULT 1,
  published_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL,
  FOREIGN KEY (cover_media_id) REFERENCES media(id) ON DELETE SET NULL
);

CREATE TABLE post_tags (
  post_id INTEGER NOT NULL,
  tag_id INTEGER NOT NULL,
  PRIMARY KEY (post_id, tag_id),
  FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE post_revisions (
  id INTEGER PRIMARY KEY,
  post_id INTEGER NOT NULL,
  revision INTEGER NOT NULL,
  title TEXT NOT NULL,
  slug TEXT NOT NULL,
  summary TEXT NOT NULL,
  content_md TEXT NOT NULL,
  category_id INTEGER,
  tag_ids_json TEXT NOT NULL DEFAULT '[]',
  created_at TEXT NOT NULL,
  UNIQUE (post_id, revision),
  FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL
);

CREATE TABLE site_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  site_title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  author TEXT NOT NULL DEFAULT '',
  navigation_json TEXT NOT NULL DEFAULT '[]',
  social_links_json TEXT NOT NULL DEFAULT '{}',
  seo_defaults_json TEXT NOT NULL DEFAULT '{}',
  updated_at TEXT NOT NULL
);

CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_posts_status_published_at ON posts(status, published_at DESC);
CREATE INDEX idx_posts_category_id ON posts(category_id);
CREATE INDEX idx_post_revisions_post_id ON post_revisions(post_id, revision DESC);
```

`migrations/002_search.sql` must create:

```sql
CREATE VIRTUAL TABLE posts_fts USING fts5(
  post_id UNINDEXED,
  title,
  summary,
  content_plain,
  tokenize='trigram'
);
```

- [ ] **Step 4: Implement connection and migration behavior**

`Open` must create the data directory, open `blog.db`, set one active writer connection, and apply:

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
```

`Migrate` must embed `migrations/*.sql`, sort them by filename, execute each migration in a transaction, and insert the filename into `schema_migrations` only after successful execution.

- [ ] **Step 5: Verify migrations and commit**

Run:

```bash
go test ./internal/database -v
go test ./...
```

Expected: PASS.

```bash
git add go.mod go.sum internal/database migrations
git commit -m "feat: add SQLite schema and migrations"
```

### Task 3: Markdown Content Renderer

**Files:**
- Create: `internal/content/model.go`
- Create: `internal/content/render.go`
- Create: `internal/content/render_test.go`

- [ ] **Step 1: Add rendering dependencies and failing tests**

Run:

```bash
go get github.com/yuin/goldmark github.com/yuin/goldmark-highlighting/v2 github.com/microcosm-cc/bluemonday github.com/alecthomas/chroma/v2
```

Define tests for headings, fenced code, reading time, plain text, and XSS rejection. The security assertion must include:

```go
result, err := renderer.Render("# Title\n<script>alert(1)</script>\n[j](javascript:alert(1))")
if err != nil {
	t.Fatal(err)
}
if strings.Contains(result.HTML, "<script") || strings.Contains(result.HTML, "javascript:") {
	t.Fatalf("unsafe html: %s", result.HTML)
}
```

- [ ] **Step 2: Run the focused test and confirm failure**

Run: `go test ./internal/content -v`

Expected: FAIL because renderer types are missing.

- [ ] **Step 3: Define stable content types**

Create `internal/content/model.go`:

```go
package content

import "time"

type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusPublished PostStatus = "published"
	StatusArchived  PostStatus = "archived"
)

type Post struct {
	ID           int64
	Slug         string
	Title        string
	Summary      string
	ContentMD    string
	ContentHTML  string
	ContentPlain string
	Status       PostStatus
	CategoryID   *int64
	CoverMediaID *int64
	TagIDs       []int64
	Revision     int64
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PostInput struct {
	Slug         string
	Title        string
	Summary      string
	ContentMD    string
	CategoryID   *int64
	CoverMediaID *int64
	TagIDs       []int64
}

type PostRevision struct {
	ID         int64
	PostID     int64
	Revision   int64
	Title      string
	Slug       string
	Summary    string
	ContentMD  string
	CategoryID *int64
	TagIDs     []int64
	CreatedAt  time.Time
}

type Heading struct {
	Level int
	ID    string
	Text  string
}

type RenderedContent struct {
	HTML           string
	PlainText      string
	Headings       []Heading
	ReadingMinutes int
}
```

- [ ] **Step 4: Implement the renderer**

`Renderer.Render(markdown string)` must:

1. Reject input over 2 MiB.
2. Use goldmark with GFM features and raw HTML disabled.
3. Generate deterministic heading IDs and a heading list.
4. Highlight fenced code through `goldmark-highlighting/v2`, backed by Chroma.
5. Sanitize the result with a bluemonday policy that permits generated code classes and heading IDs.
6. Produce plain text without HTML tags.
7. Compute reading minutes as `max(1, ceil(runeCount/500))`.

- [ ] **Step 5: Verify renderer behavior and commit**

Run: `go test ./internal/content -v`

Expected: all renderer tests PASS.

```bash
git add go.mod go.sum internal/content
git commit -m "feat: render safe Markdown content"
```

### Task 4: Draft Repository and Revision History

**Files:**
- Create: `internal/posts/repository.go`
- Create: `internal/repository/sqlite/posts.go`
- Create: `internal/repository/sqlite/posts_test.go`

- [x] **Step 1: Define the repository contract**

Create `internal/posts/repository.go`:

```go
package posts

import (
	"context"
	"errors"
	"time"

	"github.com/Ivenfpeng/diary_blog/internal/content"
)

var ErrNotFound = errors.New("post not found")
var ErrConflict = errors.New("post update conflict")

type Repository interface {
	CreateDraft(context.Context, content.PostInput, time.Time) (content.Post, error)
	GetByID(context.Context, int64) (content.Post, error)
	SaveDraft(context.Context, int64, content.PostInput, int64, time.Time) (content.Post, error)
	ListRevisions(context.Context, int64) ([]content.PostRevision, error)
	RestoreRevision(context.Context, int64, int64, int64, time.Time) (content.Post, error)
}
```

- [x] **Step 2: Write failing repository integration tests**

Use a migrated temporary SQLite database. Test create/get, unique slug rejection, optimistic conflict, revision creation, 30-version retention, tag replacement, and revision restore.

The conflict test must call `SaveDraft` twice with the same revision number and assert `errors.Is(err, posts.ErrConflict)` on the second call.

- [x] **Step 3: Run tests and confirm failure**

Run: `go test ./internal/repository/sqlite -run 'TestPostRepository' -v`

Expected: FAIL because the SQLite repository is missing.

- [x] **Step 4: Implement draft transactions**

Implement `CreateDraft`, `GetByID`, `SaveDraft`, `ListRevisions`, and `RestoreRevision`. Every update must:

1. Begin a transaction.
2. Compare the stored revision number with the expected revision.
3. Insert the previous content into `post_revisions` when meaningful fields changed.
4. Update taxonomy relationships.
5. Increment the revision number.
6. Delete revisions older than the newest 30 for that post.
7. Commit and return the complete post.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/repository/sqlite -v
go test ./...
```

Expected: PASS.

```bash
git add internal/posts internal/repository/sqlite migrations
git commit -m "feat: persist drafts and article revisions"
```

### Task 5: Publishing Service and FTS5 Search

**Files:**
- Create: `internal/posts/service.go`
- Create: `internal/posts/service_test.go`
- Modify: `internal/posts/repository.go`
- Modify: `internal/repository/sqlite/posts.go`
- Modify: `internal/repository/sqlite/posts_test.go`

- [x] **Step 1: Extend the repository contract**

Add methods:

```go
Publish(context.Context, int64, content.RenderedContent, int64, time.Time) (content.Post, error)
Archive(context.Context, int64, int64, time.Time) (content.Post, error)
GetPublishedBySlug(context.Context, string) (content.Post, error)
ListPublished(context.Context, PublishedFilter) ([]content.Post, int, error)
SearchPublished(context.Context, string, int, int) ([]content.Post, int, error)
RebuildSearch(context.Context) error
ListAdmin(context.Context, AdminFilter) ([]content.Post, int, error)
```

Define the filter exactly as:

```go
type PublishedFilter struct {
	CategorySlug string
	TagSlug      string
	Year         int
	Page         int
	PageSize     int
}

type AdminFilter struct {
	Status   content.PostStatus
	Query    string
	Page     int
	PageSize int
}
```

- [x] **Step 2: Write failing service and integration tests**

Tests must prove:

- Publishing validates title and slug.
- Publishing stores rendered HTML and indexes only published posts.
- A render or index failure leaves the previous published row unchanged.
- Archiving removes the FTS row.
- Chinese query `数据库`, identifier query `chi.Router`, and mixed query return expected articles.
- Search query length is limited to 100 runes.

- [x] **Step 3: Implement the service**

Create a service with this dependency boundary:

```go
type Service struct {
	repo     Repository
	renderer *content.Renderer
	clock    func() time.Time
}

func (s *Service) Preview(markdown string) (content.RenderedContent, error)
func (s *Service) CreateDraft(ctx context.Context, input content.PostInput) (content.Post, error)
func (s *Service) Get(ctx context.Context, id int64) (content.Post, error)
func (s *Service) SaveDraft(ctx context.Context, id int64, input content.PostInput, expectedRevision int64) (content.Post, error)
func (s *Service) ListAdmin(ctx context.Context, filter AdminFilter) ([]content.Post, int, error)
func (s *Service) ListRevisions(ctx context.Context, id int64) ([]content.PostRevision, error)
func (s *Service) RestoreRevision(ctx context.Context, id int64, revisionID int64, expectedRevision int64) (content.Post, error)
func (s *Service) Publish(ctx context.Context, id int64, expectedRevision int64) (content.Post, error)
func (s *Service) Archive(ctx context.Context, id int64, expectedRevision int64) (content.Post, error)
```

All service methods validate pagination, identifiers, title, slug, content size, and optimistic revision values before calling the repository. `Publish` must load the draft, validate required fields, render outside the transaction, and call repository `Publish` for the atomic post/FTS update.

- [x] **Step 4: Implement FTS transactions**

Use parameterized `MATCH` queries. Normalize whitespace, reject empty queries, quote user tokens instead of concatenating operators, and rank with `bm25(posts_fts)`. Publishing and archiving must update `posts` and `posts_fts` in the same transaction.

- [x] **Step 5: Verify and commit**

Run:

```bash
go test ./internal/posts ./internal/repository/sqlite -v
go test ./...
```

Expected: PASS.

```bash
git add internal/posts internal/repository/sqlite
git commit -m "feat: publish and search articles"
```

### Task 6: Public HTTP Server and Knowledge-Base UI

**Files:**
- Create: `web/assets.go`
- Create: `web/admin/index.html`
- Create: `web/templates/base.html`
- Create: `web/templates/home.html`
- Create: `web/templates/article.html`
- Create: `web/templates/list.html`
- Create: `web/templates/error.html`
- Create: `web/static/site.css`
- Create: `web/static/site.js`
- Create: `internal/web/server.go`
- Create: `internal/web/public.go`
- Create: `internal/web/http_test.go`

- [x] **Step 1: Write failing public route tests**

Use `httptest.NewServer`. Assert `/`, `/posts/{slug}`, `/categories/{slug}`, `/tags/{slug}`, and `/archive` return full HTML. The article assertion must include canonical URL, title, sanitized body, reading time, and table-of-contents heading link.

- [x] **Step 2: Run the route tests and confirm failure**

Run: `go test ./internal/web -run 'TestPublic' -v`

Expected: FAIL because server construction is missing.

- [x] **Step 3: Embed assets and assemble chi routes**

Create `web/assets.go`:

```go
package webassets

import "embed"

//go:embed templates static admin
var Assets embed.FS
```

Create `NewServer` that registers request ID, panic recovery, access logging, static assets, media, public routes, `/healthz`, and `/readyz`.

- [x] **Step 4: Build the approved public layout**

Templates must implement the approved knowledge-base structure: stable header, desktop taxonomy sidebar, dense article rows, mobile taxonomy menu, article metadata, readable prose width, code blocks, and sticky desktop table of contents.

`site.css` must define reusable tokens for forest, neutral, gold, blue, red, spacing, focus rings, content width, breakpoints, icon-button dimensions, and reduced-motion behavior. Cards must not wrap page sections.

- [x] **Step 5: Verify public behavior and visual constraints**

Run:

```bash
go test ./internal/web -run 'TestPublic' -v
go test ./...
```

Expected: PASS; HTML tests find no draft content.

- [x] **Step 6: Commit**

```bash
git add web internal/web
git commit -m "feat: add server-rendered public blog"
```

### Task 7: Search, RSS, Sitemap, and SEO Metadata

**Files:**
- Create: `internal/site/feed.go`
- Create: `internal/site/feed_test.go`
- Create: `internal/site/sitemap.go`
- Create: `internal/site/sitemap_test.go`
- Create: `web/templates/search.html`
- Modify: `internal/web/public.go`
- Modify: `internal/web/http_test.go`

- [x] **Step 1: Write failing feed and search tests**

Assert RSS contains only published posts with absolute canonical URLs, sitemap excludes drafts and archives, search escapes snippets, and an empty search renders instructions without running FTS.

- [x] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/site ./internal/web -run 'TestRSS|TestSitemap|TestSearch' -v`

Expected: FAIL.

- [x] **Step 3: Implement deterministic generators**

Use `encoding/xml` for RSS and sitemap. Inject public base URL and clock. Sort output by publication time and then slug. Limit RSS to the newest 20 posts and sitemap to published public routes.

- [x] **Step 4: Add SEO behavior**

Every public page view model must provide title, description, canonical URL, robots value, and Open Graph fields. Search result pages use `noindex,follow`. Article pages use summary as description and cover media when present.

- [x] **Step 5: Verify and commit**

Run: `go test ./internal/site ./internal/web -v`

Expected: PASS.

```bash
git add internal/site internal/web web/templates/search.html
git commit -m "feat: add search feeds and SEO metadata"
```

### Task 8: Administrator Passwords, Sessions, and CSRF

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/session.go`
- Create: `internal/auth/auth_test.go`
- Create: `internal/repository/sqlite/auth.go`
- Create: `internal/repository/sqlite/auth_test.go`
- Create: `internal/web/auth_api.go`
- Modify: `internal/web/middleware.go`
- Modify: `internal/web/server.go`

- [x] **Step 1: Add Argon2 and write failing auth tests**

Run: `go get golang.org/x/crypto/argon2`

Tests must verify correct and incorrect passwords, malformed hashes, 12-hour absolute session expiry, token hashing, logout deletion, Secure/HttpOnly/SameSite cookies, CSRF rejection, and login throttling after five failures in fifteen minutes.

- [x] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/auth ./internal/repository/sqlite ./internal/web -run 'TestPassword|TestSession|TestLogin|TestCSRF' -v`

Expected: FAIL.

- [x] **Step 3: Implement password and session primitives**

Use Argon2id with a random 16-byte salt, 64 MiB memory, three iterations, two threads, and a 32-byte key. Encode the parameters with the hash, reject values above those configured bounds during parsing, and compare in constant time. Generate independent 32-byte random session and CSRF tokens, send base64url values to the client, and store only SHA-256 token hashes in SQLite.

- [x] **Step 4: Implement HTTP auth protections**

The login response sets the session cookie as `Secure`, `HttpOnly`, and `SameSite=Strict`; it sets the CSRF cookie as `Secure` and `SameSite=Strict` but readable by the SPA. State-changing `/api/admin/*` requests must require the session, same-origin validation, matching CSRF header/cookie values, and a CSRF hash matching the active session row. Login throttling is in process and keyed by normalized username plus client IP.

- [x] **Step 5: Verify and commit**

Run: `go test ./internal/auth ./internal/repository/sqlite ./internal/web -v`

Expected: PASS.

```bash
git add go.mod go.sum internal/auth internal/repository/sqlite/auth* internal/web
git commit -m "feat: secure administrator sessions"
```

### Task 9: Administration Post API

**Files:**
- Create: `internal/web/admin_api.go`
- Create: `internal/web/errors.go`
- Create: `internal/web/admin_api_test.go`
- Modify: `internal/web/server.go`

- [ ] **Step 1: Write failing API contract tests**

Cover create, get, list, autosave, preview, publish, archive, revisions, restore, unauthenticated access, validation errors, not-found errors, and `409 conflict`. Assert the stable error shape:

```json
{
  "error": {
    "code": "post_conflict",
    "message": "The article changed on the server.",
    "fields": {},
    "request_id": "..."
  }
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `go test ./internal/web -run 'TestAdminPostAPI' -v`

Expected: FAIL.

- [ ] **Step 3: Implement request and response models**

Use strict JSON decoding with unknown fields rejected and a 2 MiB body limit. Return `201` for draft creation, `200` for save/preview/publish, `204` for delete-like session operations, `400` for malformed input, `401` for missing session, `403` for CSRF, `404` for missing resources, and `409` for revision conflicts.

- [ ] **Step 4: Register routes**

Register explicit methods for `/api/admin/posts`, `/api/admin/posts/{id}`, `/preview`, `/publish`, `/archive`, `/revisions`, and `/revisions/{revisionID}/restore`. Do not expose a generic catch-all CRUD handler.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/web -run 'TestAdminPostAPI' -v && go test ./...`

Expected: PASS.

```bash
git add internal/web
git commit -m "feat: expose article administration API"
```

### Task 10: Vue Administration Shell and Login

**Files:**
- Create: `admin/src/router.ts`
- Create: `admin/src/api/client.ts`
- Create: `admin/src/api/types.ts`
- Create: `admin/src/state/session.ts`
- Create: `admin/src/layouts/AdminLayout.vue`
- Create: `admin/src/views/LoginView.vue`
- Create: `admin/src/views/DashboardView.vue`
- Create: `admin/src/components/IconButton.vue`
- Create: `admin/src/styles.css`
- Test: `admin/src/views/LoginView.test.ts`
- Modify: `admin/src/main.ts`
- Modify: `admin/src/App.vue`

- [ ] **Step 1: Write the failing login component test**

Mount `LoginView`, submit username/password, mock `POST /api/auth/login`, and assert navigation to `/admin`. Add a failed-login test that keeps the username, clears the password, focuses the password input, and renders the server message.

- [ ] **Step 2: Run and confirm failure**

Run: `npm --prefix admin test -- LoginView`

Expected: FAIL because the view and session state do not exist.

- [ ] **Step 3: Implement typed API and router guards**

`api/client.ts` must send JSON, include credentials, attach `X-CSRF-Token` for writes, parse the stable error shape, and throw `ApiError`. Router guards call `GET /api/auth/session` once and redirect unauthenticated users to `/admin/login`.

- [ ] **Step 4: Build the workbench shell**

Create a quiet operational layout with fixed navigation width, compact top bar, stable 36px icon buttons using Lucide icons, clear focus rings, status colors, and mobile drawer behavior. Do not use nested cards or decorative dashboard panels.

- [ ] **Step 5: Verify and commit**

Run:

```bash
npm --prefix admin test
npm --prefix admin run build
```

Expected: PASS and production assets build.

```bash
git add admin
git commit -m "feat: add administration login and shell"
```

### Task 11: Article List and Markdown Editor

**Files:**
- Create: `admin/src/views/PostsView.vue`
- Create: `admin/src/views/PostEditorView.vue`
- Create: `admin/src/components/MarkdownEditor.vue`
- Create: `admin/src/components/PublishPanel.vue`
- Create: `admin/src/components/RevisionPanel.vue`
- Create: `admin/src/state/editor.ts`
- Test: `admin/src/state/editor.test.ts`
- Test: `admin/src/views/PostEditorView.test.ts`
- Modify: `admin/src/router.ts`

- [ ] **Step 1: Write failing editor state tests**

Use fake timers. Assert changes trigger autosave after 1.5 seconds, only one save runs at a time, a newer change queues one additional save, successful save updates revision, and `409` preserves local Markdown while setting conflict state.

- [ ] **Step 2: Run and confirm failure**

Run: `npm --prefix admin test -- editor PostEditorView`

Expected: FAIL.

- [ ] **Step 3: Implement CodeMirror and editor state**

`MarkdownEditor.vue` owns one CodeMirror 6 editor instance, emits string updates, supports Markdown syntax, line wrapping, keyboard focus, and cleanup on unmount. `editor.ts` owns article fields, dirty state, save status, revision, conflict state, preview HTML, and serialized autosave.

- [ ] **Step 4: Implement article workflows**

The list supports status filter, text search, pagination, create, and edit. The editor provides title, slug, summary, category, tags, Markdown, preview, save, publish, archive, and revision restore. Publish is disabled while validation fails or a save is running.

- [ ] **Step 5: Verify and commit**

Run:

```bash
npm --prefix admin test
npm --prefix admin run build
```

Expected: PASS.

```bash
git add admin/src
git commit -m "feat: add Markdown authoring workflow"
```

### Task 12: Taxonomy, Media, and Site Settings

**Files:**
- Create: `internal/repository/sqlite/taxonomy.go`
- Create: `internal/repository/sqlite/media.go`
- Create: `internal/repository/sqlite/settings.go`
- Create: `internal/repository/sqlite/management_test.go`
- Modify: `internal/web/admin_api.go`
- Create: `admin/src/views/TaxonomyView.vue`
- Create: `admin/src/views/MediaView.vue`
- Create: `admin/src/views/SettingsView.vue`
- Test: `admin/src/views/MediaView.test.ts`

- [ ] **Step 1: Write failing backend and frontend tests**

Backend tests cover unique slugs, referenced-category deletion rejection, media metadata, image type/size rejection, settings validation, and public settings cache invalidation. Frontend tests cover upload progress, alt-text requirement, and failed upload recovery.

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
go test ./internal/repository/sqlite ./internal/web -run 'TestTaxonomy|TestMedia|TestSettings' -v
npm --prefix admin test -- MediaView
```

Expected: FAIL.

- [ ] **Step 3: Implement safe media storage**

Accept JPEG, PNG, WebP, GIF, and AVIF. Limit request and decoded file size to 10 MiB, decode image metadata before acceptance, generate a random storage name, write to a temporary file, fsync, and rename into `/data/media/YYYY/MM`. Never use the original filename as a filesystem path.

- [ ] **Step 4: Implement management APIs and views**

Expose explicit taxonomy, media, and settings endpoints with authentication and CSRF. Build dense table/list views with edit dialogs, upload controls, image thumbnails, alt text, and validated site identity fields.

- [ ] **Step 5: Verify and commit**

Run:

```bash
go test ./...
npm --prefix admin test
npm --prefix admin run build
```

Expected: PASS.

```bash
git add internal/repository/sqlite internal/web admin/src
git commit -m "feat: manage taxonomy media and settings"
```

### Task 13: Logging, Health Checks, Graceful Shutdown, and Cache

**Files:**
- Create: `internal/platform/logging.go`
- Create: `internal/site/cache.go`
- Create: `internal/site/cache_test.go`
- Modify: `internal/web/middleware.go`
- Modify: `internal/web/server.go`
- Modify: `cmd/blog/main.go`
- Test: `cmd/blog/main_test.go`

- [ ] **Step 1: Write failing operational tests**

Assert panic recovery returns a request ID without stack details, access logs omit cookies, `/readyz` fails when the data directory is not writable, cache invalidation removes article and dependent list keys, and server shutdown respects a ten-second timeout.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/site ./internal/web ./cmd/blog -run 'TestCache|TestReady|TestRecovery|TestShutdown' -v`

Expected: FAIL.

- [ ] **Step 3: Implement operational behavior**

Use `log/slog` JSON output. Middleware logs request ID, method, route pattern, status, bytes, and duration. Cache entries carry explicit keys and expirations; publication invalidates the article, home, archive, category, tag, RSS, and sitemap keys.

- [ ] **Step 4: Wire graceful process lifecycle**

`serve` opens the database, runs migrations, constructs services, starts `http.Server`, listens for `SIGINT`/`SIGTERM`, and calls `Shutdown` with a ten-second context. Readiness is false until migration and storage checks pass.

- [ ] **Step 5: Verify and commit**

Run: `go test ./...`

Expected: PASS.

```bash
git add internal/platform internal/site internal/web cmd/blog
git commit -m "feat: add operational health and caching"
```

### Task 14: Backup, Restore, Migration, and Admin CLI

**Files:**
- Create: `internal/operations/backup.go`
- Create: `internal/operations/backup_test.go`
- Create: `cmd/blog/commands.go`
- Modify: `cmd/blog/main.go`
- Modify: `cmd/blog/main_test.go`

- [ ] **Step 1: Write failing CLI and restore tests**

Create a populated temporary site, run backup, restore into an empty directory, reopen SQLite, and verify article, FTS search, media bytes, and manifest checksum. Test refusal to overwrite a non-empty destination without `--force`.

- [ ] **Step 2: Run and confirm failure**

Run: `go test ./internal/operations ./cmd/blog -run 'TestBackup|TestRestore|TestAdminCommand' -v`

Expected: FAIL.

- [ ] **Step 3: Implement consistent backup format**

Create a `.tar.gz` containing `manifest.json`, `blog.db`, and `media/`. Use SQLite online backup into a temporary database, calculate SHA-256 for every archive entry, write the manifest last, fsync the completed archive, and remove temporary files on failure.

- [ ] **Step 4: Implement CLI subcommands**

Support:

```text
blog serve
blog migrate
blog backup --output PATH
blog restore --input PATH [--force]
blog admin reset-password --username NAME
blog search rebuild
```

Commands return errors to `main`, print operational output to stdout, and never print passwords or session tokens.

- [ ] **Step 5: Verify and commit**

Run: `go test ./internal/operations ./cmd/blog -v && go test ./...`

Expected: PASS.

```bash
git add internal/operations cmd/blog
git commit -m "feat: add backup restore and administration CLI"
```

### Task 15: Production Build and Docker Compose

**Files:**
- Create: `Dockerfile`
- Create: `compose.yaml`
- Create: `deploy/Caddyfile`
- Create: `.dockerignore`
- Modify: `Makefile`
- Modify: `README.md`

- [ ] **Step 1: Install Docker Desktop and verify Compose**

Run:

```bash
brew install --cask docker-desktop
open -a Docker
docker version
docker compose version
```

Expected: daemon and Compose respond successfully.

- [ ] **Step 2: Write the production Dockerfile**

Use `node:24-alpine` to install from `admin/package-lock.json` and build the SPA. Copy `admin/dist` into `web/admin`. Use `golang:1.27-alpine` to run tests and build a static `blog` binary. Use a non-root final image with CA certificates and `/data` as the writable volume.

- [ ] **Step 3: Create the Compose topology**

`compose.yaml` must define `blog` and `caddy`, an internal application network, `blog_data` and `caddy_data` volumes, a Blog healthcheck against `/healthz`, and no published Blog application port. Caddy alone publishes 80/443.

`deploy/Caddyfile` must enable compression, reverse proxy, security headers, and a 10 MiB request body limit for upload routes.

- [ ] **Step 4: Add runtime documentation**

README must include local prerequisites, Node PATH command, initial admin creation, local run, tests, Compose deployment, required environment variables, backup, restore, and upgrade sequence.

- [ ] **Step 5: Build and verify containers**

Run:

```bash
docker compose build
docker compose up -d
docker compose ps
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
```

Expected: both services healthy; endpoints return success.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile compose.yaml deploy .dockerignore Makefile README.md
git commit -m "build: add production container deployment"
```

### Task 16: End-to-End Workflow, Responsive Verification, and Release Gate

**Files:**
- Create: `package.json`
- Create: `playwright.config.ts`
- Create: `tests/e2e/blog.spec.ts`
- Create: `tests/e2e/visual.spec.ts`
- Create: `tests/e2e/fixtures/article.md`
- Modify: `Makefile`
- Modify: `README.md`

- [ ] **Step 1: Install Playwright and write failing E2E tests**

Run:

```bash
npm init -y
npm install --save-dev @playwright/test@latest
npm pkg set scripts.test:e2e="playwright test"
npx playwright install chromium
```

The primary test must log in, create a draft, upload an image, preview, publish, find the article through search and category, edit and republish, restore a revision, and verify draft/archived content is absent publicly.

- [ ] **Step 2: Add responsive visual assertions**

Test viewports `1440x1000`, `1024x768`, `390x844`, and `360x800`. Assert navigation, editor toolbar, title, article body, code blocks, and table of contents do not overlap or exceed the viewport. Capture screenshots for home, article, admin list, and editor.

- [ ] **Step 3: Run the tests and fix product defects through TDD**

Run:

```bash
npm run test:e2e
```

Expected before fixes: at least one workflow or visual assertion fails. Fix the underlying Go, Vue, template, or CSS behavior, then rerun until PASS. Do not weaken assertions to hide clipping or overlap.

- [ ] **Step 4: Run the complete release gate**

Run:

```bash
go test ./...
npm --prefix admin test
npm --prefix admin run build
npm run test:e2e
docker compose build
docker compose up -d
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
docker compose exec -T blog /app/blog backup --output /data/backups/release-smoke.tar.gz
```

Expected: every command succeeds and the backup archive exists.

- [ ] **Step 5: Perform restore smoke test**

Restore the release backup into a fresh temporary volume, start the Blog container against that volume, and verify the published article and uploaded media through HTTP.

- [ ] **Step 6: Commit final verification assets**

```bash
git add package.json package-lock.json playwright.config.ts tests Makefile README.md admin internal web cmd
git commit -m "test: verify complete blog publishing workflow"
```

## Spec Coverage Matrix

| Approved design requirement | Implemented by |
|---|---|
| Go modular monolith and configuration | Tasks 1, 13 |
| SQLite schema, migrations, WAL, FTS5 trigram | Tasks 2, 4, 5 |
| Markdown source, sanitization, headings, code highlighting | Task 3 |
| Drafts, optimistic autosave, revisions, publish rollback | Tasks 4, 5, 9, 11 |
| Server-rendered public pages and approved visual direction | Tasks 6, 7 |
| Category, tag, archive, search, RSS, sitemap, SEO | Tasks 5, 6, 7, 12 |
| Single administrator, Argon2id, sessions, CSRF, rate limiting | Tasks 8, 9 |
| Vue 3 administration workbench and Markdown editor | Tasks 10, 11, 12 |
| Safe image upload and local media persistence | Task 12 |
| In-process cache, structured logs, health and readiness | Task 13 |
| CLI migration, password reset, backup and restore | Task 14 |
| Caddy, Docker Compose, persistent data volume | Task 15 |
| Desktop/mobile verification and complete authoring workflow | Task 16 |
| No comments, public registration, Redis, queues, or public SPA | Enforced across Tasks 6-16 and final review |

## Final Verification Checklist

- [ ] `git status --short` contains only known user-owned changes.
- [ ] `go test ./...` passes.
- [ ] `npm --prefix admin test` passes.
- [ ] `npm --prefix admin run build` passes.
- [ ] `npm run test:e2e` passes at all required viewports.
- [ ] `docker compose build` succeeds from a clean checkout.
- [ ] `/healthz` and `/readyz` pass in Compose.
- [ ] Backup and restore reproduce article, search index, and media.
- [ ] Public pages work without JavaScript.
- [ ] Draft and archived articles are absent from HTML, RSS, sitemap, and search.
- [ ] No password, cookie, raw session token, or full article body appears in logs.
- [ ] Desktop and mobile screenshots have no clipped text, overlapping controls, or blank media.

## Official References

- Go releases: https://go.dev/doc/devel/release
- chi router: https://github.com/go-chi/chi
- SQLite FTS5: https://sqlite.org/fts5.html
- SQLite backup API: https://sqlite.org/backup.html
- goldmark: https://github.com/yuin/goldmark
- goldmark highlighting: https://github.com/yuin/goldmark-highlighting
- bluemonday: https://github.com/microcosm-cc/bluemonday
- Vue quick start: https://vuejs.org/guide/quick-start.html
- Vite guide: https://vite.dev/guide/
- Vue Router: https://router.vuejs.org/
- CodeMirror: https://codemirror.net/docs/
- Vitest: https://vitest.dev/guide/
- Playwright: https://playwright.dev/docs/intro
- Caddy automatic HTTPS: https://caddyserver.com/docs/automatic-https
