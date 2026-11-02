#!/usr/bin/env bash
set -Eeuo pipefail

# Build the current worktree, push an immutable image tag, then upgrade a
# remote Compose deployment over SSH. The script never commits or deletes files.

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'USAGE'
一键发布当前工作区到远端 Compose 服务器。

首次使用：先登录 GHCR，并确认本机 SSH 无密码连接远端服务器。
  docker login ghcr.io -u Ivenfpeng

常用环境变量：
  REMOTE_HOST             远端 IP/域名，默认 167.179.105.202
  REMOTE_DIR              远端部署目录，默认 /root/diary_blog
  REMOTE_COMPOSE_FILES    Compose 参数，默认 '-f compose.deploy.yaml'
  REMOTE_HEALTH_URL       发布后检查地址，默认 https://ivenpeng.top
  RELEASE_TAG             镜像标签，默认 deploy-时间-commit
  ALLOW_DIRTY=1           允许发布未提交工作区
  SKIP_CHECKS=1           跳过本地 make test/make vet
  BACKUP_BEFORE_DEPLOY=0  不执行发布前数据备份
USAGE
  exit 0
fi

REMOTE_HOST=${REMOTE_HOST:-167.179.105.202}
REMOTE_USER=${REMOTE_USER:-root}
REMOTE_DIR=${REMOTE_DIR:-/root/diary_blog}
REMOTE_COMPOSE_FILES=${REMOTE_COMPOSE_FILES:--f compose.deploy.yaml}
REMOTE_HEALTH_URL=${REMOTE_HEALTH_URL:-https://ivenpeng.top}
IMAGE_REPOSITORY=${IMAGE_REPOSITORY:-ghcr.io/ivenfpeng/diary_blog}
GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
BACKUP_BEFORE_DEPLOY=${BACKUP_BEFORE_DEPLOY:-1}
SKIP_CHECKS=${SKIP_CHECKS:-0}

SHORT_SHA=$(git rev-parse --short HEAD 2>/dev/null || printf 'nogit')
if git diff --quiet && git diff --cached --quiet; then
  TREE_SUFFIX=""
else
  TREE_SUFFIX="-dirty"
  if [[ "${ALLOW_DIRTY:-0}" != "1" ]]; then
    echo "工作区存在未提交修改。确认要发布当前工作区时，请设置 ALLOW_DIRTY=1。" >&2
    exit 1
  fi
  echo "警告：将发布当前未提交工作区，镜像标签会带 dirty。" >&2
fi

RELEASE_TAG=${RELEASE_TAG:-"deploy-$(date +%Y%m%d%H%M%S)-${SHORT_SHA}${TREE_SUFFIX}"}
IMAGE="${IMAGE_REPOSITORY}:${RELEASE_TAG}"
REMOTE="${REMOTE_USER}@${REMOTE_HOST}"

log() { printf '\n==> %s\n' "$*"; }
die() { echo "发布失败：$*" >&2; exit 1; }

command -v ssh >/dev/null || die "未找到 ssh"
command -v git >/dev/null || die "未找到 git"

if [[ "$SKIP_CHECKS" != "1" ]]; then
  log "运行本地检查"
  make test
  make vet
  git diff --check
fi

log "构建并推送 ${IMAGE}"
if command -v docker >/dev/null && docker buildx version >/dev/null 2>&1; then
  docker buildx build \
    --platform linux/amd64 \
    --build-arg GOPROXY="$GOPROXY" \
    --tag "$IMAGE" \
    --push \
    .
elif command -v podman >/dev/null; then
  podman build \
    --platform linux/amd64 \
    --build-arg GOPROXY="$GOPROXY" \
    --tag "$IMAGE" \
    .
  podman push "$IMAGE" "docker://${IMAGE}"
else
  die "需要 Docker Buildx 或 Podman"
fi

log "检查远端 Compose 配置"
ssh "$REMOTE" "set -eu; test -d '$REMOTE_DIR'; test -f '$REMOTE_DIR/.env'; command -v docker >/dev/null; docker compose version >/dev/null"

log "更新远端镜像并重启"
ssh "$REMOTE" "set -eu
cd '$REMOTE_DIR'
cp .env .env.backup-release-\$(date +%Y%m%d%H%M%S)
if [ '$BACKUP_BEFORE_DEPLOY' = 1 ]; then
  docker compose $REMOTE_COMPOSE_FILES exec -T blog /app/blog backup --output /data/backups/pre-deploy-\$(date +%Y%m%d%H%M%S).tar.gz
fi
if grep -q '^BLOG_IMAGE=' .env; then
  sed -i 's#^BLOG_IMAGE=.*#BLOG_IMAGE=$IMAGE#' .env
else
  printf '\\nBLOG_IMAGE=$IMAGE\\n' >> .env
fi
docker compose $REMOTE_COMPOSE_FILES pull blog
docker compose $REMOTE_COMPOSE_FILES up -d blog caddy
docker compose $REMOTE_COMPOSE_FILES ps"

log "验证远端服务"
ssh "$REMOTE" "set -eu
cd '$REMOTE_DIR'
docker compose $REMOTE_COMPOSE_FILES ps --status running | grep -q 'blog'
docker inspect --format '{{.State.Health.Status}}' diary_blog-blog-1 | grep -q healthy"
curl -fsS -o /dev/null -w 'healthz %{http_code}\n' "${REMOTE_HEALTH_URL%/}/healthz"
curl -fsS -o /dev/null -w 'home %{http_code}\n' "${REMOTE_HEALTH_URL%/}/"
curl -fsS -o /dev/null -w 'admin %{http_code}\n' "${REMOTE_HEALTH_URL%/}/admin/"

printf '\n发布完成：%s\n' "$IMAGE"
