# Personal Assistant

> 一个以 Go + Gin 为核心的模块化单体后端，围绕“用户/组织/RBAC 权限治理 + OJ 数据同步与任务化运营 + AI 会话/SSE 流式助手 + 图片与可观测性治理”构建的业务型平台后端。

它解决的不是单一后台管理问题，而是把账号体系、组织协作、权限控制、OJ 数据运营、AI 助手能力和一批工程治理能力整合到一套后端里。

从代码现状看，本项目更适合被定义为“面向业务场景的 Go 平台后端”，不是微服务，也不是纯 CRUD 管理台。

## 为什么它不是简单 CRUD

- 权限不是直接把规则写死在接口里，而是基于 `user-org-role` 关系表建模，DB 作为真相源，Casbin 作为权限投影层。
- OJ 模块不是单次查询接口，而是覆盖绑定、同步、统计、曲线、排行榜和任务化运营。
- OJ 任务支持任务版本化、定时调度、执行抢占和冻结快照，属于可追溯执行链路。
- AI 子域已经落地了会话持久化、SSE 单流输出、interrupt/decision 与 Eino runtime 主链。
- 异步收敛不是简单 goroutine，而是基于 Outbox + Redis Stream + Pub/Sub 组织跨模块最终一致性。

## 项目定位

- 用户/组织/RBAC 权限治理：覆盖账号生命周期、组织成员关系、角色/菜单/API/capability 权限管理。
- OJ 数据同步与任务化运营：支持 LeetCode / Luogu / Lanqiao 账号绑定、排行榜、统计曲线和任务执行。
- AI 会话/SSE 助手能力：提供会话管理、流式输出、人工决策回填和 Eino runtime 接入。
- 图片与可观测性治理：覆盖图片上传删除、双存储驱动、链路埋点、指标查询和任务追踪。

## 核心能力

### 核心功能

- 用户认证：支持注册、登录、刷新 Token、登出、资料维护、密码修改、主动注销。
- 组织管理：支持创建组织、加入组织、退出组织、切换当前组织、踢出成员、恢复成员。
- RBAC 权限：支持角色、菜单、API 管理和 capability 建模，并将权限投影到 Casbin。
- OJ 数据：支持 OJ 账号绑定、排行榜、统计数据、成绩曲线和题库同步。
- OJ 任务：支持标题分析建任务、立即执行、版本派生、重试和执行结果追踪。
- AI 会话：支持会话 CRUD、消息列表、SSE 流式输出和 interrupt/decision 决策恢复。

### 支撑能力

- 图片存储：支持本地 / 七牛双驱动、上传限流、批量删除和孤儿图片清理。
- SSE 基础设施：支持连接管理、回放、Pub/Sub backplane 和多连接控制。
- 可观测性：支持 HTTP/Gorm/任务 trace、指标聚合和查询接口。
- 缓存与限流：支持活跃态缓存、OJ 绑定限流、上传限流、排行榜缓存。

### 工程能力

- 事件驱动收敛：基于 Outbox + Redis Stream + Pub/Sub 处理权限投影、缓存投影和异步任务触发。
- 调度与协作：支持 Cron、Redis 分布式锁、任务抢占和消费者分组。
- 初始化治理：支持自动迁移、Repository/Service/Controller 装配、基础设施统一初始化。
- 运行与运维：支持 CLI、Docker、本地/生产部署配置和日志落盘。

## 架构总览

### 架构结论

- 模块化单体
- 分层架构 / Handler-Service-Repository
- 业务真相在 MySQL，投影在 Redis / Casbin
- 异步解耦采用 Outbox + Redis Stream + Pub/Sub
- AI 子域采用 Eino runtime + SSE 单流控制

### 启动链

```text
cmd/main
  -> internal/init
  -> internal/core / internal/infrastructure
  -> internal/repository
  -> internal/service
  -> internal/controller
  -> internal/router
  -> gin server
```

### 运行时关键点

- 入口从 `cmd/main.go` 启动，主初始化编排在 `internal/init/init.go`。
- `internal/core` 负责配置、日志、Gorm、Redis、Casbin、Storage、SSE、Observability、Outbox Relay、Cron 等基础设施初始化。
- `internal/router/router.go` 负责中间件挂载和路由分组，业务路由在 `internal/router/system` 中按领域拆分。
- `internal/service/system` 负责业务编排，`internal/repository` 负责数据库与投影数据访问。

## 核心模块

- 启动与基础设施  
  职责：配置、日志、Gorm、Redis、Casbin、Storage、SSE、Outbox、Cron 初始化。  
  关键目录：`cmd/`、`internal/init/`、`internal/core/`

- 接口层  
  职责：Gin 路由注册、DTO 接参、统一响应、鉴权中间件接入。  
  关键目录：`internal/router/`、`internal/controller/`、`internal/middleware/`

- 用户与认证  
  职责：注册登录、双 Token、账号状态、资料维护、当前组织上下文。  
  关键目录：`internal/service/system/userSvc.go`、`jwtSvc.go`、`baseSvc.go`

- 组织与权限  
  职责：组织管理、成员关系、角色/菜单/API/capability 管理、权限判断与权限投影。  
  关键目录：`internal/service/system/orgSvc.go`、`roleSvc.go`、`menuSvc.go`、`authorizationSvc.go`、`permissionProjectionSvc.go`

- OJ 业务  
  职责：LeetCode / Luogu / Lanqiao 账号绑定、数据同步、统计、排行、曲线、题库预热。  
  关键目录：`internal/service/system/ojSvc.go`、`ojLanqiaoSvc.go`、`internal/infrastructure/leetcode/`、`luogu/`、`lanqiao/`

- OJ 任务  
  职责：任务分析、版本管理、调度执行、执行快照、用户命中详情查询。  
  关键目录：`internal/service/system/ojTaskSvc.go`、`ojTaskDispatcher.go`

- AI / Agent  
  职责：会话管理、消息持久化、流式输出、interrupt/decision、runtime 抽象与 Eino 接入。  
  关键目录：`internal/service/system/aiSvc.go`、`aiRuntime*.go`、`internal/infrastructure/ai/eino/`

- 图片与存储  
  职责：图片上传、列表、删除、孤儿治理、本地 / 七牛驱动切换。  
  关键目录：`internal/service/system/imageSvc.go`、`pkg/storage/`、`pkg/imageops/`

- 可观测性  
  职责：请求追踪、Gorm Trace、任务 Trace、指标聚合和查询接口。  
  关键目录：`internal/core/observability.go`、`internal/middleware/observabilityMW.go`、`pkg/observability/`

- 消息与异步  
  职责：Outbox Relay、Redis Stream 消费、缓存投影、权限投影、OJ 日统计投影。  
  关键目录：`internal/core/outboxRelay.go`、`internal/core/subscriberInit.go`、`internal/infrastructure/outbox/`、`internal/infrastructure/messaging/`

## 核心链路

### 请求流

客户端请求先经过 `RequestID / Observability / Logger / Recovery / CORS` 等全局中间件，再根据分组进入 `JWTAuth / ActiveUser / PermissionMiddleware`。Controller 负责接参与响应，Service 负责业务编排，Repository 负责落 MySQL / Redis，最终由 `pkg/response` 统一返回。

### 数据流

业务真相主要落在 MySQL，例如 `users / orgs / user_org_roles / roles / menus / apis / oj_* / ai_* / outbox_events`。Redis 负责缓存、排行榜、SSE 回放、消息消费、分布式锁和 trace stream。Casbin 只承担权限投影，不承担业务真相。

### 异步流

业务写库时同步写 Outbox；Relay 把 Outbox 事件推到 Redis Stream；Subscriber 消费后做权限投影、缓存投影、OJ 日统计修复和 OJ 任务触发。定时任务则负责全量同步、排行重建、禁用账号清理和孤儿图片清理。

### 典型业务链路

1. 注册  
   创建用户后，会补齐组织关系、默认角色、权限/缓存投影，完成账号初始状态收口。

2. OJ 绑定  
   调用外部 crawler 服务拉取数据，Upsert 用户 OJ 明细，再刷新排行缓存并发布后续投影事件。

3. OJ 任务执行  
   调度器扫描待执行任务，通过 Redis 锁抢占执行权，生成任务快照并写入执行结果。

4. AI 会话流式输出  
   先创建会话，再通过 `POST /ai/conversations/:id/stream` 进入流式对话，运行时会写消息骨架、推送 SSE，并在需要时通过 interrupt/decision 恢复执行。

## 技术栈与依赖

- 框架：Go、Gin
- 存储：MySQL、Gorm、Redis
- 权限：Casbin
- 异步：Redis Stream、Pub/Sub、Cron
- AI：CloudWeGo Eino、Qwen / OpenAI 兼容接入
- 工程化：Viper、Zap、Resty、urfave/cli、Qiniu SDK

### 依赖边界

- MySQL、Redis：必需依赖
- OJ crawler 服务：可选外部依赖，使用 OJ 功能时必需
- 七牛云存储：可选依赖，不使用时可退回本地存储

## 快速开始

### 1. 前置依赖

- Go `1.23+`，项目中启用了 `toolchain go1.24.9`
- MySQL `8.x`
- Redis `6+`
- 可选：OJ crawler 服务

### 2. 配置环境变量

```bash
# Linux / macOS
cp .env.example .env

# Windows PowerShell
Copy-Item .env.example .env
```

至少确认以下配置可用：

- `DB_HOST / DB_PORT / DB_NAME / DB_USERNAME / DB_PASSWORD`
- `REDIS_ADDRESS / REDIS_PASSWORD / REDIS_DB`
- `JWT_ACCESS_TOKEN_SECRET / JWT_REFRESH_TOKEN_SECRET`
- `SYSTEM_HOST / SYSTEM_PORT`
- `CRAWLER_LEETCODE_BASE_URL / CRAWLER_LUOGU_BASE_URL / CRAWLER_LANQIAO_BASE_URL`

### 3. 本地启动

```bash
go mod tidy
go run cmd/main.go
```

默认监听：`0.0.0.0:9000`

### 4. 数据库初始化

- `AUTO_MIGRATE=true` 时，启动时会自动迁移表结构。
- 也可以手动执行：

```bash
go run cmd/main.go --sql
```

### 5. Docker 启动

本地容器启动：

```bash
docker compose up -d --build
```

说明：

- 根目录 `docker-compose.yaml` 当前只编排 `app` 服务。
- `deploy/docker-compose.prod.yml` 提供生产场景下的 `app + web` 样例。
- MySQL / Redis 需要你自行提供并保证网络可达。

## 配置说明

配置主要由 `.env.example` 和 `configs/configs.yaml` 驱动，建议按下面这些组来理解：

- `system / jwt / session`：服务监听、环境、双 Token、Session
- `mysql / redis`：数据库、缓存与连接池
- `storage / static`：本地静态目录、七牛存储、图片限制
- `crawler`：LeetCode / Luogu / Lanqiao 外部接口地址与超时重试
- `observability`：指标、trace、清理周期、采样与脱敏策略
- `task / messaging`：定时任务、Outbox、Redis Stream、分布式锁
- `sse / ai`：SSE 心跳与回放、AI provider、model、checkpoint、runtime 命令通道

具体键名和默认值，请以：

- `.env.example`
- `configs/configs.yaml`

为准。

## 命令行工具

项目内置 CLI 参数，一次仅支持一个：

- `--sql`：初始化/迁移数据库结构
- `--sql-export`：导出 SQL 数据
- `--sql-import <file>`：导入 SQL 文件

示例：

```bash
go run cmd/main.go --sql-import .\backup.sql
```

说明：`--admin` 标志在当前代码中有声明，但主执行分支尚未接入，不建议视为已完成能力。

## 认证方式

- Access Token 支持：
  - 请求头 `x-access-token`
  - 或 `Authorization: Bearer <token>`
- Refresh Token 默认走 HttpOnly Cookie：`x-refresh-token`

## 接口分组概览

> 当前代码中的路由前缀并未完全统一，下面按真实路由分组列示，不能简单理解为都挂在同一个 `/api` 前缀下。

| 分组 | 示例接口 | 权限要求 |
| --- | --- | --- |
| 健康检查 | `GET /api/v1/health`、`GET /api/v1/ping` | 无 |
| 基础服务 | `POST /base/captcha`、`POST /base/sendEmailVerificationCode` | 无 |
| 用户认证 | `POST /user/register`、`POST /user/login`、`POST /refreshToken` | 无 |
| 用户业务 | `POST /user/logout`、`PUT /user/profile`、`PUT /user/phone`、`PUT /user/password`、`POST /user/deactivate` | JWT |
| 组织业务 | `GET /system/org/my`、`PUT /system/org/current`、`POST /system/org/join`、`POST /system/org/leave` | JWT |
| 系统权限管理 | `/system/api/*`、`/system/menu/*`、`/system/role/*`、`/system/org/*`、`/system/user/*` | JWT + 权限 |
| OJ | `POST /oj/bind`、`POST /oj/lanqiao/bind`、`POST /oj/ranking_list`、`POST /oj/stats`、`POST /oj/curve` | JWT |
| OJ Task | `POST /oj/task/analyze`、`POST /oj/task`、`GET /oj/task/list`、`POST /oj/task/:id/execute-now` | JWT |
| AI 会话 | `POST /ai/conversations`、`GET /ai/conversations`、`GET /ai/conversations/:id/messages`、`DELETE /ai/conversations/:id` | JWT |
| AI SSE | `POST /ai/conversations/:id/stream`、`POST /ai/conversations/:id/interrupts/:interrupt_id/decision` | JWT |
| 图片 | `POST /api/system/image/upload`、`DELETE /api/system/image/delete`、`GET /api/system/image/list` | JWT |
| 可观测性 | `GET /system/observability/traces/detail/:id`、`POST /system/observability/traces/query`、`POST /system/observability/metrics/query` | JWT + 权限 |

## 当前完成度与边界

### 已落地主链路

- 用户 / 组织 / 权限主链路
- OJ 绑定、统计、曲线、排行主链路
- OJ 任务版本化、调度、执行与快照主链路
- AI 会话、SSE、interrupt/decision、Eino runtime 主链路
- 图片治理、可观测性、Outbox 异步收敛主链路

### 不宜过度表述的部分

- 当前代码是单体应用，不是微服务体系
- AI 子域已落地主链路，但不应表述为完整通用多 Agent 平台
- 路由前缀与接口治理并未完全统一，不能表述为完整 OpenAPI 平台

### 外部依赖边界

- OJ 数据依赖外部 crawler 服务
- 七牛仅是可选存储驱动，不是必需组件
- 生产部署编排样例存在，但完整生产环境仍需自行补齐 MySQL / Redis / 网关等外围设施

## 相关文档

- `docs/事件驱动架构-RedisStream-Outbox-双通道一致性实践.md`
- `docs/Casbin-RBAC权限系统架构文档.md`
- `docs/双Token认证方案-整合版.md`
- `docs/AI助手架构设计方案.md`
- `docs/SSE实时推送基础设施重构指导文档.md`
- `docs/图片管理-技术文档.md`
- `docs/图片上传流.md`
- `docs/flag指令.md`

如果你是面试阅读者，建议优先看：

1. `事件驱动架构-RedisStream-Outbox-双通道一致性实践`
2. `Casbin-RBAC权限系统架构文档`
3. `AI助手架构设计方案`

## 安全提醒

- 发布公开仓库前，务必替换 `.env`、`.env.example` 和配置文件中的密钥、密码与第三方凭证。
- 建议轮换数据库密码、Redis 密码、JWT 密钥、邮箱密钥、对象存储密钥。
- 确保 `.env` 不会被提交；当前仓库已在 `.gitignore` 中忽略。
- `configs/configs.yaml` 中存在历史示例值，实际部署时应统一改为安全配置。

## License

当前仓库未提供 `LICENSE` 文件。
