# Personal Assistant

> 一个基于 Go 的后端项目，聚焦用户与组织管理、RBAC 权限控制、OJ 刷题数据同步、图片资源管理。

## 项目亮点

- **用户认证**：支持注册、登录、登出、刷新 Token，采用双 Token 方案与 HttpOnly Refresh Token。
- **组织与权限**：覆盖组织、角色、菜单、API 管理，支持用户-组织-角色关系建模。
- **RBAC**：基于 Casbin 做权限校验，并通过权限投影链路支撑多实例下的策略收敛。
- **OJ 能力**：支持 LeetCode / Luogu 账号绑定、异步同步、排行榜查询与缓存投影。
- **图片管理**：支持本地 / 七牛双存储驱动，包含上传限流、孤儿文件清理等治理能力。
- **事件驱动一致性**：基于 Redis Stream + Outbox + 双通道收口，用于异步解耦、投影刷新和多实例收敛。
- **第三方基础设施接入**：在 `internal/infrastructure` 中统一封装 LeetCode / Luogu / Lanqiao 客户端，采用配置驱动，便于替换和扩展。
- **可观测性**：提供请求链路追踪、GORM/任务埋点和指标聚合查询能力。
- **稳定性治理**：集成七牛存储熔断、上传限流、Redis 分布式锁与任务协调。
- **排行榜设计**：基于 Redis ZSet + 用户快照投影 + 读模型聚合查询，兼顾查询性能与异步收敛。
- **框架设计**：按 Controller / Service / Repository / Router / Core / Infrastructure / pkg 分层，拆分业务编排、数据访问与基础设施初始化职责。

## 技术栈

- 语言与框架：Go, Gin
- 数据层：Gorm, MySQL
- 缓存与消息：Redis, Redis Stream
- 权限：Casbin
- 配置与日志：Viper, Zap
- 其他：Resty, robfig/cron, urfave/cli

## 目录结构

```text
.
├── cmd/                     # 程序入口
├── configs/                 # 配置文件（yaml + casbin model）
├── internal/
│   ├── controller/          # HTTP 控制器
│   ├── service/             # 业务逻辑
│   ├── repository/          # 数据访问层
│   ├── router/              # 路由注册
│   ├── middleware/          # 中间件
│   ├── infrastructure/      # 外部服务接入与消息基础设施（LeetCode/Luogu/Lanqiao/Outbox）
│   └── core/                # 启动流程、配置、日志、数据库、任务初始化
├── pkg/                     # 公共组件（jwt、response、storage、errors 等）
├── docs/                    # 项目文档
├── docker-compose.yaml
└── Dockerfile
```

## 快速开始（本地）

### 1. 前置依赖

- Go `>= 1.24`（`go.mod` 中配置了 `toolchain go1.24.9`）
- MySQL `8.x`
- Redis `6+`
- 可选：OJ 爬虫服务（用于 LeetCode/洛谷数据接口）

### 2. 配置环境变量

复制环境变量模板：

```bash
# Linux / macOS
cp .env.example .env

# Windows PowerShell
Copy-Item .env.example .env
```

然后按你的环境修改 `.env`，至少确认：

- `DB_HOST/DB_PORT/DB_NAME/DB_USERNAME/DB_PASSWORD`
- `REDIS_ADDRESS/REDIS_PASSWORD/REDIS_DB`
- `JWT_ACCESS_TOKEN_SECRET/JWT_REFRESH_TOKEN_SECRET`
- `SYSTEM_HOST/SYSTEM_PORT`
- `CRAWLER_LEETCODE_BASE_URL/CRAWLER_LUOGU_BASE_URL`（如使用 OJ 功能）

### 3. 启动服务

```bash
go mod tidy
go run cmd/main.go
```

默认监听：`0.0.0.0:9000`

### 4. 数据库初始化

- 默认 `AUTO_MIGRATE=true` 时会自动迁移表结构。
- 也可手动执行：

```bash
go run cmd/main.go --sql
```

## Docker 启动

```bash
docker compose up -d --build
```

说明：

- 当前 `docker-compose.yaml` 只包含 `app` 服务。
- MySQL / Redis 需要你自行提供并确保容器内可访问（例如通过 `host.docker.internal` 或同网络服务名）。

## 命令行工具

项目内置 CLI 参数（一次仅支持一个）：

- `--sql`：初始化/迁移数据库表结构
- `--sql-export`：导出 MySQL 数据（依赖名为 `mysql` 的 Docker 容器）
- `--sql-import <file>`：从 SQL 文件导入数据

示例：

```bash
go run cmd/main.go --sql-import .\backup.sql
```

## 接口分组概览

> 当前路由前缀并非完全统一，以下为主要分组。

- 公共接口（无需 JWT）
- `POST /base/captcha`
- `POST /base/sendEmailVerificationCode`
- `POST /user/register`
- `POST /user/login`
- `POST /refreshToken`

- 业务接口（需 JWT）
- `POST /user/logout`
- `PUT /user/profile`
- `PUT /user/phone`
- `PUT /user/password`
- `POST /oj/bind`
- `POST /oj/ranking_list`
- `POST /oj/stats`
- `POST /api/system/image/upload`
- `DELETE /api/system/image/delete`
- `GET /api/system/image/list`
- `GET /system/org/my`
- `PUT /system/org/current`

- 系统管理接口（需 JWT + 权限）
- `/system/api/*`
- `/system/menu/*`
- `/system/role/*`
- `/system/org/*`
- `/system/user/list`
- `/system/user/{id}`
- `/system/user/{id}/roles`
- `/system/user/assign_role`

## 认证说明

- Access Token 支持：
- 请求头 `x-access-token`
- 或 `Authorization: Bearer <token>`
- Refresh Token 默认使用 HttpOnly Cookie：`x-refresh-token`

## 相关文档

- `docs/事件驱动架构-RedisStream-Outbox-双通道一致性实践.md`
- `docs/Casbin-RBAC权限系统架构文档.md`
- `docs/双Token认证方案-整合版.md`
- `docs/图片管理-技术文档.md`
- `docs/图片上传流.md`
- `docs/flag指令.md`

第三方集成、可观测性、排行榜与框架整体设计文档持续补充中。

## 安全提醒（公开仓库前建议先做）

- 全量替换 `.env` / `.env.example` / 配置文件中的真实密钥与密码。
- 轮换数据库密码、Redis 密码、JWT 密钥、邮箱密钥、云存储密钥。
- 确保 `.env` 不会被提交（已在 `.gitignore` 中忽略）。

## License

当前仓库未提供 `LICENSE` 文件。
