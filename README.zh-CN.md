# diary_blog

[English](README.md)

`diary_blog` 是一个单二进制技术博客系统：服务端渲染公开站点、Vue 管理后台、SQLite 内容存储、搜索、媒体上传、备份恢复，以及多种 Compose 部署方案。

## 技术栈

- 后端：Go 1.27、`net/http`/Chi 风格路由、`html/template`、嵌入式静态资源、结构化 `slog` 日志。
- 管理后台：Vue 3、TypeScript、Vite、Vitest、CodeMirror、Playwright E2E。
- 存储：SQLite WAL、嵌入式迁移、FTS5 trigram 全文搜索、发布快照、历史版本、online backup。
- 安全：单管理员、Argon2id 密码、哈希会话、CSRF 双提交校验、登录限流、可信反向代理 CIDR。
- 运行时：一个 Go 二进制，加内置 Caddy 反向代理。
- 容器：多阶段 Dockerfile；Compose 文件可配合 Docker Compose 或 Podman Compose 使用。

## 本地开发

依赖：

- Go 1.27+
- Node.js 24 + npm
- Docker Compose 或 Podman Compose，用于容器部署验证

```sh
npm --prefix admin ci
make build
printf '%s\n' 'choose-a-long-password' | ./bin/blog admin reset-password --username admin
make dev
```

公开站点：<http://localhost:8080>

管理后台：<http://localhost:8080/admin>

常用检查：

```sh
make test
make vet
make release-gate
git diff --check
```

## 容器命令切换

Makefile 默认使用 Docker：

```sh
make container-build
make compose-up-http
```

本机使用 Podman 时覆盖变量即可：

```sh
make container-build CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman
BLOG_PUBLIC_URL=http://localhost:18080 BLOG_SITE_ADDRESS=http://localhost BLOG_HTTP_PORT=18080 BLOG_HTTPS_PORT=18443 \
  make compose-up-http CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman
make compose-health COMPOSE_HEALTH_URL=http://localhost:18080
```

如果宿主机 80/443 被占用或不可用，可在 `.env` 或 shell 中设置 `BLOG_HTTP_PORT`、`BLOG_HTTPS_PORT`。

## 部署方案

### 1. 本地/内网 HTTP-only

默认模式。不需要公网域名、邮箱或 TLS 证书。

```dotenv
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
```

```sh
docker compose up -d
# 或
podman-compose up -d
```

`BLOG_SITE_ADDRESS` 显式使用 `http://`，Caddy 就会保持 HTTP-only。若使用 `blog.lan` 这类内网域名，把 DNS 或 hosts 记录指向容器宿主机即可。

### 2. 公网 HTTPS：Caddy 自动证书

适合宿主机公网可访问 80/443，并希望 Caddy 自动申请和续期证书的场景。

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

### 3. HTTPS：使用已有证书文件

适合任意 CA、企业 PKI、通配符证书、NAS 证书管理器或其他 PEM 证书/私钥对。

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

证书 SAN 必须匹配 `BLOG_SITE_ADDRESS`。

### 4. 外部网关终止 HTTPS

如果 HTTPS 已经由 Nginx、Traefik、云负载均衡、NAS 门户或其他网关终止，本 Compose 栈保持 HTTP-only 即可，并把 `BLOG_PUBLIC_URL` 设置成外部 `https://...` 地址。

## 运维命令

```sh
make compose-ps
make compose-logs
make compose-health COMPOSE_HEALTH_URL=http://localhost
make backup
make restore RESTORE=/data/backups/backup-YYYY-MM-DD.tar.gz
make restore-smoke
```

应用数据位于 `blog_data` volume 的 `/data/site`，备份位于 `/data/backups`。Caddy 是唯一暴露端口的服务，Go 应用只在 Compose 私有网络内访问。
