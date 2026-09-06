# Diary Blog 开发进度台账

项目目录：`/Users/iven/work/program/diary_blog`

设计基线：`docs/superpowers/specs/2026-09-03-diary-blog-design.zh-CN.md`

实施计划：`docs/superpowers/plans/2026-09-03-diary-blog-mvp-implementation.md`

执行方式：按实施计划持续自动执行；每个 Task 完成实现、独立审查、修复与复验后提交

当前分支：`codex/diary-blog-mvp`

当前阶段：M4 Operational Release

总体状态：进行中

最后登记：2026-09-06 08:50 CST

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
| M4 Operational Release | Task 13-16 | 进行中 | 运维、备份、容器部署和 E2E 验收通过 |

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
| 15 | 生产构建与 Docker Compose | 完成 | `aa80ca1`, `b52e4d0`, `e3c5117` | 两轮独立审查修复后通过；`npm --prefix admin test`、`npm --prefix admin run build`、`go test ./...`、`go vet ./...`、YAML parse、Makefile dry-run 与 `git diff --check` 通过 | Docker CLI 不在当前宿主机 PATH，Compose build/up/curl 未执行；已修正 Caddy 外网 egress、`/data/site` 恢复路径、trusted proxy CIDR、localhost health 入口与 `10MiB` 上传限制 |
| 16 | E2E、响应式与发布门禁 | 未开始 | - | - | - |

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
- 生产容器化已完成，包含 Node 24 + Go 1.27 多阶段 Dockerfile、非 root runtime、Compose Blog/Caddy 拓扑、Caddy 压缩/安全头/10MiB 上传限制、`/data/site` 应用目录、`/data/backups` 备份路径和 restore-safe Makefile 目标。
- 工作区仍包含 `.gitignore`、`.idea/`、`.metrics/` 用户改动；从本次起 `docs/progress/` 作为总控文档持续更新。

## 风险与偏差

| 编号 | 级别 | 状态 | 内容 | 处理方式 |
|---|---|---|---|---|
| R-001 | 低 | 已缓解 | npm 默认网络曾超时 | 依赖已安装；每个任务继续执行 Vitest、TypeScript 与 Vite 构建 |
| R-002 | 中 | 已缓解 | 早期实现直接位于 `main` | 已切换到 `codex/diary-blog-mvp` 特性分支持续开发 |
| R-003 | 低 | 已缓解 | Go 官方模块代理曾超时 | 依赖已缓存；测试使用独立可写 `GOCACHE` |
| R-004 | 中 | 已裁定 | v2 结构无法还原最后一次成功发布的完整元数据 | 迁移不自动创建公开快照；旧文章须显式重新发布，避免泄露草稿 |
| R-005 | 中 | 已解决 | 配置 URL 含转义路径前缀时 RSS/Sitemap 可能重复编码 | 已统一 URL Path/RawPath 处理并通过专项审查 |
| R-006 | 中 | 待后续 | Caddy 反向代理场景需显式配置可信代理 CIDR，否则登录限流会按直接 peer 计数 | Task 13/15 运维与部署配置阶段接入运行时配置 |
| R-007 | 低 | 已裁定 | AVIF 上传校验当前验证 ISO-BMFF/AVIF 元数据、属性关联和数据 extent，不做 AV1 像素级解码 | Task 12 先采用严格结构门禁；若后续接入维护良好的 AVIF decoder，可替换为像素级验证 |
| R-008 | 低 | 已裁定 | 备份 manifest 写在 archive 最后，无法包含自身 checksum | Manifest 校验所有 payload entry；restore 拒绝 manifest 后额外条目和 payload checksum 不匹配 |
| R-009 | 中 | 已裁定 | 备份/恢复当前限制为 10,000 entries、单 entry 64 MiB、payload 总量 512 MiB、manifest 1 MiB | 首版用对称限制避免生成不可恢复备份；大站点后续需同步提高常量与测试 |
| R-010 | 中 | 待环境验证 | 当前宿主机没有 `docker` 可执行文件 | Task 15 已完成非 Docker 验证；`docker compose build/up/ps` 与 Caddy-routed curl 留到具备 Docker Desktop/Engine 的环境执行 |
| R-011 | 低 | 已裁定 | Compose 固定使用 `172.30.0.0/24` 内部网段和 Caddy `172.30.0.10/32` trusted proxy | README 说明如与宿主 Docker 网络冲突需同时调整 subnet 与 trusted proxy CIDR |

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

## 下一跟进点

1. 执行 Task 16：E2E、响应式与发布门禁。
2. Task 16 通过独立审查后完成 M4 Operational Release，并进入最终整体验证/发布清单阶段。
3. 后续每个 Task 在实现提交和独立审查后同步更新本总控文档。
