1. **分层**：Controller（`internal/controller`）仅接收请求/参数校验→调用 Service→组装响应，**禁止**业务逻辑/直连DB；
    Service（`internal/service`）只做业务编排（结构体方法，
    **不定义 interface**），**禁止**直连DB（必须调 Repository）；
    Repository（`internal/repository`）负责**全部**数据库交互（CRUD/JOIN）。
2. **DTO**：请求/响应分别放 `internal/model/dto/request`、`internal/model/dto/response`，不存在就新建，
    **禁止**直接用 Entity/散装参数。
    统一响应用 `pkg/response`：`NewResponse[T,R](c).Success/Failed`。
3. **错误与日志**：底层遇到无法处理的错误
    **立刻 return error**；
    Controller **统一捕获**→`global.Log.Error`记录→返回 `Failed`。
4. **Context**：DB操作/长链路调用必须传 `context.Context`。
5. **实现顺序**：DTO → Repository → Service → Controller → Router（`internal/router`）。
6. 路由入口 `router.go` 仅建 Group/挂中间件；具体业务路由按领域拆到 `internal/router/system`，每个模块提供 `InitXRouter(*gin.RouterGroup)`；入口通过 `GroupApp.System` 统一调用注册。
