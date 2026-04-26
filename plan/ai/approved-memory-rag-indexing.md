# Memory RAG 切分入库实施计划

## Summary

实现第 5 步 `RAG 切分入库`：把第 3 步写入 MySQL 的 `AIMemoryDocument` 切成 chunks，使用阿里云百炼 `qwen3-vl-embedding` 生成 1024 维向量，写入 Qdrant `MemoryCollectionName`，并在 MySQL 保存 chunk 与 point 映射。

本阶段只建立“可召回索引”，不把 RAG 召回接入对话上下文；召回接入放到第 6 步。

## Key Changes

- 新增 `AIMemoryDocumentChunk`，表名 `ai_memory_document_chunks`，保存 chunk 文本、hash、embedding 模型、维度、Qdrant point、索引时间和权限过滤字段。
- 扩展 memory repository：支持扫描待索引 documents、按 document 覆盖 chunks、读取 document chunks。
- 在 `domain/ai` 定义 `MemoryChunker`、`MemoryEmbedder`、`MemoryVectorStore`、`MemoryDocumentIndexer`。
- 在 `infrastructure/ai/memory` 实现 paragraph-aware chunker、DashScope multimodal embedding client、Qdrant memory vector store。
- 扩展 `AI.Memory` 配置：`EmbedEndpoint`、`EmbedDimension`、`ChunkMaxChars`、`ChunkOverlapChars`、`IndexBatchSize`、`IndexTimeoutSeconds`。
- Qdrant 启动时确保 `MemoryCollectionName` 存在，维度匹配 `AI.Memory.EmbedDimension`。
- 在 `AIMemoryService` 增加 `IndexDocuments` 与 `IndexPendingDocuments`。
- writeback 成功写入 documents 后异步触发索引，失败只记录日志；补偿入口由 `IndexPendingDocuments` 扫描未索引 documents。

## Test Plan

- chunker 单测：短文本、长文本、overlap、空文本。
- DashScope embedding client 单测：请求体、正常响应、维度不匹配、缺少 APIKey/model。
- Qdrant vector store 单测：collection、point id、payload、旧 points 删除。
- repository 单测：chunks 覆盖、待索引扫描、内容更新重建、过期/删除过滤。
- service 集成单测：索引成功、embedding 失败、Qdrant 失败、writeback 异步失败不影响主流程。
- 回归：`go test ./internal/service/system ./internal/repository/system ./internal/domain/ai ./internal/infrastructure/ai/memory` 和 `go test ./...`。

## Assumptions

- 本阶段只做索引建设，不做 RAG 召回注入。
- 默认模型锁定为 `qwen3-vl-embedding + dimension=1024`。
- 只处理 writeback 产生的 self documents；org/platform 召回权限后续再接。
- 不引入 outbox 事件；可靠性先靠 `IndexPendingDocuments` 补偿扫描。
