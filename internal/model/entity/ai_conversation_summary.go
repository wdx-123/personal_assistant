package entity

import "time"

// AIConversationSummary 表示会话级压缩摘要。
type AIConversationSummary struct {
	// ConversationID 是会话主键，同时也是 summary 表的主键。
	// 一个会话只保留一份当前有效摘要，因此后续更新按 conversation_id 直接覆盖。
	ConversationID string `json:"conversation_id" gorm:"type:varchar(64);primaryKey;comment:'会话ID'"`
	// UserID 是会话所属用户快照。
	// 即使 ScopeKey 已能表达归属，保留显式 user_id 仍有利于权限校验和后台检索。
	UserID uint `json:"user_id" gorm:"not null;index:idx_ai_conversation_summaries_user_id;comment:'所属用户ID'"`
	// OrgID 是会话所属组织快照。
	// 会话可能发生在组织上下文内，因此需要保留组织维度辅助恢复和清理。
	OrgID *uint `json:"org_id,omitempty" gorm:"index:idx_ai_conversation_summaries_org_id;comment:'所属组织ID快照'"`
	// ScopeKey 是会话摘要归属范围的统一键。
	// 后续 summary 读取时也必须遵守与 facts/documents 相同的作用域口径。
	ScopeKey string `json:"scope_key" gorm:"type:varchar(128);not null;index:idx_ai_conversation_summaries_scope_key;comment:'会话摘要作用域键'"`
	// CompressedUntilMessageID 记录摘要已经覆盖到哪一条消息。
	// 后续恢复上下文时，会用 “summary + 该消息之后的 recent turns” 重建会话状态。
	CompressedUntilMessageID string `json:"compressed_until_message_id" gorm:"type:varchar(64);not null;default:'';comment:'压缩截至消息ID'"`
	// SummaryText 是给模型直接读取的压缩后正文。
	SummaryText string `json:"summary_text" gorm:"type:longtext;not null;comment:'压缩摘要文本'"`
	// KeyPointsJSON 保存摘要中的关键结论列表。
	// 它适合在后续调试、观察和更精细的上下文恢复策略里单独消费。
	KeyPointsJSON string `json:"key_points_json" gorm:"type:longtext;not null;comment:'关键点JSON'"`
	// OpenLoopsJSON 保存当前尚未闭环的问题、待办或未确认信息。
	// 这类信息往往比普通摘要更影响下一轮对话连续性。
	OpenLoopsJSON string `json:"open_loops_json" gorm:"type:longtext;not null;comment:'未闭环事项JSON'"`
	// TokenEstimate 记录当前摘要大致占用的 token 数量。
	// 后续压缩调优和 prompt 预算控制会依赖这个估算值。
	TokenEstimate int `json:"token_estimate" gorm:"not null;default:0;comment:'token估算值'"`
	// CreatedAt 表示摘要首次创建时间。
	CreatedAt time.Time `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	// UpdatedAt 表示摘要最近一次刷新时间。
	// summary 按 conversation_id 覆盖更新，因此这个时间能直观看出最后一次压缩发生在何时。
	UpdatedAt time.Time `json:"updated_at" gorm:"type:datetime;not null;index:idx_ai_conversation_summaries_updated_at;comment:'更新时间'"`
}

// TableName 返回会话摘要表名。
func (AIConversationSummary) TableName() string {
	return "ai_conversation_summaries"
}
