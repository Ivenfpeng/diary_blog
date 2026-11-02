#!/usr/bin/env bash
set -Eeuo pipefail

# Build the current worktree, push an immutable image tag, then upgrade a
# remote Compose deployment over SSH. The script never commits or deletes files.

if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
  echo "请直接执行 ./scripts/release-remote.sh，不要使用 source 加载脚本。" >&2
  return 0 2>/dev/null
fi

export GIT_TERMINAL_PROMPT=0
exec </dev/null

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'USAGE'
一键发布当前工作区到远端 Compose 服务器。

首次使用：先登录 GHCR，并确认本机 SSH 无密码连接远端服务器。
  docker login ghcr.io -u Ivenfpeng
  # 本机没有 Docker 时使用：
  podman login ghcr.io -u Ivenfpeng

常用环境变量：
  REMOTE_HOST             远端 IP/域名，默认 167.179.105.202
  REMOTE_DIR              远端部署目录，默认 /root/diary_blog
  REMOTE_COMPOSE_FILES    Compose 参数，默认 '-f compose.deploy.yaml'
  REMOTE_HEALTH_URL       发布后检查地址，默认 https://ivenpeng.top
  TARGET_PLATFORMS        构建平台，默认 linux/amd64,linux/arm64
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
TARGET_PLATFORMS=${TARGET_PLATFORMS:-linux/amd64,linux/arm64}
GOPROXY=${GOPROXY:-https://goproxy.cn,direct}
BACKUP_BEFORE_DEPLOY=${BACKUP_BEFORE_DEPLOY:-1}
SKIP_CHECKS=${SKIP_CHECKS:-0}

SHORT_SHA=$(git rev-parse --short HEAD 2>/dev/null || printf 'nogit')
DIRTY_FILES=$(git status --porcelain=v1 | awk '{ path = substr($0, 4); if (path == "bin/blog" || path ~ /^\.metrics\//) next; print }')
if [[ -z "$DIRTY_FILES" ]]; then
  TREE_SUFFIX=""
else
  TREE_SUFFIX="-dirty"
  if [[ "${ALLOW_DIRTY:-0}" != "1" ]]; then
    printf '工作区存在未提交代码修改：\n%s\n' "$DIRTY_FILES" >&2
    echo "确认要发布当前工作区时，请设置 ALLOW_DIRTY=1。" >&2
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
    --platform "$TARGET_PLATFORMS" \
    --build-arg GOPROXY="$GOPROXY" \
    --tag "$IMAGE" \
    --push \
    .
elif command -v podman >/dev/null; then
  MANIFEST="localhost/diary_blog:release-${RELEASE_TAG}"
  podman manifest rm "$MANIFEST" 2>/dev/null || true
  podman manifest create "$MANIFEST"
  IFS=',' read -r -a platforms <<< "$TARGET_PLATFORMS"
  for platform in "${platforms[@]}"; do
    podman build \
      --platform "$platform" \
      --build-arg GOPROXY="$GOPROXY" \
      --manifest "$MANIFEST" \
      .
  done
  podman manifest push --all "$MANIFEST" "docker://${IMAGE}"
else
  die "需要 Docker Buildx 或 Podman"
fi

log "检查远端 Compose 配置"
ssh -T -o BatchMode=yes "$REMOTE" "set -eu; test -d '$REMOTE_DIR'; test -f '$REMOTE_DIR/.env'; command -v docker >/dev/null; docker compose version >/dev/null"

log "更新远端镜像并重启"
ssh -T -o BatchMode=yes "$REMOTE" "set -eu
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
ssh -T -o BatchMode=yes "$REMOTE" "set -eu
cd '$REMOTE_DIR'
docker compose $REMOTE_COMPOSE_FILES ps --status running | grep -q 'blog'
docker inspect --format '{{.State.Health.Status}}' diary_blog-blog-1 | grep -q healthy"
curl -fsS -o /dev/null -w 'healthz %{http_code}\n' "${REMOTE_HEALTH_URL%/}/healthz"
curl -fsS -o /dev/null -w 'home %{http_code}\n' "${REMOTE_HEALTH_URL%/}/"
curl -fsS -o /dev/null -w 'admin %{http_code}\n' "${REMOTE_HEALTH_URL%/}/admin/"

printf '\n发布完成：%s\n' "$IMAGE"
