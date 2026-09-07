# diary_blog

[English](README.md)

`diary_blog` 是一个单二进制技术博客系统：公开站点服务端渲染，管理后台使用 Vue SPA，内容存储在 SQLite 中，并内置搜索、媒体上传、文章发布快照、修订历史、备份恢复和 Compose 部署方案。

## 技术栈

- 后端：Go 1.27、`net/http`/Chi 风格路由、`html/template`、嵌入式静态资源、结构化 `slog` 日志。
- 管理后台：Vue 3、TypeScript、Vite、Vitest、CodeMirror、Playwright E2E。
- 存储：SQLite WAL、嵌入式迁移、FTS5 trigram 全文搜索、发布快照、历史版本、online backup。
- 安全：单管理员、Argon2id 密码哈希、哈希会话、CSRF 双提交校验、登录限流、可信反向代理 CIDR。
- 运行时：一个 Go 二进制，加 Caddy 反向代理。
- 容器：多阶段 Dockerfile、GHCR 镜像发布 workflow、源码构建 Compose、image-only 部署 Compose；Docker Compose 和 Podman Compose 都可用。

## 部署文件怎么选

仓库里有两类 Compose 文件：

- `compose.yaml`：给开发机或 CI 从源码构建镜像用，包含 `build`。
- `compose.deploy.yaml`：给服务器部署用，只拉取 `BLOG_IMAGE` 镜像，不需要源码，也不会本地构建。

HTTPS 是通过 override 文件叠加：

- HTTP-only：只用 `compose.deploy.yaml`。
- Caddy 自动 HTTPS：叠加 `compose.https-auto.yaml` 和 `deploy/Caddyfile.https-auto`。
- 使用已有 PEM 证书：叠加 `compose.https-files.yaml` 和 `deploy/Caddyfile.https-files`。

## 直接镜像部署

部署机器不需要 clone 完整源码树。只需要把 Compose 和 Caddyfile 配置文件放到同一个目录，然后拉镜像启动。

### 1. 下载最小配置包

已合并到 `main` 后可以使用 `GIT_REF=main`；PR 未合并时 `main` 还没有这些文件，请把 `GIT_REF` 改成最新 commit sha、tag，或可访问的分支 ref。

```sh
mkdir -p ~/diary_blog/deploy
cd ~/diary_blog

GIT_REF=main

curl -fsSL \
  -o compose.deploy.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.deploy.yaml"

curl -fsSL \
  -o deploy/Caddyfile \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile"
```

当前处在 PR 验证阶段时，优先使用最新 commit sha；这样 raw URL 不受分支名、PR 是否合并影响。

如果是 HTTPS 自动证书，再下载：

```sh
curl -fsSL \
  -o compose.https-auto.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.https-auto.yaml"

curl -fsSL \
  -o deploy/Caddyfile.https-auto \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile.https-auto"
```

如果是已有证书文件，再下载：

```sh
curl -fsSL \
  -o compose.https-files.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.https-files.yaml"

curl -fsSL \
  -o deploy/Caddyfile.https-files \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile.https-files"
```

校验下载内容：

```sh
head -n 5 compose.deploy.yaml
head -n 5 deploy/Caddyfile
```

`compose.deploy.yaml` 开头应该是 `services:`。如果文件里是 HTML，说明下载的是 GitHub 网页地址，不是 raw 地址；不要用 `github.com/.../blob/...`，也不要复制 Markdown 形式的 `[url](url)`。

### 2. 配置 `.env`

HTTP-only / 内网示例：

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

公网 HTTPS + Caddy 自动证书示例：

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=https://ivenpeng.top
BLOG_SITE_ADDRESS=ivenpeng.top
CADDY_EMAIL=ivenfpeng@gmail.com
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

已有 PEM 证书示例：

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=https://blog.example.com
BLOG_SITE_ADDRESS=blog.example.com
TLS_CERTS_DIR=/srv/diary-blog/certs
TLS_CERT_FILE=/certs/fullchain.pem
TLS_KEY_FILE=/certs/privkey.pem
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

关键变量说明：

| 变量 | 说明 |
| --- | --- |
| `BLOG_IMAGE` | 要运行的应用镜像。`latest` 必须已经发布到 GHCR，否则会 `not found`。生产建议用版本号或 sha tag 固定。 |
| `BLOG_PUBLIC_URL` | 公开访问地址，用于 RSS、Sitemap、链接生成和 Cookie 相关行为。外部是 HTTPS 时这里也要写 `https://...`。 |
| `BLOG_SITE_ADDRESS` | Caddy 站点地址。HTTP-only 必须显式写 `http://...`；自动 HTTPS 写域名即可，如 `ivenpeng.top`。 |
| `BLOG_HTTP_PORT` | 宿主机 HTTP 端口，默认 `80`。本机测试可改成 `18080`。 |
| `BLOG_HTTPS_PORT` | 宿主机 HTTPS 端口，默认 `443`。本机测试可改成 `18443`。 |
| `CADDY_EMAIL` | Caddy 自动申请证书时用于 ACME 注册和通知。HTTP-only 不需要。 |
| `TLS_CERTS_DIR` | 已有证书模式下，宿主机证书目录。 |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | 容器内证书和私钥路径，默认 `/certs/fullchain.pem` 和 `/certs/privkey.pem`。 |

## 启动方式

### 本地/内网 HTTP-only

不需要公网域名、邮箱或 TLS 证书。`BLOG_SITE_ADDRESS` 必须带 `http://`，这样 Caddy 不会尝试自动申请证书。

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

Podman 平替：

```sh
podman-compose -f compose.deploy.yaml pull
podman-compose -f compose.deploy.yaml up -d
```

### 公网 HTTPS：Caddy 自动证书

适合域名已解析到服务器，且公网 80/443 能访问的场景。

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
```

注意：只运行 `docker compose -f compose.deploy.yaml up -d` 是 HTTP-only 配置，不会加载自动 HTTPS Caddyfile。公网 HTTPS 必须叠加 `compose.https-auto.yaml`。

### HTTPS：使用已有证书文件

适合任意 CA、企业 PKI、通配符证书、NAS 证书管理器或其他 PEM 证书/私钥对。

```sh
docker compose -f compose.deploy.yaml -f compose.https-files.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-files.yaml up -d
```

证书 SAN 必须匹配 `BLOG_SITE_ADDRESS`。证书目录通过 `TLS_CERTS_DIR` 挂载到容器内 `/certs`。

### 外部网关终止 HTTPS

如果 HTTPS 已经由 Nginx、Traefik、云负载均衡、NAS 门户或其他网关终止，本 Compose 栈保持 HTTP-only 即可；`BLOG_PUBLIC_URL` 写外部 `https://...`，`BLOG_SITE_ADDRESS` 写内网 HTTP 地址。

## 镜像发布与 GHCR 注意事项

默认镜像是：

```text
ghcr.io/ivenfpeng/diary_blog:latest
```

如果启动时报：

```text
ghcr.io/ivenfpeng/diary_blog:latest: not found
```

说明 GHCR 上还没有这个 tag，或者 package 是私有且当前服务器没有登录权限。

手动发布镜像：

```sh
docker login ghcr.io -u Ivenfpeng

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/ivenfpeng/diary_blog:latest \
  --push .
```

只给单台 amd64 VPS 发布也可以：

```sh
docker buildx build \
  --platform linux/amd64 \
  -t ghcr.io/ivenfpeng/diary_blog:latest \
  --push .
```

服务器拉私有 GHCR 镜像时需要登录：

```sh
docker login ghcr.io -u Ivenfpeng
docker pull ghcr.io/ivenfpeng/diary_blog:latest
```

推荐生产使用固定版本：

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:v0.1.0
```

`latest` 适合快速验证，但不利于回滚和复现。

## 管理后台账号与密码

管理后台地址：

```text
https://你的域名/admin/
```

本地 HTTP-only 示例：

```text
http://localhost:18080/admin/
```

系统不会保存明文密码，只保存 Argon2id 哈希。因此“查看当前 admin 密码”在设计上不可行；如果忘记密码，直接重置。
目前没有“列出用户/查看密码”的管理命令；如果不确定账号状态，使用同一个 `--username` 重置即可，存在则更新，不存在则初始化。

初始化或重置管理员密码：

```sh
printf '%s\n' '换成一个强密码' | \
  docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog admin reset-password --username admin
```

如果你当前是 HTTPS 自动证书模式，命令里的 Compose 文件也要保持一致：

```sh
printf '%s\n' '换成一个强密码' | \
  docker compose -f compose.deploy.yaml -f compose.https-auto.yaml exec -T blog \
  /app/blog admin reset-password --username admin
```

如果容器还没长期运行，也可以用一次性容器写入同一个 `blog_data` volume：

```sh
printf '%s\n' '换成一个强密码' | \
  docker compose -f compose.deploy.yaml run --rm --no-deps blog \
  admin reset-password --username admin
```

登录用户名就是 `--username` 指定的值，例如 `admin`。用户名会做规范化处理；建议固定使用小写 `admin`。

## 常用运维命令

以下命令以 image-only 部署为例。如果你使用 HTTPS 自动证书，需要在每条命令里都加上 `-f compose.https-auto.yaml`；如果使用已有证书，需要加上 `-f compose.https-files.yaml`。

查看容器：

```sh
docker compose -f compose.deploy.yaml ps
```

查看日志：

```sh
docker compose -f compose.deploy.yaml logs --tail=200
docker compose -f compose.deploy.yaml logs -f caddy
docker compose -f compose.deploy.yaml logs -f blog
```

健康检查：

```sh
curl -i http://127.0.0.1/healthz
curl -i http://127.0.0.1/readyz
curl -I http://你的域名
curl -kI https://你的域名
```

重启：

```sh
docker compose -f compose.deploy.yaml restart
docker compose -f compose.deploy.yaml restart blog
docker compose -f compose.deploy.yaml restart caddy
```

升级镜像：

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

停止服务：

```sh
docker compose -f compose.deploy.yaml down
```

`down` 默认不会删除 named volume，站点数据仍保留。不要随手执行 `docker compose down -v`，它会删除数据卷。

备份：

```sh
docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog backup --output /data/backups/backup-$(date +%F).tar.gz
```

把备份复制到宿主机：

```sh
BLOG_CONTAINER=$(docker compose -f compose.deploy.yaml ps -q blog)
docker cp "${BLOG_CONTAINER}:/data/backups/backup-$(date +%F).tar.gz" .
```

恢复：

```sh
docker compose -f compose.deploy.yaml stop blog
docker compose -f compose.deploy.yaml run --rm --no-deps blog \
  restore --input /data/backups/backup-YYYY-MM-DD.tar.gz --force
docker compose -f compose.deploy.yaml up -d blog
```

重建搜索索引：

```sh
docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog search rebuild
```

执行迁移：

```sh
docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog migrate
```

## 数据与端口

- 应用数据在 `blog_data` volume 的 `/data/site`。
- 备份建议放在 `/data/backups`。
- Caddy 数据在 `caddy_data`，自动 HTTPS 证书也由 Caddy 管理在这里。
- 只有 Caddy 暴露宿主机端口；Go 应用只在 Compose 私有网络内提供 `8080`。
- 默认网络使用 `172.30.0.0/24`，Caddy 固定地址为 `172.30.0.10`，并被配置为可信代理。如果与宿主机已有 Docker 网络冲突，需要同时调整 Compose subnet 和 `BLOG_TRUSTED_PROXY_CIDRS`。

## HTTPS 排障清单

如果 `https://ivenpeng.top` 访问不了，按这个顺序查：

1. 确认 DNS 指向当前服务器：

   ```sh
   curl -4 ifconfig.me
   dig +short ivenpeng.top
   ```

2. 确认 80/443 端口开放：

   ```sh
   ss -lntp | grep -E ':80|:443'
   ```

   同时检查 Vultr 防火墙、安全组和系统防火墙。

3. 确认使用了 HTTPS override：

   ```sh
   docker compose -f compose.deploy.yaml -f compose.https-auto.yaml ps
   ```

4. 查看 Caddy 证书日志：

   ```sh
   docker compose -f compose.deploy.yaml -f compose.https-auto.yaml logs caddy --tail=200
   ```

5. 确认 blog 容器健康：

   ```sh
   docker compose -f compose.deploy.yaml -f compose.https-auto.yaml ps
   docker compose -f compose.deploy.yaml -f compose.https-auto.yaml logs blog --tail=100
   ```

常见原因：

- 域名还没解析到这台 VPS。
- Vultr 防火墙或系统防火墙没有放行 80/443。
- 只启动了 `compose.deploy.yaml`，没有叠加 `compose.https-auto.yaml`。
- `BLOG_IMAGE` 对应镜像不存在、未公开，或服务器没有 GHCR 权限。
- 复制命令时带了 Markdown 链接格式，导致下载到 HTML。

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

## Makefile 入口

Makefile 默认使用 Docker：

```sh
make container-build
make compose-up-http
make deploy-up-http
make compose-health COMPOSE_HEALTH_URL=http://localhost
```

本机使用 Podman 时覆盖变量即可：

```sh
make container-build CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman

BLOG_PUBLIC_URL=http://localhost:18080 \
BLOG_SITE_ADDRESS=http://localhost \
BLOG_HTTP_PORT=18080 \
BLOG_HTTPS_PORT=18443 \
BLOG_IMAGE=localhost/diary_blog_blog:latest \
make deploy-up-http CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman

make compose-health COMPOSE_HEALTH_URL=http://localhost:18080
```
