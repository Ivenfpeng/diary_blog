# diary_blog

[中文说明](README.zh-CN.md)

A single-binary Go blog with a Vue administration interface, SQLite content
storage, and Compose deployment recipes for local HTTP, automatic HTTPS, or
bring-your-own TLS certificates.

## Local development

Prerequisites:

- Go 1.27 or newer
- Node.js 24 and npm (the project uses Homebrew's `node@24` when available)
- Docker Desktop with Docker Compose v2 for the container workflow

When Homebrew's Node 24 is not already on your shell path, run:

```sh
export PATH="$(brew --prefix node@24)/bin:$PATH"
```

Install the admin dependencies, then use the project build target. It builds
the admin application, copies it into `web/admin`, and embeds those files in
the Go binary.

```sh
npm --prefix admin ci
make build
```

Create the first administrator (or reset an existing administrator password):

```sh
printf '%s\n' 'choose-a-long-password' | ./bin/blog admin reset-password --username admin
```

For a local server, use the defaults (`./data` and `http://localhost:8080`):

```sh
make dev
```

Open <http://localhost:8080> for the site and
<http://localhost:8080/admin> to sign in. Set `BLOG_PUBLIC_URL` when testing
links or cookies against another local hostname.

## Tests and checks

```sh
make test
make vet
make build
```

`make test-go` and `make vet` store their Go build cache in
`/tmp/diary-blog-go-cache` by default. Override it with `GO_CACHE=/path` if
needed.

### End-to-end release gate

Install the root browser-test dependencies once. The default local gate uses an
installed Google Chrome channel because that browser is already available on
this development host.

```sh
npm ci
npm run test:e2e
```

On a clean machine without Chrome, install Playwright's bundled Chromium and
set `PLAYWRIGHT_BUNDLED_CHROMIUM=1` when running the gate:

```sh
npx playwright install chromium
PLAYWRIGHT_BUNDLED_CHROMIUM=1 npm run test:e2e
```

`npm run test:e2e` rebuilds and embeds the administration application before
starting an isolated Blog server with temporary data. It verifies login,
authoring, media upload, preview, publication, discovery, revision restore,
and public exclusion of drafts and archived articles. It also writes the home,
article, administration-list, and editor screenshots for 1440×1000,
1024×768, 390×844, and 360×800 to `/tmp/diary-blog-e2e-results`.

Run the non-container release gate with:

```sh
make release-gate
git diff --check
```

For the production runtime gate, seed or keep one published smoke article with
an uploaded media image, then run:

```sh
make container-release-gate \
  BACKUP=/data/backups/release-smoke.tar.gz \
  SMOKE_ARTICLE_PATH=/posts/release-gate-publishing-workflow \
  SMOKE_SEARCH_QUERY=durable
```

This builds the Compose image, starts the default HTTP-only production
topology, checks `/healthz` and `/readyz` through the reverse proxy, creates
the backup, restores it into a fresh temporary Docker volume, starts a one-off
Blog container against that volume, and verifies the restored article route,
search result, and first media asset referenced by the article HTML. Override
`RESTORE_SMOKE_PORT`, `RESTORE_SMOKE_CONTAINER`, or `RESTORE_SMOKE_VOLUME` if
the defaults conflict with another local smoke run.

## Production deployment with Compose

The bundled Compose topology keeps the application container private and puts
Caddy in front as the reverse proxy. Caddy is not required because of its
certificate manager specifically; it is used here as a small edge proxy that can
serve plain HTTP, obtain public certificates automatically, or load any normal
certificate/key pair you already have.

### Local or intranet HTTP-only

This is the default mode and does not require a public domain, email address, or
TLS certificate. Create `.env` next to `compose.yaml` only when you want the
public URL or listen address to be something other than localhost:

```dotenv
BLOG_PUBLIC_URL=http://blog.lan
BLOG_SITE_ADDRESS=http://blog.lan
BLOG_HTTP_PORT=80
```

Use an explicit `http://` prefix for `BLOG_SITE_ADDRESS`; that tells Caddy to
serve HTTP-only instead of attempting automatic HTTPS. Then build and start:

```sh
docker compose build
docker compose up -d
docker compose ps
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
```

For an intranet hostname such as `blog.lan`, point local DNS or a hosts-file
entry at the Docker host and set both `BLOG_PUBLIC_URL` and `BLOG_SITE_ADDRESS`
to that address. If you change `BLOG_HTTP_PORT`, include that port in
`BLOG_PUBLIC_URL` and in health checks, for example
`make compose-health COMPOSE_HEALTH_URL=http://blog.lan:8080`. Do not point
`BLOG_PUBLIC_URL` at an internal Compose hostname: it is used to produce
canonical links and cookie settings.

### Public HTTPS with Caddy automatic certificates

Use this mode when the Docker host is reachable from the public internet on
ports 80 and 443 and you want Caddy/ACME to obtain and renew the certificate:

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

### HTTPS with an existing certificate

Use this mode for certificates issued by another CA, an enterprise PKI, a NAS
certificate manager, a wildcard certificate, or any other PEM certificate/key
pair. Mount the host directory containing those files into Caddy:

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

`TLS_CERT_FILE` and `TLS_KEY_FILE` are paths inside the Caddy container; keep
them under `/certs` unless you also change the override mount. The certificate
must include a SAN that matches `BLOG_SITE_ADDRESS`.

If an external gateway such as Nginx, Traefik, a cloud load balancer, or a NAS
portal terminates HTTPS before traffic reaches this Compose stack, keep the
default HTTP-only Compose mode and set `BLOG_PUBLIC_URL` to the external
`https://...` URL. In that layout, the bundled Caddy remains a local HTTP
reverse proxy and does not manage any certificate.

### Network, storage, and common commands

Compose assigns Caddy `172.30.0.10` on the private application network and
passes `BLOG_TRUSTED_PROXY_CIDRS=172.30.0.10/32` to the app so login throttling
and logs can use trusted forwarded client addresses without trusting arbitrary
peers. If `172.30.0.0/24` conflicts with an existing Docker network on your
host, update both the Compose subnet and the trusted proxy CIDR together.

Caddy is the only service with published ports; the application container is
available only on the internal Compose network. Port 80 is used by HTTP-only
and ACME HTTP challenge flows, while port 443 is used only by HTTPS modes.
Override `BLOG_HTTP_PORT` or `BLOG_HTTPS_PORT` if those host ports are already
in use.
Persistent blog data is stored under `/data/site` in the `blog_data` named
volume, backups under `/data/backups`, and Caddy-managed certificates, when
using automatic HTTPS, in `caddy_data`.

Use these Makefile helpers for the common operations:

```sh
make container-build
make compose-up-http
make compose-up-https-auto
make compose-up-https-files
make compose-health COMPOSE_HEALTH_URL=http://localhost
make compose-logs
```

Create the initial administrator after the containers are running:

```sh
printf '%s\n' 'choose-a-long-password' | docker compose exec -T blog /app/blog admin reset-password --username admin
```

## Backup and restore

Create an application-consistent backup inside the persistent volume:

```sh
docker compose exec -T blog /app/blog backup --output /data/backups/backup-$(date +%F).tar.gz
```

Copy the resulting archive off the Docker host for disaster recovery. To
restore, stop the application first and restore explicitly with `--force`:

```sh
docker compose stop blog
docker compose run --rm --no-deps blog restore --input /data/backups/backup-YYYY-MM-DD.tar.gz --force
docker compose up -d blog
```

`restore` validates the archive before replacing data. Use a new or known
empty `blog_data` volume when rehearsing a restore. In the container topology
the named volume is mounted at `/data`, while the application data directory is
`/data/site`; that lets restore stage and rename directories inside the volume
instead of trying to replace the mount point itself.

To rehearse a restore into an isolated temporary volume and verify restored
content through HTTP, run `make restore-smoke` after `make backup`. It copies
the archive out of the running Blog container, restores it into the temporary
volume, serves that volume on `127.0.0.1:18081`, checks the configured article
path and search query, and curls the first `/media/...` asset found in the
article HTML. The target removes the temporary container and volume when it
finishes.

## Upgrading

1. Create and copy off a backup.
2. Pull the new source or image, review any release notes, and update `.env` if
   a new required variable is introduced.
3. Run `docker compose build` (or pull the versioned image if your deployment
   uses one).
4. Run `docker compose up -d` for HTTP-only, or include the matching HTTPS
   override file; startup migrations are applied before the blog reports ready.
5. Check `docker compose ps`, then request `/healthz` and `/readyz` through
   Caddy.
6. Keep the prior image available until the health checks and a content smoke
   test succeed. Restore the backup only when rolling back data is necessary.
