# 仓库地图

使用本地图快速定位正确分层与扩展点。

## 顶层入口

- `cmd/main.go`:
  - 程序入口。
- `internal/init/init.go`:
  - 启动编排（按依赖顺序）。
- `internal/router/router.go`:
  - 全局路由分组与中间件挂载。

## 核心运行时初始化

- `internal/core/config.go`:
  - 配置加载与全局配置赋值。
- `internal/core/gorm.go`:
  - 数据库初始化。
- `internal/core/redis.go`:
  - Redis 初始化。
- `internal/core/casbin.go`:
  - Casbin 初始化。
- `internal/core/server.go`:
  - HTTP 服务启动。

## HTTP 层

- `internal/controller/system`:
  - Controller 实现。
- `internal/router/system`:
  - 各领域路由注册（`InitXRouter`）。

## 业务层

- `internal/service/system`:
  - Service 实现与业务编排。

## 数据访问层

- `internal/repository/interfaces`:
  - Repository 契约接口。
- `internal/repository/system`:
  - Repository 具体实现。
- `internal/repository/adapter`:
  - DB 适配器边界。

## 模型与契约

- `internal/model/entity`:
  - 持久化实体。
- `internal/model/dto/request`:
  - API 输入 DTO。
- `internal/model/dto/response`:
  - API 输出 DTO。
- `internal/model/config`:
  - 配置结构体定义。

## 公共包

- `pkg/response`:
  - 统一 API 响应辅助。
- `pkg/errors`:
  - BizError 与错误码抽象。
- `pkg/jwt`:
  - JWT 工具和 `GetUserID(c)`。
- `pkg/util`:
  - 可复用纯工具函数。
- `pkg/storage`, `pkg/ratelimit`, `pkg/redislock`, `pkg/rediskey`:
  - 基础设施支撑包。

## 常见改动路径

## 新增受保护 API

1. 定义请求/响应 DTO。
2. 扩展 repository 接口与实现。
3. 在 service 实现业务逻辑并包装 BizError。
4. 新增 controller 方法并保持统一响应。
5. 在对应 `internal/router/system/*` 注册路由。

## 重构现有模块

1. 定位当前分层越界点。
2. 将逻辑迁移到正确层次。
3. 非必要不改公开接口。
4. 校验错误链路与响应一致性。

## 执行评审任务

1. 从 `references/review-checklist.md` 开始。
2. 检查分层、错误链路、context 传递、配置外置。
3. 用“文件+行号”输出问题项。
