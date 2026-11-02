# diary_blog 发布与部署操作手册

本文记录从代码修改到线上发布的标准流程。默认仓库为
`Ivenfpeng/diary_blog`，开发分支为 `codex/diary-blog-mvp`，生产服务器使用
Docker Compose；本地没有 Docker 时可以将命令中的 `docker`/`docker compose`
替换为 Podman/`podman-compose`。

## 一、发布流程总览

```text
修改代码 → 本地测试与构建 → 推送开发分支 → 创建并合并 PR 到 main
→ GitHub Actions 构建 amd64/arm64 镜像并推送 GHCR
→ 服务器拉取固定镜像 → Compose 重启 → 健康检查
```

推荐生产使用版本号或 commit SHA 标签，不要长期依赖 `latest`，这样可以
快速回滚到上一个已验证版本。

## 二、修改代码后的本地检查

```sh
git switch codex/diary-blog-mvp
git pull --ff-only origin codex/diary-blog-mvp

make test
make vet
make build
git diff --check
```

涉及前端、登录、文章编辑或公开页面时，再执行完整发布门禁：

```sh
make release-gate
```

如果本机已有服务占用 E2E 默认端口：

```sh
BLOG_E2E_PORT=18082 npm run test:e2e
```

源码容器验证：

```sh
make container-build
make compose-up-http
make compose-health COMPOSE_HEALTH_URL=http://localhost:18080
```

本地使用 Podman：

```sh
GOPROXY=https://goproxy.cn,direct \
make container-build CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman

BLOG_PUBLIC_URL=http://localhost:18080 \
BLOG_SITE_ADDRESS=http://localhost \
BLOG_HTTP_PORT=18080 BLOG_HTTPS_PORT=18443 \
make deploy-up-http CONTAINER_COMPOSE=podman-compose CONTAINER_RUNTIME=podman

make compose-health COMPOSE_HEALTH_URL=http://localhost:18080
```

## 三、提交代码并创建 PR

只提交本次功能相关文件，不要把 `.metrics/`、本地数据库、构建产物和个人
配置提交进去：

```sh
git status --short
git add <本次修改的文件>
git diff --cached --check
git commit -m "feat: 简述本次变更"
git push -u origin codex/diary-blog-mvp
```

在 GitHub 创建：

```text
codex/diary-blog-mvp → main
```

PR 描述建议包含变更内容、测试命令、部署影响、数据库/配置变化和回滚方式。

## 四、镜像构建与 GHCR 发布

`.github/workflows/publish-image.yml` 会在推送到 `main`、
`codex/diary-blog-mvp`、推送 `v*` tag，或手动执行 workflow 时运行。Actions
使用 `GITHUB_TOKEN` 登录 GHCR，构建 `linux/amd64` 和 `linux/arm64`。

### 标签规则

| 触发方式 | 常见镜像标签 |
| --- | --- |
| 推送开发分支 | `codex-diary-blog-mvp`、`sha-<commit>` |
| 合并到默认分支 `main` | `latest`、`main`、`sha-<commit>` |
| 推送版本 tag | `v0.1.0`、`sha-<commit>` |

开发分支推送后直接拉取 `:latest` 可能出现 `not found`；要发布 `latest`，
需要合并到 `main`，或者临时使用开发分支生成的标签。

### Podman 手动发布多架构镜像

GHCR 令牌必须具有 `write:packages` 权限；私有 package 的服务器还需要
`read:packages`。

```sh
export IMAGE=ghcr.io/ivenfpeng/diary_blog
export TAG=v0.1.0
export MANIFEST=localhost/diary_blog:release

podman login ghcr.io -u Ivenfpeng
podman manifest rm "$MANIFEST" 2>/dev/null || true
podman manifest create "$MANIFEST"

podman build --platform linux/amd64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --manifest "$MANIFEST" .
podman build --platform linux/arm64 \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --manifest "$MANIFEST" .

podman manifest push --all "$MANIFEST" "docker://$IMAGE:$TAG"
podman manifest push --all "$MANIFEST" "docker://$IMAGE:latest"
```

如果 `go mod download` 出现 `storage.googleapis.com` TLS timeout/EOF，保留
`GOPROXY=https://goproxy.cn,direct`。跨架构构建失败时，检查 Podman machine
是否启用了 QEMU/binfmt。

## 五、服务器首次部署

服务器不需要下载完整源码，只需要 Compose 和 Caddy 配置文件：

```sh
mkdir -p ~/diary_blog/deploy
cd ~/diary_blog
GIT_REF=main

curl -fsSL -o compose.deploy.yaml \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/compose.deploy.yaml"
curl -fsSL -o deploy/Caddyfile \
  "https://raw.githubusercontent.com/Ivenfpeng/diary_blog/${GIT_REF}/deploy/Caddyfile"
head -n 5 compose.deploy.yaml
head -n 5 deploy/Caddyfile
```

`.env` 示例：

```dotenv
BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:v0.1.0
BLOG_PUBLIC_URL=https://ivenpeng.top
BLOG_SITE_ADDRESS=ivenpeng.top
CADDY_EMAIL=your-email@example.com
BLOG_HTTP_PORT=80
BLOG_HTTPS_PORT=443
```

公网自动 HTTPS：

```sh
docker login ghcr.io -u Ivenfpeng   # 私有 package 才需要
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
```

HTTP-only 内网部署：

```sh
docker compose -f compose.deploy.yaml pull
docker compose -f compose.deploy.yaml up -d
```

## 六、已有服务器升级发布

先备份，再把 `.env` 中的 `BLOG_IMAGE` 改为新版本 tag：

```sh
cd ~/diary_blog
cp .env ".env.backup.$(date +%F-%H%M%S)"

docker compose -f compose.deploy.yaml -f compose.https-auto.yaml exec -T blog \
  /app/blog backup --output "/data/backups/pre-deploy-$(date +%F-%H%M%S).tar.gz"

docker compose -f compose.deploy.yaml -f compose.https-auto.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml ps
```

使用 `latest` 时也必须执行 `pull`，否则可能继续使用本地旧镜像。不要执行
`down -v`，它会删除 `blog_data` 数据卷。

## 七、发布后验证

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml ps
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml logs --tail=100 blog
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml logs --tail=100 caddy

curl -fsS -o /dev/null -w 'healthz %{http_code}\n' https://ivenpeng.top/healthz
curl -fsS -o /dev/null -w 'home %{http_code}\n' https://ivenpeng.top/
curl -fsS -o /dev/null -w 'admin %{http_code}\n' https://ivenpeng.top/admin/
```

预期结果：健康检查 `204`，首页和管理台 `200`，blog 容器状态为 `healthy`。
前端样式更新后如浏览器仍显示旧内容，请执行 `Ctrl/Cmd + Shift + R`。

## 八、回滚

```sh
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml images
sed -i 's#^BLOG_IMAGE=.*#BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:v0.0.9#' .env
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml pull
docker compose -f compose.deploy.yaml -f compose.https-auto.yaml up -d
```

应用回滚不会自动回滚数据库内容；涉及迁移时，先确认备份可恢复，再执行
README 中的 restore 流程。

## 九、管理员密码与数据安全

系统只保存 Argon2id 密码哈希，无法查看当前明文密码。忘记密码时重置：

```sh
printf '%s\n' '换成强密码' | \
  docker compose -f compose.deploy.yaml -f compose.https-auto.yaml exec -T blog \
  /app/blog admin reset-password --username admin
```

文章永久删除只适用于已归档文章；仍被文章引用的媒体会被拒绝删除，避免
公开页面出现图片 404。生产操作前先备份，并保留最近一次可用镜像和配置。

## 十、一键发布脚本

仓库提供 `scripts/release-remote.sh`，用于把当前工作区直接发布到远端
Compose 服务器。脚本不会自动 commit、push Git，也不会删除本地文件；它会：

1. 检查工作区和运行 `make test`、`make vet`；
2. 使用 Docker Buildx 或 Podman 同时构建 `linux/amd64`、`linux/arm64` 镜像并推送 GHCR；
3. 通过 SSH 备份远端 `.env` 和站点数据；
4. 把远端 `BLOG_IMAGE` 切换到本次不可变 tag；
5. 执行 `docker compose pull/up`；
6. 检查容器健康、`/healthz`、首页和管理台。

首次配置：

```sh
docker login ghcr.io -u Ivenfpeng  # 有 Docker 时
# 本机只有 Podman 时：podman login ghcr.io -u Ivenfpeng
ssh-copy-id root@167.179.105.202  # 或提前配置好 SSH 公钥登录
./scripts/release-remote.sh --help
```

默认发布命令：

```sh
./scripts/release-remote.sh
```

默认值适配当前服务器：

```text
REMOTE_HOST=167.179.105.202
REMOTE_DIR=/root/diary_blog
REMOTE_COMPOSE_FILES=-f compose.deploy.yaml
REMOTE_HEALTH_URL=https://ivenpeng.top
```

公网 HTTPS 使用 override 时：

```sh
REMOTE_COMPOSE_FILES='-f compose.deploy.yaml -f compose.https-auto.yaml' \
./scripts/release-remote.sh
```

本机使用 Podman 构建：

```sh
IMAGE_REPOSITORY=ghcr.io/ivenfpeng/diary_blog \
GOPROXY=https://goproxy.cn,direct \
./scripts/release-remote.sh
```

脚本默认发布多架构 manifest，也可以显式指定平台：

```sh
TARGET_PLATFORMS=linux/amd64,linux/arm64 ./scripts/release-remote.sh
```

如果只需要临时发布单架构镜像：

```sh
TARGET_PLATFORMS=linux/amd64 ./scripts/release-remote.sh
```

脚本默认拒绝未提交源码修改，避免误发布半成品；本地生成的 `bin/blog` 和
`.metrics/` 会自动忽略。如果明确要发布当前未提交源码修改：

```sh
ALLOW_DIRTY=1 ./scripts/release-remote.sh
```

跳过本地检查或发布前备份（不建议生产使用）：

```sh
SKIP_CHECKS=1 BACKUP_BEFORE_DEPLOY=0 ./scripts/release-remote.sh
```

脚本每次使用类似 `deploy-20261102230500-abc1234` 的固定 tag。回滚时只需
把远端 `.env` 的 `BLOG_IMAGE` 改回上一个 tag，再执行：

```sh
ssh root@167.179.105.202 \
  "cd /root/diary_blog && sed -i 's#^BLOG_IMAGE=.*#BLOG_IMAGE=ghcr.io/ivenfpeng/diary_blog:deploy-previous#' .env && docker compose -f compose.deploy.yaml pull blog && docker compose -f compose.deploy.yaml up -d blog caddy"
```
