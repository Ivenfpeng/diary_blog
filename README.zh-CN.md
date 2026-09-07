# diary_blog

[English](README.md)

`diary_blog` 是一个单二进制 Go 技术博客系统，包含服务端渲染公开站点、Vue 管理后台、SQLite 内容存储、全文搜索、备份恢复，以及支持本地 HTTP、自动 HTTPS 和自带 TLS 证书的 Compose 部署方案。

## 功能概览

- Go 单体服务：同时提供公开 HTML、管理 API 和命令行工具。
- 公开站点：使用 Go `html/template` 服务端渲染，支持首页、文章页、分类、标签、归档、搜索、RSS 和 Sitemap。
- 管理后台：Vue 3 + TypeScript 写作工作台，支持 Markdown 编辑、预览、发布、归档、版本恢复、分类标签、媒体库和站点设置。
- 内容存储：SQLite，包含迁移、WAL、FTS5 trigram 全文搜索、发布快照和历史版本。
- 安全能力：单管理员、Argon2id 密码、哈希会话、CSRF 双提交校验、登录限流和可信反向代理配置。
- 运维能力：结构化日志、健康检查、就绪检查、公开页缓存、优雅停机、备份、恢复、迁移和管理员密码重置。
- 部署能力：Docker 多阶段构建、Docker Compose、本地/内网 HTTP-only、Caddy 自动 HTTPS、自带 TLS 证书、持久化数据卷和恢复演练命令。
- 发布门禁：Go 测试、Vue 测试、Vite 构建、Playwright E2E、响应式布局检查和容器门禁 dry-run。

## 本地开发

依赖：

- Go 1.27 或更新版本
- Node.js 24 和 npm（项目会优先使用 Homebrew 的 `node@24`）
- Docker Desktop 与 Docker Compose v2（仅容器部署和容器门禁需要）

如果 Homebrew 的 Node 24 不在当前 shell 路径中，可以先执行：

```sh
export PATH="$(brew --prefix node@24)/bin:$PATH"
```

安装管理后台依赖后，使用项目构建目标。该目标会构建 Vue 管理后台、同步到 `web/admin`，并把静态文件嵌入 Go 二进制：

```sh
npm --prefix admin ci
make build
```

创建第一个管理员，或重置已有管理员密码：

```sh
printf '%s\n' 'choose-a-long-password' | ./bin/blog admin reset-password --username admin
```

本地启动服务时使用默认数据目录 `./data` 和默认公开地址 `http://localhost:8080`：

```sh
make dev
```

打开 <http://localhost:8080> 访问公开站点，打开 <http://localhost:8080/admin> 登录管理后台。若使用其他本地域名或端口测试链接与 Cookie，请设置 `BLOG_PUBLIC_URL`。

## 测试与检查

常用检查：

```sh
make test
make vet
make build
```

`make test-go` 和 `make vet` 默认把 Go 构建缓存放在 `/tmp/diary-blog-go-cache`。如需调整，可使用 `GO_CACHE=/path` 覆盖。

### 端到端发布门禁

首次运行浏览器端测试前，安装根目录测试依赖。默认本地门禁使用已安装的 Google Chrome channel，因为当前开发宿主机已有该浏览器：

```sh
npm ci
npm run test:e2e
```

如果干净机器没有 Chrome，可以安装 Playwright bundled Chromium，并在运行时设置 `PLAYWRIGHT_BUNDLED_CHROMIUM=1`：

```sh
npx playwright install chromium
PLAYWRIGHT_BUNDLED_CHROMIUM=1 npm run test:e2e
```

`npm run test:e2e` 会重新构建并嵌入管理后台，然后启动一个使用临时数据目录的隔离 Blog 服务。它会验证登录、写作、媒体上传、预览、发布、发现、版本恢复，以及草稿和归档文章不会出现在公开页面、搜索、RSS 和 Sitemap 中。它还会为 1440×1000、1024×768、390×844、360×800 四组视口生成首页、文章页、管理列表页和编辑器截图，输出到 `/tmp/diary-blog-e2e-results`。

运行非容器发布门禁：

```sh
make release-gate
git diff --check
```

生产运行时门禁需要先保留或创建一篇已发布的 smoke 文章，并确保文章中包含上传过的媒体图片，然后执行：

```sh
make container-release-gate \
  BACKUP=/data/backups/release-smoke.tar.gz \
  SMOKE_ARTICLE_PATH=/posts/release-gate-publishing-workflow \
  SMOKE_SEARCH_QUERY=durable
```

该命令会构建 Compose 镜像、启动默认 HTTP-only 生产拓扑、通过反向代理检查 `/healthz` 和 `/readyz`、创建备份、把备份恢复到全新的临时 Docker volume，再启动一次性 Blog 容器验证恢复后的文章路由、搜索结果和文章 HTML 中引用的第一个媒体资源。若默认端口、容器名或 volume 名冲突，可覆盖 `RESTORE_SMOKE_PORT`、`RESTORE_SMOKE_CONTAINER` 或 `RESTORE_SMOKE_VOLUME`。

## 使用 Compose 生产部署

内置 Compose 拓扑会让应用容器保持私有，并把 Caddy 放在前面作为反向代理。这里选择 Caddy 不是因为证书必须由 Caddy 管理，而是因为它足够轻量：既能服务纯 HTTP，也能自动申请公网证书，还能加载任意已有的 PEM 证书和私钥。

### 本地或内网 HTTP-only

这是默认模式，不需要公网域名、邮箱或 TLS 证书。只有当你想把公开 URL 或监听地址改成非 localhost 时，才需要在 `compose.yaml` 旁创建 `.env`：

```dotenv
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
```

`BLOG_SITE_ADDRESS` 需要显式带上 `http://` 前缀；这会告诉 Caddy 使用 HTTP-only，而不是尝试自动 HTTPS。然后构建并启动：

```sh
docker compose build
docker compose up -d
docker compose ps
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
```

如果使用 `blog.lan` 之类的内网域名，请把本地 DNS 或 hosts 记录指向 Docker 宿主机，并让 `BLOG_PUBLIC_URL` 和 `BLOG_SITE_ADDRESS` 保持一致。不要把 `BLOG_PUBLIC_URL` 指向 Compose 内部主机名；它会用于生成 canonical 链接和 Cookie 设置。如果修改了 `BLOG_HTTP_PORT`，请把端口也写进 `BLOG_PUBLIC_URL` 和健康检查地址，例如 `make compose-health COMPOSE_HEALTH_URL=http://blog.lan:8080`。

### 公网 HTTPS：由 Caddy 自动申请证书

当 Docker 宿主机可以从公网访问 80/443 端口，并希望 Caddy/ACME 自动申请和续期证书时，使用这个模式：

```dotenv
BLOG_PUBLIC_URL=https://blog.example.com
BLOG_SITE_ADDRESS=blog.example.com
CADDY_EMAIL=ops@example.com
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

```sh
docker compose -f compose.yaml -f compose.https-auto.yaml up -d
```

### HTTPS：使用已有通用证书

当证书来自其他 CA、企业 PKI、NAS 证书管理器、通配符证书或任何已有 PEM 证书/私钥对时，使用这个模式。把宿主机证书目录挂载给 Caddy：

```dotenv
BLOG_PUBLIC_URL=https://blog.example.com
BLOG_SITE_ADDRESS=blog.example.com
TLS_CERTS_DIR=/srv/diary-blog/certs
TLS_CERT_FILE=/certs/fullchain.pem
TLS_KEY_FILE=/certs/privkey.pem
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

```sh
docker compose -f compose.yaml -f compose.https-files.yaml up -d
```

`TLS_CERT_FILE` 和 `TLS_KEY_FILE` 是 Caddy 容器内路径；除非同步修改 override 挂载，否则建议保持在 `/certs` 下。证书的 SAN 必须匹配 `BLOG_SITE_ADDRESS`。

如果 HTTPS 已经由外部网关终止，例如 Nginx、Traefik、云负载均衡或 NAS 门户，请保留默认 HTTP-only Compose 模式，并把 `BLOG_PUBLIC_URL` 设置为外部 `https://...` 地址。在这种拓扑里，内置 Caddy 只是本地 HTTP 反向代理，不管理任何证书。

### 网络、存储和常用命令

Compose 会把 Caddy 固定在私有应用网络的 `172.30.0.10`，并把 `BLOG_TRUSTED_PROXY_CIDRS=172.30.0.10/32` 传给应用，使登录限流和日志能够使用可信的 forwarded client address，同时不信任任意 peer。如果宿主机已有 Docker 网络与 `172.30.0.0/24` 冲突，需要同时调整 Compose subnet 和 trusted proxy CIDR。

Caddy 是唯一暴露端口的服务；应用容器只在内部 Compose 网络中可访问。HTTP-only 和 ACME HTTP challenge 会使用 80 端口，443 端口只在 HTTPS 模式使用。如果宿主机端口冲突，可覆盖 `BLOG_HTTP_PORT` 或 `BLOG_HTTPS_PORT`。博客持久数据存储在 `blog_data` volume 的 `/data/site`，备份存储在 `/data/backups`；使用 Caddy 自动 HTTPS 时，Caddy 管理的证书存储在 `caddy_data`。

常用容器命令也可以使用 Makefile：

```sh
make container-build
make compose-up-http
make compose-up-https-auto
make compose-up-https-files
make compose-health COMPOSE_HEALTH_URL=http://localhost
make compose-logs
```

容器启动后创建初始管理员：

```sh
printf '%s\n' 'choose-a-long-password' | docker compose exec -T blog /app/blog admin reset-password --username admin
```

## 备份与恢复

在持久化 volume 中创建应用一致性备份：

```sh
docker compose exec -T blog /app/blog backup --output /data/backups/backup-$(date +%F).tar.gz
```

请把生成的备份归档复制到 Docker 宿主机之外，作为灾备副本。

恢复前先停止应用，然后显式使用 `--force`：

```sh
docker compose stop blog
docker compose run --rm --no-deps blog restore --input /data/backups/backup-YYYY-MM-DD.tar.gz --force
docker compose up -d blog
```

`restore` 会先验证归档再替换数据。恢复演练建议使用新的或已确认为空的 `blog_data` volume。在容器拓扑中，命名 volume 挂载在 `/data`，应用数据目录是 `/data/site`；这样恢复逻辑可以在 volume 内 staging 和 rename 数据目录，而不是替换挂载点本身。

要把备份恢复到隔离临时 volume 并通过 HTTP 验证恢复结果，可以在 `make backup` 后运行：

```sh
make restore-smoke
```

该目标会把运行中 Blog 容器内的备份复制到 staging volume，恢复到临时数据 volume，在 `127.0.0.1:18081` 启动一次性服务，检查配置的文章路径、搜索词，并请求文章 HTML 中第一个 `/media/...` 资源。执行结束后会清理临时容器和 volume。

## 升级

1. 创建备份，并把备份复制到 Docker 宿主机之外。
2. 拉取新源码或新镜像，阅读发布说明；如果新增必填环境变量，同步更新 `.env`。
3. 执行 `docker compose build`，或在使用版本化镜像时拉取对应镜像。
4. HTTP-only 模式执行 `docker compose up -d`；HTTPS 模式则带上对应 override 文件。启动迁移会在 Blog 报告 ready 之前完成。
5. 检查 `docker compose ps`，再通过 Caddy 请求 `/healthz` 和 `/readyz`。
6. 在健康检查和内容 smoke test 成功前，保留上一版镜像。只有在需要回滚数据时才恢复备份。

## 常用命令速查

```sh
make build                         # 构建管理后台并生成 Go 二进制
make dev                           # 本地启动服务
make test                          # Go + 管理后台单测
make release-gate                  # 非容器发布门禁
make container-release-gate         # 容器构建、健康检查、备份、恢复演练
make backup                         # 容器内创建备份
make restore RESTORE=/data/backups/backup-YYYY-MM-DD.tar.gz
make restore-smoke                  # fresh-volume 恢复演练
```

## 当前限制

- 当前开发宿主机没有 Docker CLI，因此容器 build/up/health/restore-smoke 的实机验证需要在具备 Docker Desktop/Engine 的宿主机补跑。
- 当前开发宿主机无法稳定下载 Playwright bundled Chromium；本地 E2E 默认使用已安装 Chrome。干净 CI 或开发机需要安装 bundled Chromium 并设置 `PLAYWRIGHT_BUNDLED_CHROMIUM=1`，或提供可用的 Chrome channel。
