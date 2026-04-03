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
   - 只读查询聚合结构放在 `internal/model/readmodel`，不要放入 `internal/model/entity`。
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
13. 权限与投影规则：
   - 权限真相只认 DB 关系表；Casbin 是权限投影，不是业务真相。
   - `pkg/casbin` 只保留纯权限引擎能力，禁止承载 `owner / super_admin / capability 映射` 这类业务判断。
   - `internal/service` 只允许通过 `AuthorizationService` 做业务授权；业务 Service 禁止直接操作 `Enforcer`。
   - `role-menu / role-api / role-capability / menu-api` 变更统一写 DB + outbox，不允许在业务 Service 内直接全量刷新 Casbin。
   - `user-org-role / 成员状态` 变更允许同步收口当前主体投影，但仍必须补发异步修复事件。

## 计划落盘规则

- 只要任务属于新增、重构、修复、联调、排障、迁移、删除、配置调整这类执行型工作，先写计划，不直接改代码。
- 计划文件固定写到 `plan/<module>/pending-<task>.md`。
- 结构名固定使用英文：根目录为 `plan/`，跨模块目录为 `plan/cross-module/`，状态前缀为 `pending-` 和 `approved-`。
- `<module>` 和 `<task>` 按语义决定中英文：稳定技术名词优先英文，如 `auth`、`permission`；更自然的业务表达可保留中文，如 `组织`、`菜单权限收口`。
- 纯问答、纯解释、纯代码审查、纯只读排查，不强制生成计划文件。
- 计划目录规则以 `plan/README.md` 为准。

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

## 提问引用规则

- 向用户提澄清问题时，在问题后追加一行：
  - `依据规则: #<rule-number>`
- 若同时依据多条规则，使用逗号分隔：
  - `依据规则: #2,#3,#9`
