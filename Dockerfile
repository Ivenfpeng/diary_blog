# syntax=docker/dockerfile:1

FROM node:24-alpine AS admin-build
WORKDIR /src/admin

COPY admin/package.json admin/package-lock.json ./
RUN npm ci

COPY admin/ ./
RUN npm run build

FROM golang:1.27-alpine AS go-build
WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org

COPY go.mod go.sum ./
RUN GOPROXY="${GOPROXY}" GOSUMDB="${GOSUMDB}" go mod download

COPY . ./
COPY --from=admin-build /src/admin/dist ./web/admin

RUN GOCACHE=/tmp/go-build-cache go test ./... \
    && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/blog ./cmd/blog

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S blog \
    && adduser -S -G blog -h /app blog \
    && mkdir -p /data/site /data/backups \
    && chown -R blog:blog /data

WORKDIR /app
COPY --from=go-build /out/blog /app/blog

ENV BLOG_ADDR=:8080 \
    BLOG_DATA_DIR=/data/site

VOLUME ["/data"]
EXPOSE 8080
USER blog

ENTRYPOINT ["/app/blog"]
CMD ["serve"]
