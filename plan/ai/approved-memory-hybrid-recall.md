# Memory 混合召回策略实施计划

## Summary

实现第 7 步 `混合召回策略`：把 `summary / facts / RAG / recent turns / tool selector 输入` 从当前分散逻辑收口为统一 planner。核心目标是固定优先级、预算和诊断输出：summary 固定保留，facts 在预算内稳定排序保留，RAG 优先被裁剪，recent turns 至少保留当前意图，tool selector 不会只看到被裁剪到失真的 history。

## Key Changes

- 新增 `aiHybridContextPlanner`，输入 current query、raw history、summary、facts、RAG candidates、visible tools 和配置预算，输出 `runtime history + plan diagnostics`；`aiContextAssembler.Build` 改为通过该 planner 统一生成上下文。
- 调整 memory message：保留 `Conversation Summary`、`Stable Facts`、`Long-term Documents`，删除完整 `Current Query` 分区；当前 query 继续作为本轮最后一条 user message 进入 runtime，不在 memory message 中重复注入。
- 固定上下文优先级：Conversation Summary pinned 保留；Stable Facts 按 `namespace -> source priority -> updated_at DESC` 排序后在 `RecallMaxChars` 内保留；RAG documents 按 Qdrant score 排序并受 `RAGMaxChars` 限制；预算不足时优先裁剪 RAG。
- Recent turns 策略：默认按 `AI.Memory.RecentRawTurns` 保留；即使压缩阈值很小，也至少保留最近 1 轮 user/assistant，当前 query 对应 user message必须保留；如果当前 query 尚未在 stored history 中，则 selector 输入额外显式携带 query。
- Tool selector 输入保障：selector 使用 `current query + hybrid memory summary/facts + retained recent turns`，不依赖可能被裁剪到丢失当前意图的普通 history；tools 仍通过现有 `DynamicSystemPrompt + selected Tools` 注入，不塞进 memory message。
- Diagnostics：planner 返回各来源的候选数、保留数、裁剪数、估算 token/chars、是否触发压缩、recent turns 保留数量、RAG min score/命中数量；本阶段先用于 service 日志和单测断言，后续第 8 步再接 trace/调试接口。

## Test Plan

- Hybrid planner 单测：summary 永远保留；facts 按 namespace/source/updated_at 排序；RAG 按 score 排序；预算不足时先裁剪 RAG，再裁剪 facts，summary 不丢。
- Current query 单测：memory message 不包含 `Current Query` 分区；runtime/selector 输入仍能拿到 current query；极小预算下 current query 和最近至少 1 轮仍保留。
- Compression 单测：未超过阈值时保留 memory + 原始 history；超过阈值时保留 memory + recent turns；`RecentRawTurns` 默认生效。
- Tool selector 集成测试：selector 收到 current query、memory summary/facts、recent turns；不会只收到被压缩后的空/失真 history。
- 回归测试：现有 writeback、context recovery、RAG recall、Qdrant search 测试继续通过；执行 `go test ./internal/service/system ./internal/domain/ai ./internal/infrastructure/ai/memory` 和 `go test ./...`。

## Assumptions

- `source priority` 使用现有 `SourceKind`，优先级为：tool/realtime service > explicit user statement > admin/manual > model inferred > unknown；未识别来源排最后。
- v1 不新增数据库表，不新增 HTTP API，不接 rerank，不开放 org/platform memory。
- planner diagnostics 先作为内部结构和日志字段，不在对外响应中暴露。
