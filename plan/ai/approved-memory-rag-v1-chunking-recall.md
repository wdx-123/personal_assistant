# AI Memory RAG V1 切分与召回升级

## 目标

- 把当前 `段落 + 字符窗口` 升级为 `段落优先 -> 强句界 -> 软边界 -> 硬切` 的分层 chunking。
- 把 recall 从“只注入命中的单个 chunk”升级为固定邻块扩窗 `k-1/k/k+1`。
- 不改 HTTP / Router / DTO / 配置键，继续复用现有 `ChunkMaxChars`、`ChunkOverlapChars`、`RecallTopK`、`RAGMaxChars`。

## 范围

- `internal/infrastructure/ai/memory/chunker.go`
- `internal/domain/ai`
- `internal/repository/interfaces`
- `internal/repository/system/aiMemoryRepo.go`
- `internal/service/system/aiMemoryRecall.go`
- `internal/service/system/aiHybridContext.go`
- 对应测试文件

## 改动

- 保留 `ParagraphChunker` 对外入口，内部引入 block 识别、prose/list/table/code 分层切分和 V1 overlap 去重 helper。
- 新增内部查询类型 `MemoryDocumentChunkRef{DocumentID, ChunkIndex}`。
- 在 `AIMemoryRepository` 增加 `ListDocumentChunksByRefs(ctx, refs)`，用于批量回查邻接 chunk。
- recall 保留现有向量召回口径，升级为 primary chunk 命中后按 `k-1/k/k+1` 扩窗、去重、合并并注入。
- `renderAIMemoryRAGLine` 优先渲染扩窗后的合并文本，预算仍由 `RAGMaxChars` 控制。

## 验证

- `go test ./internal/infrastructure/ai/memory ./internal/repository/system ./internal/service/system`

## 风险

- 代码块、表格和列表切分逻辑变复杂，需用测试锁住边界行为。
- 邻块扩窗若不做 overlap 去重，会造成重复上下文；实现时必须去重后再注入。

## 执行顺序

1. 实现 chunker 分层切分与 overlap helper。
2. 扩展 domain / repository 邻块回查接口与实现。
3. 升级 recall 邻块扩窗、合并和渲染逻辑。
4. 更新索引与召回相关测试。
5. 跑目标测试集验证。

## 待确认

- 无。本计划已由用户确认执行。
