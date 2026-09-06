NODE_BIN := $(shell brew --prefix node@24 2>/dev/null)/bin
NODE_ENV := PATH="$(NODE_BIN):$(PATH)"
GO_CACHE ?= /tmp/diary-blog-go-cache
BACKUP ?= /data/backups/backup.tar.gz
RESTORE ?= $(BACKUP)

.PHONY: test-go test-admin test-e2e test vet admin-build sync-admin build release-gate dev container-build compose-up compose-down compose-ps compose-logs backup restore

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

backup:
	docker compose exec -T blog /app/blog backup --output $(BACKUP)

restore:
	docker compose stop blog
	docker compose run --rm --no-deps blog restore --input $(RESTORE) --force
	docker compose up -d blog
