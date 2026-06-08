# 目标

实现 AI 记忆第 4 步“上下文压缩，以及上下文恢复”：将长会话输入从“全量历史”改为“conversation summary + facts + recent turns”，在下一轮请求进入 runtime 前恢复必要上下文。

# 范围

- 只实现读侧上下文恢复和压缩，不实现 RAG 向量召回。
- 复用第 3 步写入的 `AIConversationSummary` 与 `AIMemoryFact`。
- 不新增 HTTP API，不改 Controller/Router。
- 不新增数据库表。
- document 记忆暂不进入 prompt；第 5/6 步 RAG 切分与召回再接入。

# 改动

- 改造 `AIMemoryService`：
  - 实现 `Recall(ctx, aiMemoryRecallInput)`。
  - 新增 `RecallMessages(ctx, aiMemoryRecallInput)`，满足现有 `aiMemoryProvider`。
  - 新增 `CompressMessages(ctx, aiContextCompressionInput)`，满足现有 `aiContextCompressor`。
- 在 `AIMemoryService.Recall` 中：
  - 计算 self scope，读取当前会话 summary。
  - 读取 self 可见 facts。
  - 将 summary/facts 转成稳定的 memory context message。
  - 不读取 org/platform 记忆，避免权限语义扩大。
- 在 `CompressMessages` 中：
  - 按 `global.Config.AI.Memory.RecentRawTurns` 保留最近 N 轮原始消息。
  - 如果消息未超过阈值，保持原样。
  - 如果已存在 memory message，则保持其在前，后接 recent turns。
- 改造 service 注入：
  - 在 `SetUp` 中把同一个 `AIMemoryService` 同时注入 `AIDeps.Memory`、`AIDeps.Compressor`、`AIDeps.Writeback`。
  - 保证 `aiContextAssembler.Build` 进入 runtime 前自动执行 recall + compress。
- 补充测试：
  - memory disabled 时上下文不变。
  - 有 summary/facts 时生成 memory message。
  - 长历史只保留 recent turns，并保留 memory message。
  - `go test ./internal/service/system ./internal/repository/system ./internal/domain/ai` 通过。

# 验证

- 运行目标测试：
  - `go test ./internal/service/system ./internal/repository/system ./internal/domain/ai`
- 完成后再运行：
  - `go test ./...`

# 风险

- v1 将 summary/facts 作为一条 synthetic system-style context message 注入；当前 domain 只有 user/assistant role，需采用 assistant role 或扩展 role。优先不扩 runtime 协议，使用 assistant role 且内容带明显边界。
- summary 质量仍取决于第 3 步规则摘要，后续可替换为真正 compressor。
- 只接 self 记忆，org/platform 记忆需要后续授权能力明确后再开。

# 执行顺序

1. 用户确认计划后，将本文件改名为 `approved-memory-context-recovery.md`。
2. 实现 `AIMemoryService.RecallMessages` 和 `Recall`。
3. 实现 `AIMemoryService.CompressMessages`。
4. 在 `SetUp` 中注入 Memory/Compressor。
5. 补单测并运行验证。

# 待确认

等待用户明确确认后执行。
