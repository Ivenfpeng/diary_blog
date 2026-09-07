# Diary Blog 开发进度台账

项目目录：`/Users/iven/work/program/diary_blog`

设计基线：`docs/superpowers/specs/2026-09-03-diary-blog-design.zh-CN.md`

实施计划：`docs/superpowers/plans/2026-09-03-diary-blog-mvp-implementation.md`

执行方式：按实施计划持续自动执行；每个 Task 完成实现、独立审查、修复与复验后提交

当前分支：`codex/diary-blog-mvp`

当前阶段：发布收尾

总体状态：最终整分支复审已通过；Podman 本地 HTTP-only 容器部署、image-only 部署与编辑器保存回归验证通过；HTTPS 部署模式待目标环境实测

最后登记：2026-09-07 16:53 CST

## 状态定义

| 状态 | 含义 |
|---|---|
| 未开始 | 尚未写入测试或实现 |
| 进行中 | 已进入红绿循环，但任务提交尚未完成 |
| 阻塞 | 因环境、依赖、设计或测试问题无法继续 |
| 待验收 | 实现任务已提交，等待监控任务独立验证 |
| 完成 | 提交存在，规定测试由监控任务独立验证通过 |

## 里程碑

| 里程碑 | 任务范围 | 状态 | 完成条件 |
|---|---:|---|---|
| M1 Core Foundation | Task 1-5 | 完成 | 数据库、Markdown、草稿、发布与搜索核心完成 |
| M2 Public Product | Task 6-7 | 完成 | 公开页面、搜索、RSS、Sitemap 和 SEO 可用 |
| M3 Authoring Product | Task 8-12 | 完成 | 登录、管理 API、Vue 写作后台和媒体管理可用 |
| M4 Operational Release | Task 13-16 | 完成 | 运维、备份、容器部署和 E2E 验收通过 |

## 任务登记

| Task | 内容 | 状态 | 提交 | 验证证据 | 阻塞/备注 |
|---:|---|---|---|---|---|
| 1 | 工具链与项目骨架 | 完成 | `9948a48` | Go、Vitest、Vue TypeScript 与 Vite 构建通过 | 已建立 Go/Vue 工具链 |
| 2 | SQLite 连接与迁移 | 完成 | `67426b3` | 数据库迁移测试及 Go 全量测试通过 | WAL、外键、超时与嵌入迁移完成 |
| 3 | Markdown 内容渲染 | 完成 | `43c2260` | Markdown、安全过滤、标题目录与阅读时间测试通过 | - |
| 4 | 草稿仓储与历史版本 | 完成 | `8b5d602` | 4 组仓储集成测试、Go/Vue 全量验证通过 | 乐观锁、标签替换、30 版保留与恢复完成 |
| 5 | 发布服务与 FTS5 搜索 | 完成 | `9ee624f` | Go race、vet、中文/标识符/混合搜索与事务回滚测试通过 | - |
| 6 | 公开 HTTP 与知识库 UI | 完成 | `1de5348`, `d4bef94`, `356c872` | 两轮独立审查后通过；Go 全量、race、vet 与 Vue 构建通过 | 使用不可变发布快照；旧 v2 文章升级后需重新发布 |
| 7 | 搜索、RSS、Sitemap、SEO | 完成 | `41d236e`, `7646a27`, `39913c5` | 两轮独立审查后通过；配置源地址、Host 污染防护及转义路径测试通过 | - |
| 8 | 管理员密码、会话与 CSRF | 完成 | `b355547`, `3079e2d`, `4aa0141`, `b8c0977`, `eb6d21b` | 四轮安全复审后通过；Go 全量、race、vet、登录限流与迁移专项测试通过 | 双阶段限流、单管理员、12 小时会话、Cookie/CSRF、密文存储完成 |
| 9 | 管理文章 API | 完成 | `36d66ad`, `9b413e3` | 一轮独立审查修复后通过；Go 全量、race、vet 与管理 API 契约测试通过 | create/get/list/autosave/preview/publish/archive/revisions/restore 已受会话与 CSRF 保护 |
| 10 | Vue 后台框架与登录 | 完成 | `2dc1bc6`, `ebc09c8`, `d3d5500` | 独立审查通过；`npm --prefix admin test`、`npm --prefix admin run build` 通过 | 已修正 `/admin` base 路由与匿名 restore 后登录跳转 |
| 11 | 文章列表与 Markdown 编辑器 | 完成 | `3b18f87`, `7c8cb32`, `3b9df88`, `7fa87a4` | 两轮独立审查修复后通过；`npm --prefix admin test`、`npm --prefix admin run build` 通过 | 已修正 Markdown 父级同步、发布校验与 destructive 操作期间未保存内容保护 |
| 12 | 分类、媒体与站点设置 | 完成 | `f857dca`, `2e37bc6`, `0f63e4b`, `d68ed8a`, `7710281`, `5ea64fb` | 四轮独立审查修复后通过；`go test ./...`、`go vet ./...`、`npm --prefix admin test`、`npm --prefix admin run build`、`git diff --check` 通过 | WebP 使用真实解码；AVIF 采用严格结构/属性/extent 校验而非像素级解码 |
| 13 | 日志、健康检查、缓存与停机 | 完成 | `aa78da2`, `b106691` | 一轮独立审查修复后通过；Task 13 targeted、`go test ./...`、`go vet ./...`、`git diff --check` 通过 | 已修正 panic 日志状态与 slug 变更后的旧文章缓存失效 |
| 14 | 备份、恢复、迁移与管理 CLI | 完成 | `7695e9e`, `5bd3db2`, `69ec676`, `e38ca9e`, `a454f28` | 两轮独立审查修复后通过；Task 14 targeted、`go test ./...`、`go vet ./...`、`git diff --check` 通过 | 已改用 SQLite online backup；恢复前校验 checksum、路径白名单、数据库 quick_check 与迁移版本；强制恢复具备 rollback；备份/恢复限制对齐 |
| 15 | 生产构建与 Docker/Podman Compose | 完成 | `aa80ca1`, `b52e4d0`, `e3c5117`, `a1128fa` | 两轮独立审查修复后通过；`npm --prefix admin test`、`npm --prefix admin run build`、`go test ./...`、`go vet ./...`、YAML parse、Makefile dry-run 与 `git diff --check` 通过；Podman Compose 本地 HTTP-only build/up/health/home/admin/backup 验证通过 | 已支持默认 HTTP-only、本地/内网部署、Caddy 自动 HTTPS、自带证书 HTTPS 和 Docker/Podman 命令切换；HTTPS 模式需在具备域名/证书的目标环境补跑 |
| 16 | E2E、响应式与发布门禁 | 完成 | `23a07e2`, `8bac5e8`, `da090f3` | 两轮独立审查修复后通过；`npm run test:e2e`、`go test ./...`、`npm --prefix admin test`、`go vet ./...`、`git diff --check`、Makefile dry-run 通过 | 已补全 draft/archived 全公开面排除矩阵、响应式重叠/遮挡/垂直裁切断言和 fresh-volume restore-smoke 门禁；Docker CLI 不在当前宿主机 PATH，容器实际运行门禁未执行 |

## 最终整体验收

| 范围 | 状态 | 提交 | 验证证据 | 备注 |
|---|---|---|---|---|
| Task 1-16 整分支复审 | 完成 | `45e48ab`, `083a638` | 最终独立 scoped re-review 通过；`make release-gate`、`make -n container-release-gate`、`make -n restore-smoke \| sh -n`、`git diff --check` 通过 | 修复了最终 review 的 7 个 Important 与 2 个 Minor；`083a638` 仅移除误跟踪的 SDD scratch 报告 |

## 当前实现快照

- Go：`go1.27.1 darwin/arm64`。
- Go 模块：`github.com/Ivenfpeng/diary_blog`。
- 已完成 SQLite schema/migrations、Markdown 渲染、草稿/版本、发布服务与 FTS5 搜索。
- 公开站点已使用 Go `html/template` SSR，具备首页、文章、分类、标签、归档、移动导航和公开错误页。
- 公开读取使用不可变发布快照；草稿编辑不会污染最后一次成功发布内容。
- 搜索页、RSS、Sitemap 与 SEO 已经通过独立审查并完成。
- 管理员认证已具备 Argon2id 密码、哈希会话、CSRF 双提交校验、双阶段登录限流与单管理员约束。
- 管理文章 JSON API 已完成，包含稳定错误 envelope、严格 JSON 解码、2 MiB body 限制、乐观锁冲突映射和显式 admin 路由。
- Vue 管理后台基础 shell 已完成，包含 typed API client、会话恢复、路由守卫、登录表单、固定侧栏和移动抽屉。
- 文章列表与 Markdown 编辑器已完成，包含 autosave 串行化、冲突态保护、CodeMirror 生命周期、预览、发布、归档和版本恢复。
- 分类、标签、媒体库与站点设置已完成，包含 CSRF 管理端点、SQLite 全局 taxonomy slug trigger、10 MiB 媒体上传边界、WebP 解码校验、AVIF 结构校验、媒体 alt text 更新和管理视图 edit/delete 对话框。
- 运行期能力已完成，包含 JSON slog、请求 ID、访问日志 status/bytes/route pattern、panic recovery、readyz 存储检查、进程内公开页/RSS/Sitemap 缓存、发布/归档缓存失效和 10 秒优雅停机。
- 备份、恢复、迁移与管理 CLI 已完成，包含 SQLite online backup、`.tar.gz` manifest/checksum、恢复前数据库/迁移验证、强制恢复 rollback、`backup`/`restore`/`migrate`/`admin reset-password`/`search rebuild` 子命令，以及备份输出 media/symlink 防护。
- 生产容器化已完成，包含 Node 24 + Go 1.27 多阶段 Dockerfile、GHCR 镜像发布 workflow、源码构建 Compose、image-only 部署 Compose、非 root runtime、Compose Blog/Caddy 拓扑、Docker/Podman Compose 命令切换、默认本地/内网 HTTP-only、Caddy 自动 HTTPS、自带 PEM 证书 HTTPS、Caddy 压缩/安全头/10MiB 上传限制、`/data/site` 应用目录、`/data/backups` 备份路径和 restore-safe Makefile 目标。
- Podman 本地部署验证已完成：`podman-compose` 1.6.0 / Podman 5.8.3，`BLOG_HTTP_PORT=18080`、`BLOG_HTTPS_PORT=18443` 启动成功，`/healthz`、`/readyz`、首页、管理后台入口和容器内备份命令通过。
- image-only 部署验证已完成：`compose.deploy.yaml` 不包含 `build` 字段，使用 `BLOG_IMAGE=localhost/diary_blog_blog:latest` 直接启动，通过 `/healthz`、`/readyz` 与管理后台资源检查；生产镜像默认指向 `ghcr.io/ivenfpeng/diary_blog:latest`；README 已补充部署机只下载最小 Compose/Caddyfile 配置后执行 `docker compose pull && docker compose up -d` 的路径，并扩展 GHCR、Docker buildx、Podman 单架构/多架构 manifest、HTTPS、admin 密码、运维命令与排障说明。
- 管理后台保存体验已修复：非法 slug 会在前端提示并暂停 autosave，不再持续向 `/api/admin/posts/{id}` 发送必然失败的 PUT；slug 表单 `pattern` 已兼容浏览器 `v` flag；文章编辑器改用已存在分类下拉与标签复选框，避免手填不存在 ID 导致保存 400；不存在的分类、标签或封面媒体引用会由后端返回 `post_validation` 400，不再泄露为 500。
- E2E 与发布门禁已完成，包含 Playwright 完整写作流、draft/archived 在首页/分类/归档/搜索/RSS/Sitemap 的公开排除验证、桌面/平板/移动响应式重叠/遮挡/裁切断言、截图产物、非容器 release gate，以及 Compose fresh-volume restore-smoke 目标。
- 最终整分支复审修复已完成，包含有界/LRU/代际公共缓存、发布期间编辑锁、公共站点设置/RSS/SEO 消费与缓存失效、分页浏览与搜索、Linux 非 root 可读的 restore-smoke staging volume，以及 fail-fast restore-smoke 检查。
- 工作区仍包含 `.gitignore`、`.idea/`、`.metrics/` 用户改动；从本次起 `docs/progress/` 作为总控文档持续更新。

## 风险与偏差

| 编号 | 级别 | 状态 | 内容 | 处理方式 |
|---|---|---|---|---|
| R-001 | 低 | 已缓解 | npm 默认网络曾超时 | 依赖已安装；每个任务继续执行 Vitest、TypeScript 与 Vite 构建 |
| R-002 | 中 | 已缓解 | 早期实现直接位于 `main` | 已切换到 `codex/diary-blog-mvp` 特性分支持续开发 |
| R-003 | 低 | 已缓解 | Go 官方模块代理曾超时 | 依赖已缓存；测试使用独立可写 `GOCACHE` |
| R-004 | 中 | 已裁定 | v2 结构无法还原最后一次成功发布的完整元数据 | 迁移不自动创建公开快照；旧文章须显式重新发布，避免泄露草稿 |
| R-005 | 中 | 已解决 | 配置 URL 含转义路径前缀时 RSS/Sitemap 可能重复编码 | 已统一 URL Path/RawPath 处理并通过专项审查 |
| R-006 | 中 | 已解决 | Caddy 反向代理场景需显式配置可信代理 CIDR，否则登录限流会按直接 peer 计数 | Task 15 已通过 `BLOG_TRUSTED_PROXY_CIDRS` 接入运行时配置并在 Compose 中限定为 Caddy 固定地址；Docker 实测仍由 R-010/R-013 跟踪 |
| R-007 | 低 | 已裁定 | AVIF 上传校验当前验证 ISO-BMFF/AVIF 元数据、属性关联和数据 extent，不做 AV1 像素级解码 | Task 12 先采用严格结构门禁；若后续接入维护良好的 AVIF decoder，可替换为像素级验证 |
| R-008 | 低 | 已裁定 | 备份 manifest 写在 archive 最后，无法包含自身 checksum | Manifest 校验所有 payload entry；restore 拒绝 manifest 后额外条目和 payload checksum 不匹配 |
| R-009 | 中 | 已裁定 | 备份/恢复当前限制为 10,000 entries、单 entry 64 MiB、payload 总量 512 MiB、manifest 1 MiB | 首版用对称限制避免生成不可恢复备份；大站点后续需同步提高常量与测试 |
| R-010 | 中 | 已缓解 | 当前宿主机没有 `docker` 可执行文件 | 已使用本机 Podman 5.8.3 + `podman-compose` 1.6.0 平替完成默认 HTTP-only Compose build/up/health/home/admin/backup 验证；Docker CLI 本身仍未安装 |
| R-011 | 低 | 已裁定 | Compose 固定使用 `172.30.0.0/24` 内部网段和 Caddy `172.30.0.10/32` trusted proxy | README 说明如与宿主 Docker 网络冲突需同时调整 subnet 与 trusted proxy CIDR |
| R-012 | 中 | 待环境验证 | 当前宿主机无法下载 Playwright bundled Chromium，E2E 默认使用已安装 Chrome channel | CI 或干净开发机需先运行 `npx playwright install chromium` 并使用 `PLAYWRIGHT_BUNDLED_CHROMIUM=1 npm run test:e2e`，或安装可用 Chrome channel |
| R-013 | 中 | 部分验证 | Docker CLI 缺失导致 Docker 版 `container-release-gate` 与 `restore-smoke` 未实际执行 | Podman Compose 已验证 build/up/health/home/admin/backup；fresh-volume restore-smoke 仍需要先准备带媒体的发布 smoke 文章；HTTPS 自动证书和自带证书模式需在对应目标环境补跑 |
| R-014 | 低 | 已解决 | 编辑器 autosave 会把明显非法 slug 发给后端，浏览器控制台出现重复 400；不存在分类/标签/封面媒体引用曾被映射成 500 | 已增加前端字段提示与 autosave 暂停；后端将 SQLite 约束错误映射为 `post_validation`；Podman 部署重建后验证通过 |

## 变更记录

| 时间 | 事件 |
|---|---|
| 2026-09-03 17:17 CST | 执行任务创建基础实施计划。 |
| 2026-09-03 17:27 CST | Go 1.27.1 安装完成，配置模块完成红绿循环。 |
| 2026-09-03 17:32 CST | chi 与 modernc SQLite 通过国内代理下载成功。 |
| 2026-09-03 17:35 CST | 监控任务独立运行 `go test ./...`，当前 Go 测试通过；Task 1 因 npm 安装仍未完成。 |
| 2026-09-04 09:42 CST | Task 4 完成并提交草稿仓储、乐观锁、版本历史和恢复。 |
| 2026-09-04 09:51 CST | Task 5 完成并提交发布服务、FTS5 搜索及事务回滚保障。 |
| 2026-09-04 10:16 CST | Task 6 经两轮修复与独立审查通过；公开内容改用不可变发布快照。 |
| 2026-09-04 10:20 CST | Task 7 首轮提交完成；正在修复配置 URL 转义路径的重复编码。 |
| 2026-09-04 11:05 CST | Task 7 经两轮审查通过；M2 Public Product 完成，进入 Task 8。 |
| 2026-09-04 16:17 CST | Task 8 经四轮安全复审通过；管理员认证、会话、CSRF 与限流完成，进入 Task 9。 |
| 2026-09-04 16:32 CST | Task 9 经一轮审查修复通过；管理文章 API 完成，进入 Task 10。 |
| 2026-09-04 16:47 CST | Task 10 独立审查通过；Vue 后台 shell 与登录完成，进入 Task 11。 |
| 2026-09-05 09:46 CST | Task 11 经两轮审查修复通过；文章列表与 Markdown 编辑器完成，进入 Task 12。 |
| 2026-09-05 22:00 CST | Task 12 经四轮审查修复通过；分类、媒体上传与站点设置完成，M3 完成并进入 Task 13。 |
| 2026-09-05 22:25 CST | Task 13 经一轮审查修复通过；运行期日志、健康检查、缓存与优雅停机完成，进入 Task 14。 |
| 2026-09-06 08:28 CST | Task 14 经两轮审查修复通过；备份、恢复、迁移和管理 CLI 完成，进入 Task 15。 |
| 2026-09-06 08:50 CST | Task 15 经两轮审查修复通过；生产 Dockerfile、Compose、Caddy 与部署文档完成，进入 Task 16。 |
| 2026-09-06 19:34 CST | Task 16 经两轮审查修复通过；E2E、响应式验证、非容器 release gate 与 Compose restore-smoke 命令完成，M4 完成并进入最终整分支复审。 |
| 2026-09-07 09:41 CST | 最终整分支复审修复波通过 scoped re-review；`make release-gate` 与静态容器门禁检查通过，Docker 实机门禁等待具备 Docker 的宿主机补跑。 |
| 2026-09-07 10:47 CST | 根据部署反馈补强 Compose 策略：默认 HTTP-only 支持本地/内网部署；新增 Caddy 自动 HTTPS 与自带 PEM 证书 HTTPS override；中英文 README 与 Makefile 入口同步更新。 |
| 2026-09-07 11:04 CST | 使用本机 Podman 平替 Docker 完成本地部署验证：容器构建包含前端 build 与 Go 全量测试，HTTP-only Compose 启动成功，`/healthz`、`/readyz`、首页、管理后台和备份命令通过；README 精简为技术栈与部署方案。 |
| 2026-09-07 11:29 CST | 修复管理后台保存 400/500 体验：非法 slug 前端提示并阻止 autosave；缺失分类、标签或封面媒体引用映射为 400；Podman 本地部署已重建并确认加载新前端资源。 |
| 2026-09-07 11:45 CST | 新增无需源码构建的 image-only 部署路径：`compose.deploy.yaml` 仅拉取/运行 `BLOG_IMAGE`，新增 GHCR 镜像发布 workflow，README 说明生产只需 Compose/Caddyfile 配置文件；Podman 本地用预构建镜像验证通过。 |
| 2026-09-07 11:47 CST | README 继续精简补充部署机最小下载命令：只拉取 `compose.deploy.yaml` 与所需 Caddyfile，再通过 `docker compose -f compose.deploy.yaml pull/up` 部署，无需 clone 源码或本地构建。 |
| 2026-09-07 15:55 CST | 根据 Vultr 部署反馈完善中英文 README：补充 Compose 文件选择、raw curl 下载、`.env` 变量、HTTP/HTTPS 启动方式、GHCR 镜像 not found 处理、admin 密码不可查看只能重置、备份恢复/升级/日志/健康检查和 HTTPS 排障清单。 |
| 2026-09-07 16:21 CST | 修复管理后台测试报错：将 slug/taxonomy HTML pattern 调整为浏览器 `v` flag 可编译形式；文章编辑器加载分类/标签列表，使用下拉与复选框替代手填 ID，避免不存在 taxonomy 引用触发保存 400；targeted Vitest 通过。 |
| 2026-09-07 16:53 CST | README 补充 Podman 镜像发布方案：包含单架构 `podman build/push`，以及通过 `podman manifest create/build/push --all` 发布 `linux/amd64` + `linux/arm64` 多架构镜像到 GHCR。 |

## 下一跟进点

1. 准备一篇包含上传媒体的已发布 smoke 文章后，使用 Podman 或 Docker 补跑 `make container-release-gate BACKUP=/data/backups/release-smoke.tar.gz SMOKE_ARTICLE_PATH=/posts/<slug> SMOKE_SEARCH_QUERY=<query>`，补齐 fresh-volume restore-smoke 实测证据。
2. 如目标环境使用公网 HTTPS 或已有证书，分别补跑 `make compose-up-https-auto` 或 `make compose-up-https-files`，并用对应 `COMPOSE_HEALTH_URL` 检查 `/healthz` 与 `/readyz`。
3. 准备发布清单与 changelog，明确 Docker 实机门禁和 Playwright bundled Chromium 下载是发布前环境项。
4. 若要合并/推送/发版，使用应用或本地 git 流程执行；当前分支不自动推送。
