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

## 14. AI 子域渐进式 DDD 规则

- 当前项目的正式口径是 `MVC 主体 + AI 子域渐进式 DDD`。
- 这表示：
  - 项目整体仍以传统 MVC 目录和职责划分为主。
  - AI 子域在复杂度上升后，允许渐进式补 `internal/domain/ai` 与 `internal/infrastructure/ai`。
  - 默认目标不是全量 DDD 重构，也不是项目整体目录全面改名。
- AI 子域目录职责：
  - `internal/controller/system`、`internal/router/system`
    - 继续作为 AI HTTP / SSE 入口层。
  - `internal/service/system`
    - 继续作为 AI 应用编排层，负责会话流程、上下文组装、tool 注册与授权收口、sink/projector 协调。
  - `internal/domain/ai`
    - 放稳定协议、事件、tool/runtime 抽象、领域语义。
    - 禁止依赖 Gin、GORM、Eino、Redis、第三方模型 SDK。
  - `internal/infrastructure/ai`
    - 放 Eino / Local runtime、模型 SDK、tool adapter、checkpoint / approval / runtime control 等技术实现。
  - `internal/repository/*`
    - 继续负责 AI 持久化访问，不因为 AI 子域拆分而绕开 Repository。
- AI 任务落点判断：
  - 协议与抽象：优先落 `domain/ai`
  - 应用编排：优先落 `service/system`
  - 基础设施适配：优先落 `infrastructure/ai`
  - 持久化：优先落 `repository/*`
- 禁止模式：
  - 把 AI 子域演进误表述为“已经完成全量 DDD 重构”。
  - 把 runtime、tool、trace、prompt 拼装、恢复控制等复杂度继续无边界堆进单个 `service` 文件。
  - 在 `domain/ai` 中直接依赖 HTTP、数据库或具体 Agent 框架。
  - 因 AI 子域局部改造而强行推动整个项目做无必要的目录迁移。
- 阶段演进说明：
  - A2UI、interrupt、approval、runtimecontrol 等能力允许按阶段收缩、停用或重建。
  - 无论具体功能阶段如何变化，AI 子域的依赖方向和目录边界必须保持稳定。

## 计划落盘规则

- 只要任务属于新增、重构、修复、联调、排障、迁移、删除、配置调整这类执行型工作，先写计划，不直接改代码。
- 计划文件固定写到 `plan/<module>/pending-<task>.md`。
- 结构名固定使用英文：根目录为 `plan/`，跨模块目录为 `plan/cross-module/`，状态前缀为 `pending-` 和 `approved-`。
- `<module>` 和 `<task>` 按语义决定中英文：稳定技术名词优先英文，如 `auth`、`permission`；更自然的业务表达可保留中文，如 `组织`、`菜单权限收口`。
- 纯问答、纯解释、纯代码审查、纯只读排查，不强制生成计划文件。
- 计划目录规则以项目根目录 `plan/README.md` 为准。

## 计划命名规则

- 文件名格式固定为 `pending-<task>.md` 或 `approved-<task>.md`。
- `<task>` 必须直接体现本次执行目标，可用英文技术短语，也可用中文业务短语，但都不能空泛。
- 合格示例：`pending-login-auth-refactor.md`、`pending-菜单权限收口.md`、`pending-组织权限联调.md`。
- 后续若用户直接说“先出计划”，默认先写入对应模块目录下的 `pending-<task>.md`，无需额外指定路径。

## 审查后执行规则

- 生成待审计划后，只允许查代码、读文档、跑非修改型检查；未获明确确认前，不允许实施改动。
- 用户明确确认后，先将计划文件改名为 `approved-<task>.md`，再按计划执行。
- 执行前需要在对话中回报计划路径和摘要，供用户审查。
- 若执行中发现范围明显变化，禁止静默扩项，必须重新生成新的 `pending-<task>.md` 给用户复审。
- 涉及路由、服务、配置、权限或跨模块联调的执行任务，同样必须先经过待审计划流程。

## 提问引用规范

在实现或评审过程中需要向用户澄清时，追加一行：

- `依据规则: #<rule-number>`

若适用多条规则：

- `依据规则: #2,#3,#9`
