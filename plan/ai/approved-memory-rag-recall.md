# Memory RAG 召回实施计划

## Summary

实现第 6 步 `RAG 召回`：用户发起对话时，用当前 query 生成与索引阶段完全一致的 embedding，在 Qdrant memory collection 中检索 self scope 的长期记忆 chunks，经 MySQL 二次校验后，把命中的内容注入现有 memory message 的 `Long-term Documents` 分区。RAG 失败按 fail open 降级，不影响 summary、facts、recent turns。

## Key Changes

- 扩展配置：新增 `AI.Memory.RecallMinScore float64` 和 `AI.Memory.RAGMaxChars int`，分别绑定 `AI_MEMORY_RECALL_MIN_SCORE`、`AI_MEMORY_RAG_MAX_CHARS`；默认 `RecallMinScore=0.2`，`RAGMaxChars=2000`。
- 扩展 RAG 协议：在 `domain/ai` 增加 query 召回输入、搜索结果、`MemoryVectorSearcher` 接口；query embedding 复用现有 `MemoryEmbedder`，强制使用 `aiMemoryEmbedModel()` 和 `aiMemoryEmbedDimension()`。
- 扩展 Qdrant 检索：在 memory vector store 增加 `SearchChunks`，使用 `qdrant.QueryPoints`；filter 使用 `scope_key=aidomain.BuildSelfMemoryScopeKey(userID)`、`visibility=self`、`user_id=userID`，禁止写成 `self:{userID}`。
- 扩展 MySQL 回查：repository 新增按 `qdrant_point_id` 批量读取 chunks 的方法，并 join documents 二次校验 document 未删除、未过期；service 按 Qdrant score 顺序恢复排序，并过滤低于 `RecallMinScore` 的结果。
- 扩展 `AIMemoryService.Recall`：在 `Conversation Summary`、`Stable Facts` 后追加 `Long-term Documents`；RAG 召回、embedding、Qdrant、MySQL 回查任一失败只记录 warn 并降级为空 RAG，不返回错误中断对话。

## Test Plan

- Qdrant search 单测：验证 collection、query vector、topK、min score 前置数据、self scope filter、visibility/user_id filter。
- Repository 单测：按 point ids 回查 chunks；已删除、过期 document 不返回；service 能按 Qdrant score 顺序恢复排序。
- Service 单测：命中 RAG 时 memory message 包含 `Long-term Documents`；低分结果被过滤；空 query、memory disabled、long-term disabled 不召回；embedding/Qdrant/MySQL 失败时 summary/facts 仍正常返回。
- 集成回归：`aiContextAssembler` 注入 memory 后，runtime history 包含 summary/facts/RAG，并继续被 `CompressMessages` 保留在上下文最前。
- 执行测试：`go test ./internal/service/system ./internal/repository/system ./internal/domain/ai ./internal/infrastructure/ai/memory`，最后跑 `go test ./...`。

## Assumptions

- 本次只做 self memory RAG 召回并注入上下文，不开放 org/platform 召回。
- 本次不接 `qwen3-vl-rerank`，不做混合排序；第 7 步再处理 facts、summary、RAG、tools 的统一预算和重排。
- RAG 正文以 MySQL chunk 表为准，Qdrant 只作为向量检索和 point metadata 来源。
