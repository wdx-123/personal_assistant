package interfaces

import (
	"context"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

// AIMemoryRepository 定义记忆模块的最小仓储能力集合。
type AIMemoryRepository interface {
	// WithTx 绑定事务上下文，保持 memory repository 与现有 repository group 的事务风格一致。
	WithTx(tx any) AIMemoryRepository

	// UpsertFact 按 scope_key + namespace + fact_key 唯一键覆盖写入结构化事实。
	UpsertFact(ctx context.Context, fact *entity.AIMemoryFact) error
	// ListFacts 按默认规则读取 facts，并自动过滤过期记录。
	ListFacts(ctx context.Context, query aidomain.MemoryFactQuery) ([]*entity.AIMemoryFact, error)

	// BatchUpsertDocuments 批量写入或覆盖长期记忆文档元数据。
	BatchUpsertDocuments(ctx context.Context, docs []*entity.AIMemoryDocument) error
	// ListDocuments 按默认规则读取 documents，并自动过滤软删除和过期记录。
	ListDocuments(ctx context.Context, query aidomain.MemoryDocumentQuery) ([]*entity.AIMemoryDocument, error)
	// ListDocumentsByIDs 按 document id 读取仍有效的 documents。
	ListDocumentsByIDs(ctx context.Context, ids []string) ([]*entity.AIMemoryDocument, error)
	// ListDocumentsNeedingIndex 扫描需要建立或重建向量索引的 documents。
	ListDocumentsNeedingIndex(ctx context.Context, limit int, embedModel string, dimension int) ([]*entity.AIMemoryDocument, error)
	// ReplaceDocumentChunks 按 document 覆盖保存最新 chunks。
	ReplaceDocumentChunks(ctx context.Context, documentID string, chunks []*entity.AIMemoryDocumentChunk) error
	// ListDocumentChunks 按 document 读取 chunks，按 chunk_index 升序返回。
	ListDocumentChunks(ctx context.Context, documentID string) ([]*entity.AIMemoryDocumentChunk, error)
	// ListDocumentChunksByPointIDs 按 Qdrant point ids 回查仍有效的 chunks。
	ListDocumentChunksByPointIDs(ctx context.Context, pointIDs []string) ([]*entity.AIMemoryDocumentChunk, error)

	// GetConversationSummary 按 conversation_id + user_id + org_id + scope_key 读取当前有效摘要。
	GetConversationSummary(ctx context.Context, query aidomain.MemoryConversationSummaryQuery) (*entity.AIConversationSummary, error)
	// UpsertConversationSummary 按 conversation_id 覆盖更新会话摘要。
	UpsertConversationSummary(ctx context.Context, summary *entity.AIConversationSummary) error
}
