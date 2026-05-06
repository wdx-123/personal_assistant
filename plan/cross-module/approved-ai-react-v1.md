# 目标

在当前 `domain/ai -> service/system -> infrastructure/ai/eino` 分层内完成自研单 Agent ReAct V1，保持 HTTP 和 domain 稳定协议不变，不接入 ADK、不引入 interrupt/resume/planner，并为后续 `memory / 上下文压缩 / LLM 网关` 预埋接口与装配点。

# 范围

- 保持现有 AI HTTP 路由、控制器、DTO 和 SSE 事件集不变。
- 在 `service/system` 内新增上下文装配扩展点，收口历史消息转换、动态 prompt 构造和未来上下文扩展点。
- 在 `infrastructure/ai/eino` 内稳定当前 ReAct tool loop，并预埋可选 `ChatModelFactory` 注入点。
- 保留现有工具集，不新增业务工具。

# 改动

- 保持 `aidomain.Runtime`、`aidomain.StreamInput`、`aidomain.Event`、`aidomain.Tool` 协议不变。
- 扩展 `AIDeps`，增加可选的 `Memory / Compressor / PromptBuilder` 依赖。
- 新增 `aiMemoryProvider / aiContextCompressor / aiPromptBuilder / aiContextAssembler` 内部协作者。
- 将上下文装配从 `aiSvc.go` 中下沉为独立协作者，但继续由 `AIService.StreamConversation` 负责主编排。
- 继续由 `aiToolRegistry` 负责工具注册、可见性过滤和执行前二次鉴权。
- 规范动态 prompt，固定“只用可见工具、缺参不猜、执行期仍会再鉴权、工具失败不编造结果”等约束。
- 在 `eino` runtime 中保持 `streamTextOnly + streamWithTools` 双路径和单 Agent 顺序 ReAct 循环。
- 对 `chat_model.go` / `options.go` 预埋 `ChatModelFactory` 注入点；默认未注入时沿用现有 provider 分支。
- 补齐 ReAct 相关测试，覆盖无工具、有工具、缺参追问、工具失败、未注入扩展依赖等场景。

# 验证

- `go test ./internal/infrastructure/ai/eino`
- `go test ./internal/service/system`
- 必要时补充针对 AI 相关文件的定向测试用例

# 风险

- 动态 prompt 和上下文装配拆分后，若拼装顺序变动，可能影响模型输出和工具选择行为。
- `ChatModelFactory` 预埋若处理不当，可能破坏现有 `qwen/openai/ark` provider 路径。
- 保持“工具失败即终止当前轮”的既有行为，需确保 trace 和 SSE 错误终态仍能稳定收敛。

# 执行顺序

1. 计划文件落盘并转为 `approved`
2. 拆出 `service/system` 的上下文装配与 prompt 协作者
3. 扩展 `AIDeps` 和 `AIService` 的上下文依赖注入
4. 拆出 `eino` 的 tool schema 适配文件，并预埋 `ChatModelFactory`
5. 补充和修正单元测试
6. 执行定向 `go test`

# 待确认

- 本次缺参场景采用普通多轮对话自然追问，不设计 interrupt/resume 状态机。
- 后续能力仅预埋接口和装配点，不提供默认空实现，不引入当前不可见的新行为分支。
