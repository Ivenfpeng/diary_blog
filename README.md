# diary_blog

[中文说明](README.zh-CN.md)

`diary_blog` is a single-binary technical blog: a server-rendered public site,
a Vue admin console, SQLite storage, search, media uploads, published snapshots,
revision history, backup/restore, and Docker/Podman Compose deployment.

## Tech stack

- Backend: Go 1.27, `net/http`/Chi-style routing, `html/template`, embedded
  assets, and structured `slog` logging.
- Admin UI: Vue 3, TypeScript, Vite, Vitest, a built-in rich text editor, and
  Playwright E2E.
- Storage: SQLite WAL, embedded migrations, FTS5 trigram search, published
  snapshots, revision history, and online backup.
- Security: single-admin auth, Argon2id password hashes, hashed sessions, CSRF
  double-submit checks, login throttling, and trusted reverse-proxy CIDRs.
- Runtime: one Go binary behind Caddy.
- Containers: multi-stage Dockerfile, GHCR image publishing workflow,
  source-build Compose, and image-only deployment Compose.

## Deployment files

- `compose.yaml`: builds the image from source. Use it for local development,
  CI, and release validation. It passes `GOPROXY` and `GOSUMDB` environment
  values into the container build.
- `compose.deploy.yaml`: pulls and runs `BLOG_IMAGE`. Use it on servers where
  you do not want to clone the source or build locally.
- `compose.https-auto.yaml`: override for Caddy-managed public HTTPS.
- `compose.https-files.yaml`: override for an existing PEM certificate/key pair.

## Direct image deployment

The deployment host only needs the Compose and Caddyfile configuration files.
After the PR is merged, `GIT_REF=main` works. Before merge, set `GIT_REF` to
the latest commit sha, tag, or an accessible branch ref; `main` will not contain
new deployment files yet.

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

During PR verification, prefer a commit sha so the raw URL is stable and does
not depend on branch-name parsing or merge status.

For Caddy automatic HTTPS:

```sh
curl -fsSL \
  -o compose.https-auto.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.https-auto.yaml"

curl -fsSL \
  -o deploy/Caddyfile.https-auto \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile.https-auto"
```

For file-based HTTPS:

```sh
curl -fsSL \
  -o compose.https-files.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.https-files.yaml"

curl -fsSL \
  -o deploy/Caddyfile.https-files \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile.https-files"
```

If a downloaded file contains HTML, the URL is a GitHub web page, not a raw
file. Use `raw.githubusercontent.com`, not `github.com/.../blob/...`.

## Environment

HTTP-only example:

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

Caddy automatic HTTPS example:

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=https://ivenpeng.top
BLOG_SITE_ADDRESS=ivenpeng.top
CADDY_EMAIL=ivenfpeng@gmail.com
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

Existing certificate example:

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

Important variables:

| Variable | Purpose |
| --- | --- |
| `BLOG_IMAGE` | Application image to run. `latest` must already exist in GHCR. Use a version or sha tag for production. |
| `BLOG_PUBLIC_URL` | Public site URL used for RSS, Sitemap, generated links, and cookie behavior. Include the external scheme and host port when applicable, for example `http://localhost:18080`. |
| `BLOG_SITE_ADDRESS` | Caddy's in-container site address. Use explicit `http://...` for HTTP-only; use a hostname for automatic HTTPS. When Compose maps host `18080` to container `80`, use `http://localhost` here, not `http://localhost:18080`. |
| `BLOG_HTTP_PORT` | Host HTTP port, default `80`. |
| `BLOG_HTTPS_PORT` | Host HTTPS port, default `443`. |
| `CADDY_EMAIL` | ACME email for Caddy automatic HTTPS. |
| `TLS_CERTS_DIR` | Host certificate directory for file-based HTTPS. |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | Certificate and key paths inside the container. |

## Start modes

HTTP-only:

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

Caddy automatic HTTPS:

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
```

Existing certificate files:

```sh
docker compose -f compose.deploy.yaml -f compose.https-files.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-files.yaml up -d
```

If HTTPS terminates at Nginx, Traefik, a load balancer, or another gateway,
keep this stack in HTTP-only mode and set `BLOG_PUBLIC_URL` to the external
`https://...` URL.

## Content authoring notes

The admin console stores article body content as Markdown. Its built-in rich
text editor serializes headings, paragraphs, emphasis, quotes, lists, code
blocks, and uploaded images back to Markdown before saving. Use the editor's
image button, paste an image file, or drag an image file into the editor; images
are uploaded to the media library and inserted as `/media/...` Markdown links.
Embedded `data:image/...` URLs are blocked so large base64 images are not saved
into posts.

Article, category, and tag slugs support Unicode letters, Unicode numbers, and
single hyphens. Chinese slugs are valid, for example:

```text
数据库-笔记-2026
事实-2026
```

Spaces, slashes, leading/trailing hyphens, and repeated hyphens are rejected.
This keeps public routes unambiguous: `/posts/数据库-笔记-2026` is valid, while
`/posts/数据库/笔记` would be interpreted as multiple path segments. Some tools
may display copied Chinese URLs as percent-encoded text; that is normal HTTP URL
encoding.

When a new article has no manual slug yet, the admin editor generates one from
the title automatically. Editing the slug field turns off title-based updates
for that article session.

## GHCR image notes

The default image is:

```text
ghcr.io/ivenfpeng/diary_blog:latest
```

If Compose reports `not found`, that tag has not been published or is private
to the current Docker login.

Publish manually:

```sh
docker login ghcr.io -u Ivenfpeng

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  -t ghcr.io/ivenfpeng/diary_blog:latest \
  --push .
```

Publish a single-architecture image with Podman:

```sh
podman login ghcr.io -u Ivenfpeng

podman build \
  --platform linux/amd64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  -t ghcr.io/ivenfpeng/diary_blog:codex-diary-blog-mvp \
  -t ghcr.io/ivenfpeng/diary_blog:latest \
  .

podman push ghcr.io/ivenfpeng/diary_blog:codex-diary-blog-mvp
podman push ghcr.io/ivenfpeng/diary_blog:latest
```

Publish an amd64 + arm64 multi-architecture image with Podman:

```sh
podman login ghcr.io -u Ivenfpeng

IMAGE=ghcr.io/ivenfpeng/diary_blog
MANIFEST=localhost/diary_blog:multiarch

podman manifest rm "${MANIFEST}" 2>/dev/null || true
podman manifest create "${MANIFEST}"

podman build \
  --platform linux/amd64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --manifest "${MANIFEST}" \
  .

podman build \
  --platform linux/arm64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --manifest "${MANIFEST}" \
  .

podman manifest push --all "${MANIFEST}" "docker://${IMAGE}:codex-diary-blog-mvp"
podman manifest push --all "${MANIFEST}" "docker://${IMAGE}:latest"
```

If `go mod download` fails with a `storage.googleapis.com` TLS timeout or EOF,
the default Go module proxy is unreachable from the build environment. Keep the
`--build-arg GOPROXY=https://goproxy.cn,direct` option above, or switch it to
`--build-arg GOPROXY=https://proxy.golang.org,direct` when the official proxy is
faster on your network.

If a cross-architecture build fails, verify that the Podman machine or Linux
host has QEMU/binfmt enabled. A VPS is usually `linux/amd64`; Apple Silicon is
usually `linux/arm64`, so building the other architecture requires emulation.

For one amd64 VPS:

```sh
docker buildx build \
  --platform linux/amd64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  -t ghcr.io/ivenfpeng/diary_blog:latest \
  --push .
```

Pull from a private package:

```sh
docker login ghcr.io -u Ivenfpeng
docker pull ghcr.io/ivenfpeng/diary_blog:latest
```

## Admin account

Admin URL:

```text
https://your-domain/admin/
```

The system stores only an Argon2id hash, not the plaintext password. You cannot
view the current password; reset it instead.
There is no user-list or password-view command. If you are unsure whether the
admin account exists, reset the same `--username`; it will initialize or update
that account.

Reset or initialize the administrator:

```sh
printf '%s\n' 'replace-with-a-strong-password' | \
  docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog admin reset-password --username admin
```

For automatic HTTPS, use the same Compose file set:

```sh
printf '%s\n' 'replace-with-a-strong-password' | \
  docker compose -f compose.deploy.yaml -f compose.https-auto.yaml exec -T blog \
  /app/blog admin reset-password --username admin
```

## Operations

Use the same `-f` file set that you used to start the stack.

```sh
docker compose -f compose.deploy.yaml ps
docker compose -f compose.deploy.yaml logs --tail=200
docker compose -f compose.deploy.yaml logs -f caddy
docker compose -f compose.deploy.yaml logs -f blog
```

Health checks:

```sh
curl -i http://127.0.0.1/healthz
curl -i http://127.0.0.1/readyz
curl -I http://your-domain
curl -kI https://your-domain
```

Upgrade:

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

Backup:

```sh
docker compose -f compose.deploy.yaml exec -T blog \
  /app/blog backup --output /data/backups/backup-$(date +%F).tar.gz
```

Restore:

```sh
docker compose -f compose.deploy.yaml stop blog
docker compose -f compose.deploy.yaml run --rm --no-deps blog \
  restore --input /data/backups/backup-YYYY-MM-DD.tar.gz --force
docker compose -f compose.deploy.yaml up -d blog
```

Rebuild search:

```sh
docker compose -f compose.deploy.yaml exec -T blog /app/blog search rebuild
```

Run migrations:

```sh
docker compose -f compose.deploy.yaml exec -T blog /app/blog migrate
```

`docker compose down` keeps named volumes by default. Do not run
`docker compose down -v` unless you intentionally want to delete site data.

## Data and ports

- Site data lives at `/data/site` in the `blog_data` volume.
- Backups should live under `/data/backups`.
- Caddy stores managed HTTPS data in `caddy_data`.
- Only Caddy publishes host ports; the Go app stays on the private Compose
  network.
- The default internal subnet is `172.30.0.0/24`, with Caddy at
  `172.30.0.10`. If this conflicts with another Docker network, update both
  the Compose subnet and `BLOG_TRUSTED_PROXY_CIDRS`.

## HTTPS troubleshooting

Check DNS:

```sh
curl -4 ifconfig.me
dig +short ivenpeng.top
```

Check listening ports:

```sh
ss -lntp | grep -E ':80|:443'
```

Check that the HTTPS override is active:

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml ps
```

Check Caddy certificate logs:

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml logs caddy --tail=200
```

Common causes:

- The domain does not point to the VPS.
- Vultr firewall or the host firewall blocks 80/443.
- The stack was started without `compose.https-auto.yaml`.
- `BLOG_IMAGE` does not exist in GHCR or the server is not logged in.
- A Markdown or GitHub `/blob/` URL was copied instead of the raw file URL.

## Local development

Prerequisites:

- Go 1.27+
- Node.js 24 + npm
- Docker Compose or Podman Compose for container deployment checks

```sh
npm --prefix admin ci
make build
printf '%s\n' 'choose-a-long-password' | ./bin/blog admin reset-password --username admin
make dev
```

Open <http://localhost:8080> for the public site and
<http://localhost:8080/admin> for the admin console.

Useful checks:

```sh
make test
make vet
make release-gate
git diff --check
```

Playwright E2E defaults to `http://127.0.0.1:18080`. If a local
Docker/Podman deployment is already using that port, run it on another port:

```sh
BLOG_E2E_PORT=18082 npm run test:e2e
```

You can also override the full base URL:

```sh
BLOG_E2E_BASE_URL=http://127.0.0.1:18082 npm run test:e2e
```
