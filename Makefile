NODE_BIN := $(shell brew --prefix node@24 2>/dev/null)/bin
NODE_ENV := PATH="$(NODE_BIN):$(PATH)"
GO_CACHE ?= /tmp/diary-blog-go-cache
CONTAINER_COMPOSE ?= docker compose
CONTAINER_RUNTIME ?= docker
COMPOSE_FILES ?= -f compose.yaml
DEPLOY_COMPOSE_FILES ?= -f compose.deploy.yaml
BLOG_IMAGE ?= ghcr.io/ivenfpeng/diary_blog:latest
BACKUP ?= /data/backups/backup.tar.gz
RESTORE ?= $(BACKUP)
RESTORE_SMOKE_CONTAINER ?= diary-blog-restore-smoke
RESTORE_SMOKE_VOLUME ?= diary_blog_restore_smoke
RESTORE_SMOKE_STAGING_VOLUME ?= diary_blog_restore_smoke_staging
RESTORE_SMOKE_PORT ?= 18081
SMOKE_ARTICLE_PATH ?= /posts/release-gate-publishing-workflow
SMOKE_SEARCH_QUERY ?= durable
COMPOSE_HEALTH_URL ?= http://localhost

export BLOG_IMAGE
export BLOG_PUBLIC_URL
export BLOG_SITE_ADDRESS
export BLOG_HTTP_PORT
export BLOG_HTTPS_PORT
export CADDY_EMAIL
export TLS_CERTS_DIR
export TLS_CERT_FILE
export TLS_KEY_FILE

.PHONY: test-go test-admin test-e2e test vet admin-build sync-admin build release-gate container-release-gate dev container-build compose-up compose-up-http compose-up-https-auto compose-up-https-files deploy-pull deploy-up-http deploy-up-https-auto deploy-up-https-files compose-down compose-ps compose-logs compose-health backup restore restore-smoke

test-go:
	GOCACHE=$(GO_CACHE) go test ./...

test-admin:
	$(NODE_ENV) npm --prefix admin test

test-e2e:
	$(NODE_ENV) npm run test:e2e

test: test-go test-admin

vet:
	GOCACHE=$(GO_CACHE) go vet ./...

admin-build:
	$(NODE_ENV) npm --prefix admin run build

sync-admin:
	rm -rf web/admin
	cp -R admin/dist web/admin

build: admin-build sync-admin
	GOCACHE=$(GO_CACHE) go build -o bin/blog ./cmd/blog

release-gate: test-go test-admin admin-build test-e2e vet

container-release-gate: container-build compose-up compose-health backup restore-smoke

dev:
	go run ./cmd/blog serve

container-build:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) build

compose-up:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) up -d

compose-up-http:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) up -d

compose-up-https-auto:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) -f compose.https-auto.yaml up -d

compose-up-https-files:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) -f compose.https-files.yaml up -d

deploy-pull:
	$(CONTAINER_COMPOSE) $(DEPLOY_COMPOSE_FILES) pull

deploy-up-http:
	$(CONTAINER_COMPOSE) $(DEPLOY_COMPOSE_FILES) up -d

deploy-up-https-auto:
	$(CONTAINER_COMPOSE) $(DEPLOY_COMPOSE_FILES) -f compose.https-auto.yaml up -d

deploy-up-https-files:
	$(CONTAINER_COMPOSE) $(DEPLOY_COMPOSE_FILES) -f compose.https-files.yaml up -d

compose-down:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) down

compose-ps:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) ps

compose-logs:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) logs -f

compose-health:
	curl --fail $(COMPOSE_HEALTH_URL)/healthz
	curl --fail $(COMPOSE_HEALTH_URL)/readyz

backup:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) exec -T blog /app/blog backup --output $(BACKUP)

restore:
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) stop blog
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) run --rm --no-deps blog restore --input $(RESTORE) --force
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) up -d blog

restore-smoke:
	set -eu; \
	tmp_dir=$$(mktemp -d); \
	cleanup() { $(CONTAINER_RUNTIME) rm -f $(RESTORE_SMOKE_CONTAINER) >/dev/null 2>&1 || true; $(CONTAINER_RUNTIME) volume rm -f $(RESTORE_SMOKE_VOLUME) >/dev/null 2>&1 || true; $(CONTAINER_RUNTIME) volume rm -f $(RESTORE_SMOKE_STAGING_VOLUME) >/dev/null 2>&1 || true; rm -rf "$$tmp_dir"; }; \
	trap cleanup EXIT; \
	$(CONTAINER_RUNTIME) rm -f $(RESTORE_SMOKE_CONTAINER) >/dev/null 2>&1 || true; \
	$(CONTAINER_RUNTIME) volume rm -f $(RESTORE_SMOKE_VOLUME) >/dev/null 2>&1 || true; \
	$(CONTAINER_RUNTIME) volume rm -f $(RESTORE_SMOKE_STAGING_VOLUME) >/dev/null 2>&1 || true; \
	$(CONTAINER_RUNTIME) volume create $(RESTORE_SMOKE_VOLUME) >/dev/null; \
	$(CONTAINER_RUNTIME) volume create $(RESTORE_SMOKE_STAGING_VOLUME) >/dev/null; \
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) run --rm --no-deps --user 0 --entrypoint /bin/sh -e RESTORE_SOURCE=$(RESTORE) -v $(RESTORE_SMOKE_STAGING_VOLUME):/staging blog -ec 'cp "$$RESTORE_SOURCE" /staging/release-smoke.tar.gz; chmod 0444 /staging/release-smoke.tar.gz'; \
	$(CONTAINER_COMPOSE) $(COMPOSE_FILES) run --rm --no-deps -v $(RESTORE_SMOKE_VOLUME):/data -v $(RESTORE_SMOKE_STAGING_VOLUME):/restore:ro blog restore --input /restore/release-smoke.tar.gz --force; \
	image_id=$$($(CONTAINER_COMPOSE) $(COMPOSE_FILES) images -q blog); \
	test -n "$$image_id"; \
	$(CONTAINER_RUNTIME) run -d --rm --name $(RESTORE_SMOKE_CONTAINER) -p 127.0.0.1:$(RESTORE_SMOKE_PORT):8080 -v $(RESTORE_SMOKE_VOLUME):/data -e BLOG_ADDR=:8080 -e BLOG_DATA_DIR=/data/site -e BLOG_PUBLIC_URL=http://localhost:$(RESTORE_SMOKE_PORT) "$$image_id" serve; \
	for _ in $$(seq 1 40); do curl --fail -s http://127.0.0.1:$(RESTORE_SMOKE_PORT)/readyz >/dev/null && break; sleep 1; done; \
	curl --fail -s http://127.0.0.1:$(RESTORE_SMOKE_PORT)/readyz >/dev/null; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)$(SMOKE_ARTICLE_PATH)" > "$$tmp_dir/article.html"; \
	grep -q "$(SMOKE_SEARCH_QUERY)" "$$tmp_dir/article.html"; \
	grep -Eo '/media/[^" ]+' "$$tmp_dir/article.html" > "$$tmp_dir/media-paths.txt"; \
	media_path=$$(head -n 1 "$$tmp_dir/media-paths.txt"); \
	test -n "$$media_path"; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)$$media_path" >/dev/null; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)/search?q=$(SMOKE_SEARCH_QUERY)" > "$$tmp_dir/search.html"; \
	grep -q "$(SMOKE_ARTICLE_PATH)" "$$tmp_dir/search.html"
