NODE_BIN := $(shell brew --prefix node@24 2>/dev/null)/bin
NODE_ENV := PATH="$(NODE_BIN):$(PATH)"

.PHONY: test-go test-admin test admin-build build dev

test-go:
	go test ./...

test-admin:
	$(NODE_ENV) npm --prefix admin test

test: test-go test-admin

admin-build:
	$(NODE_ENV) npm --prefix admin run build

build: admin-build
	go build -o bin/blog ./cmd/blog

dev:
	go run ./cmd/blog serve
