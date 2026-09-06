# diary_blog

A single-binary Go blog with a Vue administration interface, SQLite content
storage, and a Caddy reverse proxy for production deployment.

## Local development

Prerequisites:

- Go 1.27 or newer
- Node.js 24 and npm (the project uses Homebrew's `node@24` when available)
- Docker Desktop with Docker Compose v2 for the container workflow

When Homebrew's Node 24 is not already on your shell path, run:

```sh
export PATH="$(brew --prefix node@24)/bin:$PATH"
```

Install the admin dependencies and build its static files before building the
Go binary. The admin build is copied into `web/admin`, which is embedded into
the binary.

```sh
npm --prefix admin ci
npm --prefix admin run build
go build -o bin/blog ./cmd/blog
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

## Production deployment with Compose

Create an environment file next to `compose.yaml` with the public hostname and
the email address Caddy uses for certificate registration:

```dotenv
BLOG_DOMAIN=blog.example.com
BLOG_PUBLIC_URL=https://blog.example.com
CADDY_EMAIL=ops@example.com
# Optional: use a unique cookie name when sharing a parent domain.
BLOG_COOKIE_NAME=diary_blog_session
```

`BLOG_DOMAIN`, `BLOG_PUBLIC_URL`, and `CADDY_EMAIL` are required production
values. `BLOG_COOKIE_NAME` is optional. Do not point `BLOG_PUBLIC_URL` at an
internal Compose hostname: it is used to produce canonical links and cookie
settings.

Build and start the production topology:

```sh
docker compose build
docker compose up -d
docker compose ps
curl --fail http://localhost/healthz
curl --fail http://localhost/readyz
```

Caddy is the only service with published ports (80 and 443); the application
container is available only on the internal Compose network. Persistent blog
data is stored in the `blog_data` named volume and Caddy certificates in
`caddy_data`. Use `make container-build`, `make compose-up`, and
`make compose-logs` as equivalents for the common operations.

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
empty `blog_data` volume when rehearsing a restore.

## Upgrading

1. Create and copy off a backup.
2. Pull the new source or image, review any release notes, and update `.env` if
   a new required variable is introduced.
3. Run `docker compose build` (or pull the versioned image if your deployment
   uses one).
4. Run `docker compose up -d`; startup migrations are applied before the blog
   reports ready.
5. Check `docker compose ps`, then request `/healthz` and `/readyz` through
   Caddy.
6. Keep the prior image available until the health checks and a content smoke
   test succeed. Restore the backup only when rolling back data is necessary.
