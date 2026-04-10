# Cross-Module Go 代码生产级中文注释补全计划

## Summary
- 计划文件路径定为 `plan/cross-module/pending-go-code-生产级中文注释补全.md`；获批后改名为 `plan/cross-module/approved-go-code-生产级中文注释补全.md`，再执行注释补充。
- 执行范围锁定为 2026-04-08 当前工作区内所有已修改或未跟踪的 `.go` 文件，共 47 个；不看暂存区，不处理 `.md`、README、`docs/` 等文档文件。
- 交付方式锁定为直接修改仓库文件，并在对话中回报已处理文件分组与验证结果；不在对话里整批粘贴完整代码。

## Key Changes
- 只做注释层改动，不改业务逻辑、变量名、控制流、结构体字段、接口签名、路由、错误码、配置读取方式。
- 按模块分组补注释，优先处理可读性最弱且并发/流式逻辑最重的区域：
  - `internal/infrastructure/sse` 与 `internal/core/sse.go`：重点解释 `context` 取消、goroutine 生命周期、channel 缓冲、锁粒度、心跳、写超时、慢消费者处理、Pub/Sub 订阅退出、资源关闭时机。
  - `internal/service/system` 与 AI 相关 Controller/Repository/DTO/Entity：重点解释会话流启动、interrupt/decision 流程、状态迁移、消息持久化、运行时与 sink 的职责边界。
  - 其余基础与业务文件：`flag`、`global`、`core`、`init`、`router`、`repository`、`service`、`pkg/errors` 等，补齐函数职责、边界条件、错误处理原因和分层目的。
- 每个函数或方法都补完整中文函数注释，至少覆盖：作用、参数、返回值、核心流程、注意事项。
- 超过 20 行的函数，函数体内至少拆成 3 段局部注释；对关键分支、提前返回、错误包装/透传、资源操作、并发控制补“为什么这样做”。
- 对无函数文件补类型、接口、常量、关键字段注释，保证阅读时能理解该文件承担的职责。
- 对意图无法从局部代码完全确认的地方，明确使用“根据上下文推测”标注，避免把猜测写成确定事实。

## API / Type Changes
- 无对外行为变更。
- 无接口、方法签名、结构体字段、配置结构、路由契约变更。
- 仅新增或优化中文文档注释与局部说明性注释，使导出的类型、接口和核心内部结构更易读。

## Test Plan
- 注释补全后对所有触达文件执行 `gofmt`，仅用于保持 Go 语法与注释排版合法。
- 执行已验证可跑的定向检查：
  - `go test ./internal/infrastructure/sse -count=1`
  - `go test ./internal/service/system -run TestAiRuntimeLocal -count=1`
- 对其余受影响包执行编译型 smoke test，确认纯注释改动未引入语法问题：
  - `go test ./flag ./global ./internal/controller/system ./internal/core ./internal/init ./internal/model/... ./internal/repository/... ./internal/router/... ./internal/service/... ./pkg/errors -run '^$' -count=1`

## Assumptions
- 因暂存区为空，本次按“当前工作区所有已修改/未跟踪 Go 文件”执行，这是已确认的范围替代方案。
- 测试文件同样属于代码文件，纳入注释补全范围。
- 若执行中发现新增文件或范围明显变化，需要重新生成新的 `pending-*.md` 计划，不静默扩项。
- 实施顺序固定为：先落待审计划文件，再等你确认；确认后改名为 `approved-*`，随后开始实际补注释。
