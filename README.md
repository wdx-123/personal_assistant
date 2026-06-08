# Personal Assistant

> 一个基于 Go + Gin 的算法后端系统，围绕用户组织、权限投影、OJ 数据同步、图片资源治理、AI 会话记忆和可观测性做工程化整合。

它的核心价值在于把认证授权、组织协作、异步任务、外部 OJ 数据、AI 流式对话、记忆召回、事件一致性和运行期观测放到同一个后端体系里，并用清晰的分层边界约束复杂度。

整体架构口径是：**传统 MVC 主体 + AI 子域渐进式 DDD**。项目主体仍按 Controller / Service / Repository / Router / Core 组织；AI 子域在复杂度较高的位置补充 `internal/domain/ai` 和 `internal/infrastructure/ai`，用于隔离稳定协议与具体运行时实现。

## 核心能力

| 模块 | 能力 | 关键实现 |
| --- | --- | --- |
| 用户与认证                              | 注册、登录、登出、刷新 Token、账号状态管理 | Access Token + Refresh Token；Refresh Token 使用 HttpOnly Cookie；活跃态校验支持 Redis 缓存与 DB 回源 |
| 组织与权限                              | 组织、成员、角色、菜单、API、能力点管理 | 权限真相保存在 DB 关系表；Casbin 作为权限投影；用户当前组织通过 `current_org_id` 参与授权上下文 |
| OJ 数据    | LeetCode / Luogu / Lanqiao 账号绑定、数据同步、排行榜、曲线统计 | 外部 crawler client、Redis Stream、Outbox、缓存投影、读模型聚合 |
| OJ 任务    | 任务创建、版本派生、立即执行、重试、执行明细查询 | 任务调度、快照落库、执行用户明细、Redis 分布式锁防重复执行 |
| AI 助手 | 会话管理、SSE 流式输出、Eino / local runtime 切换、AI tool 协议 | `domain/ai.Runtime` 定义运行时协议；Service 负责上下文组装、tool 授权和落库收尾 |
| AI 记忆 | 会话摘要、事实记忆、长期文档、RAG 召回 | MySQL 保存事实和摘要；Qdrant 保存向量 chunk；写回链路由 extractor + policy 控制 |
| 图片资源 | 上传、删除、列表、本地 / 七牛存储 | 上传限流、七牛熔断、软删除、孤儿文件清理 |
| 事件一致性 | 权限、缓存、OJ 统计、任务触发等异步收敛 | Outbox + Redis Stream + subscriber；多实例通过锁和消息投影降低重复处理 |
| 可观测性 | 请求链路、指标聚合、运行时查询 | Request ID、W3C trace propagation、metrics flush、trace span 入库和查询接口 |

## 架构总览

```text
Client
  |
  v
Gin Router + Middleware
  |-- RequestID / Observability / CORS / Timeout
  |-- JWTAuth / ActiveUserMW / PermissionMiddleware
  v
Controller
  |
  v
Service
  |-- AuthorizationService
  |-- AI context / memory / tool orchestration
  |-- Outbox events / projections / task scheduling
  v
Repository
  |
  v
MySQL

Redis / Redis Stream / Qdrant / external crawler / object storage
通过 core、infrastructure 和 pkg 封装接入，不直接塞进 Controller。
```

主要目录职责：

```text
cmd/                         程序入口
configs/                     YAML 配置与 Casbin model
internal/core/               配置、日志、DB、Redis、Qdrant、AI、SSE、Casbin、存储、任务等初始化
internal/router/             路由组和中间件挂载
internal/controller/system/  HTTP 参数接收、校验、响应组装
internal/service/system/     业务编排、权限收口、AI 上下文和任务调度
internal/repository/         DB CRUD / JOIN / 读模型查询
internal/domain/ai/          AI 稳定协议、事件、runtime、tool、memory 抽象
internal/infrastructure/     外部 OJ client、Redis 消息、SSE、AI runtime、Qdrant 适配
internal/model/              entity、DTO、readmodel、config
pkg/                         jwt、response、errors、casbin、storage、ratelimit、redislock、observability 等公共能力
docs/                        架构、权限、AI、事件、图片、接口说明
plan/                        执行型任务计划
```

## 重点实现链路

### 认证与权限

- 登录后发放 Access Token，并通过 HttpOnly Cookie 设置 Refresh Token。
- `JWTAuth` 负责解析访问令牌，`ActiveUserMW` 负责账号活跃态校验，`PermissionMiddleware` 负责组织、角色和权限上下文。
- JWT 不再信任角色字段，授权时动态读取 DB / 投影状态。
- `role-menu`、`role-api`、`role-capability`、`menu-api` 等关系变化先写 DB，再通过 outbox / subscriber 收敛 Casbin 投影。

### OJ 数据与任务

- OJ 账号绑定和数据同步通过 infrastructure client 调用外部 crawler 服务。
- 绑定、题目 upsert、每日统计、缓存刷新等链路通过 Redis Stream 和 Outbox 解耦。
- 排行榜读侧使用缓存投影和 read model 聚合，避免每次请求都扫明细表。
- OJ 任务支持创建、派生版本、立即执行、重试和执行明细查询；调度与执行侧使用 Redis 锁降低多实例重复执行风险。

### AI 会话与记忆

- HTTP 会话接口负责创建、查询和删除会话；流式输出走 SSE。
- Service 在执行前准备用户消息、会话历史、当前组织上下文、可见工具和记忆上下文。
- `internal/domain/ai` 只定义 runtime、event、sink、tool、memory 等稳定协议，不依赖 Gin、GORM、Eino 或 Redis。
- `internal/infrastructure/ai` 承载 Eino runtime、local runtime、tool schema、Qdrant memory store、embedding、chunker 等技术实现。
- 成功的 AI 轮次会触发记忆写回：消息快照 -> extractor 提取候选 -> policy 裁决 scope / visibility / TTL -> MySQL 保存 summary / facts / documents -> 可选切分并写入 Qdrant。

### 事件一致性与观测

- Outbox Relay 将业务事件投递到 Redis Stream，subscriber 负责投影修复和异步处理。
- 权限投影、缓存投影、OJ 每日统计投影和 OJ 任务触发各自有明确 topic / group / consumer 配置。
- 可观测性中间件统一注入 request id，支持 W3C trace 解析与注入。
- metrics 和 trace span 通过批量 flush / Redis Stream 入库，并通过 `/system/observability/*` 查询。

## 技术栈

- 语言与框架：Go 1.24、Gin
- 数据访问：GORM、MySQL
- 缓存与消息：Redis、Redis Stream
- 权限：Casbin、gorm-adapter
- AI：CloudWeGo Eino、OpenAI/Qwen/Ark 兼容模型适配、Qdrant
- 存储：本地文件、七牛云
- 配置与日志：Viper、godotenv、Zap、lumberjack
- 任务与稳定性：robfig/cron、Redis 分布式锁、限流、熔断
- 工程质量：go test、go vet、golangci-lint

## 本地启动

### 1. 前置依赖

必需：

- Go `1.24.x`，`go.mod` 中指定 `toolchain go1.24.9`
- MySQL 8.x
- Redis 6+

按需启用：

- Qdrant：默认配置为启用；如果没有本地 Qdrant，请先把 `QDRANT_ENABLED=false`
- OJ crawler 服务：用于 LeetCode / Luogu / Lanqiao 数据同步
- AI 模型 API Key：`SSE_AI_RUNTIME_MODE=eino` 时使用；Eino 初始化失败会回退到 local runtime
- 七牛云：仅 `STORAGE_CURRENT=qiniu` 时需要

### 2. 准备配置

```powershell
Copy-Item .env.example .env
```

至少检查这些配置：

```text
DB_HOST / DB_PORT / DB_NAME / DB_USERNAME / DB_PASSWORD
REDIS_ADDRESS / REDIS_PASSWORD / REDIS_DB
JWT_ACCESS_TOKEN_SECRET / JWT_REFRESH_TOKEN_SECRET
SYSTEM_SESSIONS_SECRET
SYSTEM_HOST / SYSTEM_PORT
QDRANT_ENABLED / QDRANT_ENDPOINT / QDRANT_GRPC_HOST / QDRANT_GRPC_PORT
SSE_AI_RUNTIME_MODE / AI_PROVIDER / AI_API_KEY / AI_MODEL
CRAWLER_LEETCODE_BASE_URL / CRAWLER_LUOGU_BASE_URL / CRAWLER_LANQIAO_BASE_URL
```

最小本地启动建议：

```text
QDRANT_ENABLED=false
SSE_AI_RUNTIME_MODE=local
STORAGE_CURRENT=local
```

### 3. 安装依赖并启动

```powershell
go mod download
go run .\cmd\main.go
```

默认监听地址由 `SYSTEM_HOST` + `SYSTEM_PORT` 决定，模板中为 `0.0.0.0:9000`。

启动时会按顺序初始化配置、日志、Qdrant、敏感数据编解码器、DB、自动迁移、Redis、SSE、AI runtime、Casbin、存储、限流器、Repository、Observability、subscriber、权限投影和定时任务。

### 4. 数据库初始化

`AUTO_MIGRATE=true` 时启动会自动建表和迁移。也可以手动执行：

```powershell
go run .\cmd\main.go --sql
```

## Docker 启动

```powershell
docker compose up -d --build
```

当前 `docker-compose.yaml` 只包含 `app` 服务：

- 暴露端口：`9000:9000`
- 挂载目录：`./static:/app/static`、`./log:/app/log`
- 读取环境变量：`.env`

MySQL、Redis、Qdrant 不在 compose 内，需要外部提供。容器访问宿主机服务时，可按实际环境使用 `host.docker.internal` 或同网络服务名。

## CLI 与 CI

内置 CLI 一次只允许执行一个命令：

```powershell
go run .\cmd\main.go --sql
go run .\cmd\main.go --sql-export
go run .\cmd\main.go --sql-import .\backup.sql
```

CI 当前执行：

```text
go mod download
bash scripts/check_no_legacy_error_tracking.sh
go test ./...
go vet ./...
golangci-lint v1.64
```

## 接口分组

公共接口：

```text
GET  /api/v1/health
GET  /api/v1/ping
POST /base/captcha
POST /base/sendEmailVerificationCode
POST /user/register
POST /user/login
POST /refreshToken
```

登录后可访问的业务接口：

```text
POST   /user/logout
PUT    /user/profile
PUT    /user/phone
PUT    /user/password
POST   /user/deactivate

POST   /oj/bind
POST   /oj/lanqiao/bind
POST   /oj/ranking_list
POST   /oj/stats
POST   /oj/curve

POST   /oj/task
GET    /oj/task/list
POST   /oj/task/analyze
POST   /oj/task/:id/execute-now
POST   /oj/task/:id/revise
POST   /oj/task/:id/retry
GET    /oj/task/:id

POST   /ai/conversations
GET    /ai/conversations
GET    /ai/conversations/:id/messages
DELETE /ai/conversations/:id
POST   /ai/conversations/:id/stream

POST   /api/system/image/upload
DELETE /api/system/image/delete
GET    /api/system/image/list

GET    /system/org/my
PUT    /system/org/current
POST   /system/org/join
POST   /system/org/leave
```

需要 JWT + 权限投影校验的系统管理接口：

```text
/system/api/*
/system/menu/*
/system/role/*
/system/org/*
/system/user/*
/system/observability/*
```

Access Token 支持：

```text
x-access-token: <token>
Authorization: Bearer <token>
```

Refresh Token 默认从 HttpOnly Cookie `x-refresh-token` 读取，刷新接口也兼容 JSON body 中的 refresh token 字段。

## 文档索引

重点文档：

- `docs/Casbin-RBAC权限系统架构文档.md`
- `docs/双Token认证方案-整合版.md`
- `docs/事件驱动架构-RedisStream-Outbox-双通道一致性实践.md`
- `docs/SSE实时推送基础设施重构指导文档.md`
- `docs/图片管理-技术文档.md`
- `docs/图片上传流.md`
- `docs/context从api贯穿到repository.md`
- `docs/AI/AI项目演进说明.md`
- `docs/AI/AI领域+DDD架构拆分.md`
- `docs/AI/记忆模块设计.md`
- `docs/AI/记忆-最后-混合召回.md`
- `docs/AI/Qdrant向量数据库配置.md`
- `docs/apifox/*.openapi.json`

## 安全说明

- `.env` 不应提交到公开仓库。
- 公开前必须轮换 MySQL、Redis、JWT、Session、邮箱、AI、Qdrant、七牛等密钥。
- `configs/configs.yaml` 和 `.env.example` 只能保留占位值或本地开发默认值。
- Qdrant API Key、AI API Key、七牛 AK/SK 等只允许通过环境变量注入。

## License

当前仓库未提供 `LICENSE` 文件。
