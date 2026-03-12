# 项目规则（权威版）

本文件是本仓库后端改动的权威规则来源。

## 1. 分层

- Controller（`internal/controller`）：
  - 只负责接参、参数校验、调用 Service、组装响应。
  - 禁止承载业务逻辑。
  - 禁止直连 DB。
- Service（`internal/service`）：
  - 只负责业务编排。
  - 持久化必须调用 Repository。
  - 禁止直连 DB。
- Repository（`internal/repository`）：
  - 负责全部 DB 交互，包括 CRUD 和 JOIN。

## 2. DTO 规范

- 请求 DTO：`internal/model/dto/request`
- 响应 DTO：`internal/model/dto/response`
- 禁止使用 Entity 或散装参数作为 API 契约。
- 统一使用 `pkg/response`。

## 3. 错误与日志

- 底层无法处理的错误立即返回。
- Controller 统一捕获并用 `global.Log.Error` 记录。
- 返回统一错误响应。

## 4. Context 传递

- DB 调用和长链路调用必须传 `context.Context`。

## 5. 实现顺序

按以下顺序实现：

- DTO -> Repository -> Service -> Controller -> Router

## 6. 路由组织

- `internal/router/router.go` 仅负责路由组与中间件。
- 业务路由按领域拆分到 `internal/router/system`。
- 每个模块提供 `InitXRouter(*gin.RouterGroup)`。
- 统一通过 `GroupApp.System` 注册。

## 7. 初始化规范

- 具体初始化放在 `internal/core`。
- `internal/init/init.go` 仅按依赖顺序编排。
- 禁止重复启动同一组件。

## 8. 注释要求

- 非显而易见逻辑必须补充必要的生产级注释。

## 9. 禁止硬编码

- 可变配置定义在 `internal/model/config`。
- 配置由 configs 驱动并通过 `global.Config` 读取。
- 业务代码仅保留必要零值兜底。

## 10. 业务错误策略（新功能）

- Repository：
  - 仅返回原始 `error`。
- Service：
  - 用 `pkg/errors` 包装业务错误：
    - `errors.New(code)`
    - `errors.NewWithMsg(code, msg)`
    - `errors.Wrap(code, cause)`
    - `errors.WrapWithMsg(code, msg, cause)`
- Controller：
  - `global.Log.Error(...)`
  - `response.BizFailWithError(err, c)`
- 响应辅助函数：
  - 只负责响应，避免和日志耦合。
- 错误码分段遵循 `pkg/errors/codes.go`：
  - `1xxxx` 通用
  - `2xxxx` 用户
  - `3xxxx` 组织与权限
  - `4xxxx` OJ

## 11. 复用 pkg

- 新增工具前先复用已有 `pkg/*`。
- Controller 获取当前用户 ID 统一使用 `pkg/jwt.GetUserID(c)`。
- 业务无关工具放入 `pkg/util`。
- 跨模块、无业务上下文依赖逻辑从 Service 下沉到 `pkg`。

## 12. 新增独立基础设施能力

- 适用范围：
  - Trace、Metric、Audit、限流适配、消息客户端等可跨模块复用能力。
- 目录与职责：
  - 初始化、连接创建、生命周期管理放在 `internal/core`。
  - `internal/init/init.go` 仅做依赖编排与启动顺序控制。
  - 通用封装放在 `pkg/*`，避免在多个业务模块重复实现。
  - 若需暴露设施路由（如 `/metrics`），在 `internal/router/system` 定义，`internal/router/router.go` 只做挂载。
- 接入边界：
  - Controller 禁止直接依赖第三方基础设施 SDK。
  - Service 禁止直接 `new` 第三方客户端；统一通过 `pkg/*` 或已初始化实例接入。
  - Repository 仅负责数据访问，不承担基础设施编排。
- 配置约束：
  - endpoint、timeout、batch、采样率、开关等可变参数定义在 `internal/model/config`。
  - 运行时统一通过 `global.Config` 读取，仅保留必要零值兜底。
  - `pkg/*` 若需可复用配置，定义模块内 `Options`；由 `core`/`init` 将 `global.Config` 映射后注入。
  - 禁止在 `pkg/*` 直接依赖 `viper` 或 `global.Config`。
- Redis/MySQL 等基础资源边界：
  - 连接初始化、连接池参数、健康检查和关闭动作属于运行时基础设施，放在 `internal/core`（或等价基础设施启动层）。
  - 业务 CRUD/JOIN 仍只允许在 `internal/repository`，禁止下沉到基础设施通用层。
  - Service 通过 Repository 或 `pkg/*` 抽象能力使用资源，不直接编排底层连接细节。
- 错误策略：
  - 初始化失败返回原始 `error`，由上层决定是否 fail-fast。
  - 可降级的运行失败必须记录 `global.Log.Error`，禁止静默吞错。
- 推荐实现顺序：
  - `Config -> Core -> Init -> pkg封装/中间件 -> Service接入（如需要） -> Router暴露（如需要）`

## 13. 权限与投影

- 权限真相只认 DB 关系表；Casbin 是权限投影，不是业务真相。
- `pkg/casbin` 只保留纯权限引擎能力，禁止承载 `owner / super_admin / capability 映射` 这类业务判断。
- `internal/service` 只允许通过 `AuthorizationService` 做业务授权；业务 Service 禁止直接操作 `Enforcer`。
- `role-menu / role-api / role-capability / menu-api` 变更统一写 DB + outbox，不允许在业务 Service 内直接全量刷新 Casbin。
- `user-org-role / 成员状态` 变更允许同步收口当前主体投影，但仍必须补发异步修复事件。

## 提问引用规范

在实现或评审过程中需要向用户澄清时，追加一行：

- `依据规则: #<rule-number>`

若适用多条规则：

- `依据规则: #2,#3,#9`
