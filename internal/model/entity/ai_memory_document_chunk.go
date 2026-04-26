package entity

import "time"

// AIMemoryDocumentChunk 表示长期记忆文档切分后的可向量化片段。
type AIMemoryDocumentChunk struct {
	// ID 是 chunk 的稳定主键，由 document_id、chunk_index 和内容 hash 生成。
	ID string `json:"id" gorm:"type:varchar(64);primaryKey;comment:'记忆文档chunk ID'"`
	// DocumentID 关联 ai_memory_documents.id。
	DocumentID string `json:"document_id" gorm:"type:varchar(64);not null;index:idx_ai_memory_document_chunks_document_id,priority:1;index:idx_ai_memory_document_chunks_document_index,priority:1;comment:'记忆文档ID'"`
	// ScopeKey 是权限过滤所需的归属键。
	ScopeKey string `json:"scope_key" gorm:"type:varchar(128);not null;index:idx_ai_memory_document_chunks_scope_key_memory_type,priority:1;comment:'记忆作用域键'"`
	// ScopeType 冗余保存作用域类型。
	ScopeType string `json:"scope_type" gorm:"type:varchar(32);not null;comment:'记忆作用域类型'"`
	// Visibility 表示访问等级。
	Visibility string `json:"visibility" gorm:"type:varchar(32);not null;comment:'记忆访问等级'"`
	// UserID 是关联用户快照。
	UserID *uint `json:"user_id,omitempty" gorm:"index;comment:'关联用户ID'"`
	// OrgID 是关联组织快照。
	OrgID *uint `json:"org_id,omitempty" gorm:"index;comment:'关联组织ID'"`
	// MemoryType 标记长期记忆类型。
	MemoryType string `json:"memory_type" gorm:"type:varchar(32);not null;index:idx_ai_memory_document_chunks_scope_key_memory_type,priority:2;comment:'长期记忆类型'"`
	// Topic 是轻量主题标签。
	Topic string `json:"topic" gorm:"type:varchar(128);not null;default:'';index:idx_ai_memory_document_chunks_topic,comment:'主题'"`
	// ChunkIndex 表示 chunk 在文档内的顺序。
	ChunkIndex int `json:"chunk_index" gorm:"not null;index:idx_ai_memory_document_chunks_document_index,priority:2;comment:'chunk顺序'"`
	// ContentText 是参与 embedding 的 chunk 正文。
	ContentText string `json:"content_text" gorm:"type:longtext;not null;comment:'chunk正文'"`
	// ContentHash 是规范化 chunk 正文后的稳定 hash。
	ContentHash string `json:"content_hash" gorm:"type:char(64);not null;default:'';comment:'chunk内容hash'"`
	// TokenEstimate 是轻量 token 估算值。
	TokenEstimate int `json:"token_estimate" gorm:"not null;default:0;comment:'token估算值'"`
	// EmbeddingModel 记录生成该 chunk 向量时使用的模型。
	EmbeddingModel string `json:"embedding_model" gorm:"type:varchar(128);not null;default:'';index:idx_ai_memory_document_chunks_embedding,priority:1;comment:'Embedding模型名'"`
	// EmbeddingDimension 记录向量维度。
	EmbeddingDimension int `json:"embedding_dimension" gorm:"not null;default:0;index:idx_ai_memory_document_chunks_embedding,priority:2;comment:'Embedding维度'"`
	// QdrantPointID 是该 chunk 在 Qdrant 中的 point id。
	QdrantPointID string `json:"qdrant_point_id" gorm:"type:varchar(128);not null;default:'';uniqueIndex:uk_ai_memory_document_chunks_qdrant_point_id;comment:'Qdrant point ID'"`
	// IndexedAt 表示该 chunk 完成 Qdrant 写入的时间。
	IndexedAt *time.Time `json:"indexed_at,omitempty" gorm:"type:datetime;index;comment:'向量索引时间'"`
	// CreatedAt 表示首次写入时间。
	CreatedAt time.Time `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	// UpdatedAt 表示最近一次更新时间。
	UpdatedAt time.Time `json:"updated_at" gorm:"type:datetime;not null;comment:'更新时间'"`
}

// TableName 返回记忆文档 chunk 表名。
func (AIMemoryDocumentChunk) TableName() string {
	return "ai_memory_document_chunks"
}
