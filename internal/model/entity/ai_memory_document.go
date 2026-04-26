package entity

import (
	"time"

	"gorm.io/gorm"
)

// AIMemoryDocument 表示可语义召回的长期记忆文档。
type AIMemoryDocument struct {
	// ID 是文档记录的稳定主键。
	// 当前阶段用它作为 upsert 键；后续如引入 chunk 表，也仍然需要它作为文档根 ID。
	ID string `json:"id" gorm:"type:varchar(64);primaryKey;comment:'记忆文档ID'"`
	// ScopeKey 决定文档归属到哪个记忆范围，例如个人、组织或平台运维域。
	ScopeKey string `json:"scope_key" gorm:"type:varchar(128);not null;index:idx_ai_memory_documents_scope_key_memory_type_updated_at,priority:1;uniqueIndex:uk_ai_memory_documents_scope_key_dedup_key,priority:1;comment:'记忆作用域键'"`
	// ScopeType 冗余保存作用域类型，方便过滤和调试，不必每次从 ScopeKey 字符串反推。
	ScopeType string `json:"scope_type" gorm:"type:varchar(32);not null;comment:'记忆作用域类型'"`
	// Visibility 表示这份文档允许被哪些主体读取。
	// 未来召回时会先做 scope 过滤，再做 visibility 过滤。
	Visibility string `json:"visibility" gorm:"type:varchar(32);not null;comment:'记忆访问等级'"`
	// UserID 是关联用户快照，主要服务于调试、清理和权限辅助判断。
	UserID *uint `json:"user_id,omitempty" gorm:"index;comment:'关联用户ID'"`
	// OrgID 是关联组织快照，便于做组织级召回和后台管理查询。
	OrgID *uint `json:"org_id,omitempty" gorm:"index;comment:'关联组织ID'"`
	// MemoryType 标记这份文档属于哪一类长期记忆。
	// 后续可以用它区分 semantic、episodic、procedural、incident、faq 等不同召回策略。
	MemoryType string `json:"memory_type" gorm:"type:varchar(32);not null;index:idx_ai_memory_documents_scope_key_memory_type_updated_at,priority:2;comment:'长期记忆类型'"`
	// Topic 是对文档主题的轻量标签，适合做简单过滤、聚合和调试展示。
	Topic string `json:"topic" gorm:"type:varchar(128);not null;default:'';index:idx_ai_memory_documents_topic_updated_at,priority:1;comment:'主题'"`
	// Title 是文档标题，主要用于人类可读展示和后台排查。
	Title string `json:"title" gorm:"type:varchar(255);not null;default:'';comment:'标题'"`
	// Summary 是文档的短摘要，用于低成本展示、排序和 prompt 拼装前的预览。
	Summary string `json:"summary" gorm:"type:varchar(1000);not null;default:'';comment:'摘要'"`
	// ContentText 是实际参与召回和后续 embedding 的正文内容。
	// 当前阶段按单文档存储，后续多 chunk 阶段会从这里切分出 chunk 子表。
	ContentText string `json:"content_text" gorm:"type:longtext;not null;comment:'召回文本内容'"`
	// ContentHash 是规范化正文内容后的稳定哈希。
	// 第一版规则去重优先看 source/topic 组合，缺失时再回退到内容哈希。
	ContentHash string `json:"content_hash" gorm:"type:char(64);not null;default:'';comment:'正文内容哈希'"`
	// SummaryHash 是规范化摘要后的稳定哈希。
	// 当缺少 source 标识时，规则去重会优先回退到摘要哈希。
	SummaryHash string `json:"summary_hash" gorm:"type:char(64);not null;default:'';comment:'摘要哈希'"`
	// DedupKey 是第一版规则去重的统一键。
	// 它按 source/topic 组合或摘要/正文哈希生成，并在同一 scope 下保持唯一。
	DedupKey string `json:"dedup_key" gorm:"type:varchar(255);not null;default:'';uniqueIndex:uk_ai_memory_documents_scope_key_dedup_key,priority:2;comment:'规则去重键'"`
	// Importance 表示这份文档的重要度，用于后续治理和召回排序。
	Importance float64 `json:"importance" gorm:"type:decimal(5,4);not null;default:0;comment:'重要度'"`
	// QualityScore 表示这份文档的质量分，例如结构完整度、噪音程度或人工校验结果。
	QualityScore float64 `json:"quality_score" gorm:"type:decimal(5,4);not null;default:0;comment:'质量分'"`
	// EmbeddingModel 记录这份文档对应的向量模型。
	// 这样后续切模型或做重建时，可以判断旧索引是不是需要重算。
	EmbeddingModel string `json:"embedding_model" gorm:"type:varchar(128);not null;default:'';comment:'Embedding模型名'"`
	// QdrantPointID 是单 chunk 模式下的兼容字段。
	// 当前只保留一个 point id；进入多 chunk 阶段后，会拆到独立 chunk 表管理。
	QdrantPointID string `json:"qdrant_point_id" gorm:"type:varchar(128);not null;default:'';comment:'Qdrant单chunk兼容point ID'"`
	// SourceKind 表示文档来源，例如 conversation_summary、faq_import、incident_postmortem。
	SourceKind string `json:"source_kind" gorm:"type:varchar(64);not null;default:'';comment:'来源类型'"`
	// SourceID 记录来源对象标识，便于后续去重、回源和追踪。
	SourceID string `json:"source_id" gorm:"type:varchar(128);not null;default:'';comment:'来源ID'"`
	// EffectiveAt 表示文档从什么时候开始可参与召回；为空时表示立即可用。
	EffectiveAt *time.Time `json:"effective_at,omitempty" gorm:"type:datetime;comment:'生效时间'"`
	// ExpiresAt 表示文档何时过期。
	// repository 默认会过滤过期文档，避免旧 runbook 或旧 FAQ 继续污染召回结果。
	ExpiresAt *time.Time `json:"expires_at,omitempty" gorm:"type:datetime;index:idx_ai_memory_documents_expires_at;comment:'过期时间'"`
	// CreatedAt 表示文档首次写入时间。
	CreatedAt time.Time `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	// UpdatedAt 表示文档最近一次更新或重建时间。
	UpdatedAt time.Time `json:"updated_at" gorm:"type:datetime;not null;index:idx_ai_memory_documents_scope_key_memory_type_updated_at,priority:3;index:idx_ai_memory_documents_topic_updated_at,priority:2;comment:'更新时间'"`
	// DeletedAt 使用 GORM 软删除。
	// documents 与 facts 不同，它更适合“逻辑失效”而不是唯一键覆盖，所以保留软删除能力。
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:'软删除时间'"`
}

// TableName 返回长期记忆文档表名。
func (AIMemoryDocument) TableName() string {
	return "ai_memory_documents"
}
