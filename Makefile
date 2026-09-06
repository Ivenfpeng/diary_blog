NODE_BIN := $(shell brew --prefix node@24 2>/dev/null)/bin
NODE_ENV := PATH="$(NODE_BIN):$(PATH)"
GO_CACHE ?= /tmp/diary-blog-go-cache
BACKUP ?= /data/backups/backup.tar.gz
RESTORE ?= $(BACKUP)
RESTORE_SMOKE_CONTAINER ?= diary-blog-restore-smoke
RESTORE_SMOKE_VOLUME ?= diary_blog_restore_smoke
RESTORE_SMOKE_PORT ?= 18081
SMOKE_ARTICLE_PATH ?= /posts/release-gate-publishing-workflow
SMOKE_SEARCH_QUERY ?= durable

.PHONY: test-go test-admin test-e2e test vet admin-build sync-admin build release-gate container-release-gate dev container-build compose-up compose-down compose-ps compose-logs compose-health backup restore restore-smoke

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
	go build -o bin/blog ./cmd/blog

release-gate: test-go test-admin admin-build test-e2e vet

container-release-gate: container-build compose-up compose-health backup restore-smoke

dev:
	go run ./cmd/blog serve

container-build:
	docker compose build

compose-up:
	docker compose up -d

compose-down:
	docker compose down

compose-ps:
	docker compose ps

compose-logs:
	docker compose logs -f

compose-health:
	curl --fail http://localhost/healthz
	curl --fail http://localhost/readyz

backup:
	docker compose exec -T blog /app/blog backup --output $(BACKUP)

restore:
	docker compose stop blog
	docker compose run --rm --no-deps blog restore --input $(RESTORE) --force
	docker compose up -d blog

restore-smoke:
	tmp_dir=$$(mktemp -d); \
	cleanup() { docker rm -f $(RESTORE_SMOKE_CONTAINER) >/dev/null 2>&1 || true; docker volume rm -f $(RESTORE_SMOKE_VOLUME) >/dev/null 2>&1 || true; rm -rf "$$tmp_dir"; }; \
	trap cleanup EXIT; \
	docker rm -f $(RESTORE_SMOKE_CONTAINER) >/dev/null 2>&1 || true; \
	docker volume rm -f $(RESTORE_SMOKE_VOLUME) >/dev/null 2>&1 || true; \
	docker volume create $(RESTORE_SMOKE_VOLUME); \
	docker compose cp blog:$(RESTORE) "$$tmp_dir/release-smoke.tar.gz"; \
	docker compose run --rm --no-deps -v $(RESTORE_SMOKE_VOLUME):/data -v "$$tmp_dir:/restore:ro" blog restore --input /restore/release-smoke.tar.gz --force; \
	image_id=$$(docker compose images -q blog); \
	test -n "$$image_id"; \
	docker run -d --rm --name $(RESTORE_SMOKE_CONTAINER) -p 127.0.0.1:$(RESTORE_SMOKE_PORT):8080 -v $(RESTORE_SMOKE_VOLUME):/data -e BLOG_ADDR=:8080 -e BLOG_DATA_DIR=/data/site -e BLOG_PUBLIC_URL=http://localhost:$(RESTORE_SMOKE_PORT) "$$image_id" serve; \
	for _ in $$(seq 1 40); do curl --fail -s http://127.0.0.1:$(RESTORE_SMOKE_PORT)/readyz >/dev/null && break; sleep 1; done; \
	curl --fail -s http://127.0.0.1:$(RESTORE_SMOKE_PORT)/readyz >/dev/null; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)$(SMOKE_ARTICLE_PATH)" > "$$tmp_dir/article.html"; \
	grep -q "$(SMOKE_SEARCH_QUERY)" "$$tmp_dir/article.html"; \
	media_path=$$(grep -Eo '/media/[^" ]+' "$$tmp_dir/article.html" | head -n 1); \
	test -n "$$media_path"; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)$$media_path" >/dev/null; \
	curl --fail -s "http://127.0.0.1:$(RESTORE_SMOKE_PORT)/search?q=$(SMOKE_SEARCH_QUERY)" | grep -q "$(SMOKE_ARTICLE_PATH)"
