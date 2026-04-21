# 目标

将 AI 子域从 Plan、Tool、A2UI、Interrupt/Resume 的复杂 Agent Runtime 收缩为最基础的流式对话闭环：

```text
Controller/Router -> Service -> domain/ai Runtime/Sink/Event -> infrastructure/ai Runtime -> SSE -> DB message 落库
```

# 范围

- 保留现有 MVC 外壳：AI Router、Controller、Service、Repository。
- 新增 `internal/domain/ai` 作为 AI 子域最小稳定协议层。
- 新增/迁移基础 runtime 到 `internal/infrastructure/ai/local` 和 `internal/infrastructure/ai/eino`。
- 保留基础 5 个 AI API 和 SSE 流式输出。
- 删除或停用 A2UI、Plan、Tool、Interrupt、Decision、Resume、Task/Progress/Docs 工具链路。
- 不做 DB 表结构删除；`ai_interrupts` 表和 `trace_items_json/ui_blocks_json/scope_json` 字段先保留兼容。

# 改动

- 新增 domain runtime/sink/event/message 类型，domain 只依赖标准库。
- 将 runtime 契约收缩为 `Name()` 和 `Stream(ctx, input, sink)`。
- Service 不再调用 Plan，不创建 interrupt，只创建 user message 和 assistant message，再调用 runtime stream。
- Sink/projector 只处理 `conversation_started`、`assistant_token`、`message_completed`、`error`、`done`。
- 删除 decision 路由、Controller、Service、DTO 使用。
- 删除旧 planner、intent、control plane、runtimecontrol、Eino tool/approval/checkpoint/docs/task/progress 链路。

# 验证

- `go test ./internal/domain/ai/...`
- `go test ./internal/infrastructure/ai/...`
- `go test ./internal/service/system/...`
- `go test ./internal/controller/system/...`
- `go test ./internal/router/system/...`
- `go test ./...`

# 风险

- 前端若仍依赖 `structured_block` 或 decision API，会出现功能缺口；本阶段由前端同步收缩。
- 历史 interrupt 数据会保留；本阶段不 drop 表，避免数据库迁移扩大范围。
- Eino 当前实现与工具链耦合较深；先实现无工具基础流式 runtime，必要时回退 local。

# 执行顺序

1. 新增 domain 协议和基础 runtime。
2. 接入 Service 最小流式流程。
3. 收缩 HTTP/API 面。
4. 删除旧复杂链路。
5. 逐步运行测试并修复编译问题。

# 待确认

用户已明确要求实施该计划。
