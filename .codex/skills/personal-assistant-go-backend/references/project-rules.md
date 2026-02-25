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

## 提问引用规范

在实现或评审过程中需要向用户澄清时，追加一行：

- `依据规则: #<rule-number>`

若适用多条规则：

- `依据规则: #2,#3,#9`
