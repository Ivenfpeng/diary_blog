# diary_blog

[中文说明](README.zh-CN.md)

`diary_blog` is a single-binary technical blog: a server-rendered public site,
a Vue admin console, SQLite storage, search, media uploads, backup/restore, and
Compose deployment recipes.

## Tech stack

- Backend: Go 1.27, `net/http`/Chi-style routing, `html/template`, embedded
  assets, structured `slog` logging.
- Admin UI: Vue 3, TypeScript, Vite, Vitest, CodeMirror, Playwright E2E.
- Storage: SQLite with WAL, embedded migrations, FTS5 trigram search,
  published snapshots, revision history, and online backup.
- Security: single-admin auth, Argon2id passwords, hashed sessions, CSRF
  double-submit checks, login throttling, and trusted reverse-proxy CIDRs.
- Runtime: one Go binary plus Caddy as the bundled reverse proxy.
- Containers: multi-stage Dockerfile, GHCR image publishing, Compose files
  that work with Docker Compose or Podman Compose.

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

## Direct image deployment

You do not need the full source tree on a deployment host. Copy only these
files into one directory:

- `compose.deploy.yaml`
- `compose.https-auto.yaml` or `compose.https-files.yaml`, only for HTTPS modes
- `deploy/Caddyfile`
- `deploy/Caddyfile.https-auto` or `deploy/Caddyfile.https-files`, only for HTTPS modes

Then set the image and run Compose:

```sh
mkdir -p diary-blog/deploy
cd diary-blog
curl -fsSLO https://raw.githubusercontent.com/Ivenfpeng/diary_blog/main/compose.deploy.yaml
curl -fsSLo deploy/Caddyfile https://raw.githubusercontent.com/Ivenfpeng/diary_blog/main/deploy/Caddyfile
```

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:latest
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
```

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

Use `ghcr.io/ivenfpeng/diary_blog:<version-or-sha>` instead of `latest` when
you want pinned releases. The project also keeps `compose.yaml` for building
from source locally.

## Container command switch

Makefile defaults to Docker:

```sh
make container-build
make compose-up-http
make deploy-up-http
```

Use Podman by overriding the command variables:

```sh
make container-build CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman
BLOG_PUBLIC_URL=http://localhost:18080 BLOG_SITE_ADDRESS=http://localhost BLOG_HTTP_PORT=18080 BLOG_HTTPS_PORT=18443 \
  make deploy-up-http CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman BLOG_IMAGE=localhost/diary_blog_blog:latest
make compose-health COMPOSE_HEALTH_URL=http://localhost:18080
```

If host ports 80/443 are occupied or unavailable, set `BLOG_HTTP_PORT` and
`BLOG_HTTPS_PORT` in `.env` or the shell.

## Deployment modes

### 1. Local or intranet HTTP-only

Default mode. No public domain, email address, or TLS certificate is required.

```dotenv
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
```

```sh
docker compose -f compose.deploy.yaml up -d
# or
podman-compose -f compose.deploy.yaml up -d
```

Use an explicit `http://` address to keep Caddy in HTTP-only mode. For an
intranet name such as `blog.lan`, point DNS or a hosts entry to the container
host.

### 2. Public HTTPS with Caddy automatic certificates

Use this when the host is reachable from the public internet on 80/443 and
Caddy should obtain and renew the certificate.

```dotenv
BLOG_PUBLIC_URL=https://blog.example.com
BLOG_SITE_ADDRESS=blog.example.com
CADDY_EMAIL=ops@example.com
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
```

### 3. HTTPS with existing certificate files

Use this for certificates from any CA, enterprise PKI, wildcard certificate, NAS
certificate manager, or other PEM certificate/key pair.

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
docker compose -f compose.deploy.yaml -f compose.https-files.yaml up -d
```

The certificate SAN must match `BLOG_SITE_ADDRESS`.

### 4. External HTTPS gateway

If HTTPS terminates at Nginx, Traefik, a cloud load balancer, a NAS portal, or
another gateway, keep this Compose stack in HTTP-only mode and set
`BLOG_PUBLIC_URL` to the external `https://...` URL.

## Operations

```sh
make compose-ps
make compose-logs
make compose-health COMPOSE_HEALTH_URL=http://localhost
make backup
make restore RESTORE=/data/backups/backup-YYYY-MM-DD.tar.gz
make restore-smoke
```

The app stores site data under `/data/site` in the `blog_data` volume and
backups under `/data/backups`. Caddy is the only published service; the Go app
stays on the private Compose network.
