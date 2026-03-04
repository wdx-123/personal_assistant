# AGENTS.md

## 作用范围

仅在以下目录工作时应用本规则：

- `d:/workspace_go/personal_assistant`

## Skill 联动

- 在本仓库进行 Go 后端开发、重构、评审和排障时，优先触发 `$personal-assistant-go-backend`。
- 若任务不属于本仓库后端范围，使用 Codex 默认行为。

## Go 后端规则

1. 分层与边界：
   - `internal/controller` 只负责接参、校验、调用 Service、组装响应。
   - `internal/service` 只负责业务编排。
   - `internal/repository` 负责全部 DB CRUD/JOIN。
   - 禁止在 Controller 写业务逻辑。
   - 禁止 Service 直连 DB。
2. DTO 使用：
   - 请求 DTO 放在 `internal/model/dto/request`。
   - 响应 DTO 放在 `internal/model/dto/response`。
   - 禁止直接暴露 Entity 作为 API 入参/出参。
   - 统一使用 `pkg/response` 的响应辅助函数。
3. 错误与日志链路：
   - 底层无法处理的错误立即返回原始 `error`。
   - Controller 统一使用 `global.Log.Error` 记录日志。
   - 返回统一错误响应。
4. Context 传递：
   - DB 操作和长链路调用必须传递 `context.Context`。
5. 实现顺序：
   - 必须按 `DTO -> Repository -> Service -> Controller -> Router` 实现。
6. 路由组织：
   - `internal/router/router.go` 只负责路由组和中间件挂载。
   - 业务路由按领域拆分在 `internal/router/system`。
   - 每个模块提供 `InitXRouter(*gin.RouterGroup)`。
   - 统一通过 `GroupApp.System` 注册。
7. 初始化布局：
   - 具体初始化逻辑放在 `internal/core`。
   - `internal/init/init.go` 仅做编排。
   - 禁止重复创建或重复启动同一组件。
8. 注释要求：
   - 对非显而易见的逻辑补充必要的生产级注释。
9. 禁止硬编码：
   - 可变参数定义在 `internal/model/config`。
   - 配置由 `configs` 驱动，业务代码通过 `global.Config` 读取。
   - 仅允许保留必要的零值兜底。
10. 新功能业务错误策略：
    - Repository：只返回原始 `error`。
    - Service：使用 `pkg/errors` 包装 BizError：
      - `errors.New(code)`
      - `errors.NewWithMsg(code, msg)`
      - `errors.Wrap(code, cause)`
      - `errors.WrapWithMsg(code, msg, cause)`
    - Controller：`global.Log.Error()` 记录日志，并用 `response.BizFailWithError(err, c)` 返回。
    - 响应辅助函数只负责响应，不耦合日志。
    - 错误码分段遵循 `pkg/errors/codes.go`：
      - `1xxxx` 通用
      - `2xxxx` 用户
      - `3xxxx` 组织与权限
      - `4xxxx` OJ
11. 复用 `pkg` 公共能力：
    - 新增工具前先复用已有 `pkg/*`。
    - Controller 获取当前用户 ID 必须使用 `pkg/jwt.GetUserID(c)`。
    - 业务无关的纯函数放 `pkg/util`。
    - 跨模块、无业务上下文依赖的逻辑应从 Service 下沉到 `pkg`。
12. 新增独立基础设施能力（如 Trace/Metric/Audit/限流适配/消息客户端）：
    - 目标是跨模块复用，并与具体业务领域解耦。
    - 目录与职责：
      - 初始化、连接创建、生命周期管理放在 `internal/core`。
      - `internal/init/init.go` 仅做依赖编排与启动顺序控制。
      - 通用封装放在 `pkg/*`，禁止在多个业务 Service 内重复造轮子。
      - 若需暴露设施路由（如 `/metrics`），在 `internal/router/system` 定义，`internal/router/router.go` 只做挂载。
    - 接入边界：
      - Controller 禁止直接依赖第三方基础设施 SDK。
      - Service 禁止直接 `new` 第三方客户端；统一通过 `pkg/*` 封装或已初始化实例接入。
      - Repository 仅负责数据访问，不承担基础设施编排。
    - 配置与开关：
      - endpoint、timeout、batch、采样率、开关等可变参数必须定义在 `internal/model/config`。
      - 业务代码统一通过 `global.Config` 读取，仅允许必要零值兜底。
      - `pkg/*` 若需可复用配置，定义模块内 `Options` 结构体；由 `core`/`init` 将 `global.Config` 映射为 `Options` 后注入。
      - 禁止在 `pkg/*` 直接依赖 `viper` 或 `global.Config`。
    - 错误策略：
      - 初始化阶段失败应返回原始 `error`，由上层决定 fail-fast。
      - 运行阶段可降级的失败必须记录 `global.Log.Error`，禁止静默吞错。
    - Redis/MySQL 等基础资源边界：
      - 连接初始化、连接池参数、健康检查和关闭动作属于运行时基础设施，放在 `internal/core`（或等价基础设施启动层）。
      - 业务 CRUD/JOIN 仍只允许在 `internal/repository` 实现，禁止下沉到“基础设施通用层”。
      - Service 仅通过 Repository 或 `pkg/*` 抽象能力使用资源，不直接编排底层连接细节。
    - 实施顺序：
      - `Config -> Core -> Init -> pkg封装/中间件 -> Service接入（如需要） -> Router暴露（如需要）`

## 提问引用规则

- 向用户提澄清问题时，在问题后追加一行：
  - `依据规则: #<rule-number>`
- 若同时依据多条规则，使用逗号分隔：
  - `依据规则: #2,#3,#9`
