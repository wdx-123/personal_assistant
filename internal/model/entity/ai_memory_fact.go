package entity

import "time"

// AIMemoryFact 表示结构化稳定事实记忆。
type AIMemoryFact struct {
	// ID 是事实记录的数据库主键，仅用于表内唯一标识和更新审计。
	ID uint `json:"id" gorm:"primaryKey;comment:'记忆事实主键ID'"`
	// ScopeKey 是统一生成的归属键，决定这条事实属于哪个主体范围。
	// 典型格式包括 self:user:{user_id}、org:{org_id}、platform_ops。
	ScopeKey string `json:"scope_key" gorm:"type:varchar(128);not null;uniqueIndex:uk_ai_memory_facts_scope_key_namespace_fact_key,priority:1;index:idx_ai_memory_facts_scope_key_updated_at,priority:1;comment:'记忆作用域键'"`
	// ScopeType 冗余保存作用域类型，避免调用方每次都从 ScopeKey 反解析归属级别。
	ScopeType string `json:"scope_type" gorm:"type:varchar(32);not null;comment:'记忆作用域类型'"`
	// Visibility 表示访问等级，决定当前主体是否被允许读取这条事实。
	// 它和 ScopeKey 一起构成最终的授权边界。
	Visibility string `json:"visibility" gorm:"type:varchar(32);not null;comment:'记忆访问等级'"`
	// UserID 是关联用户快照，便于权限校验、清理任务和后台调试使用。
	UserID *uint `json:"user_id,omitempty" gorm:"index;comment:'关联用户ID'"`
	// OrgID 是关联组织快照，便于组织级记忆过滤、清理和调试使用。
	OrgID *uint `json:"org_id,omitempty" gorm:"index;comment:'关联组织ID'"`
	// Namespace 表示事实所属的业务命名空间，例如 user_preference、oj_profile。
	// 后续治理和召回会先按 namespace 分层处理，再按 fact_key 精确定位。
	Namespace string `json:"namespace" gorm:"type:varchar(64);not null;uniqueIndex:uk_ai_memory_facts_scope_key_namespace_fact_key,priority:2;comment:'事实命名空间'"`
	// FactKey 是 namespace 下的具体键，例如 answer_style、current_goal。
	FactKey string `json:"fact_key" gorm:"type:varchar(64);not null;uniqueIndex:uk_ai_memory_facts_scope_key_namespace_fact_key,priority:3;comment:'事实键'"`
	// FactValueJSON 保存结构化事实值本体。
	// 使用 JSON 而不是拆散成多列，是为了兼容不同 namespace 的异构字段结构。
	FactValueJSON string `json:"fact_value_json" gorm:"type:longtext;not null;comment:'事实值JSON'"`
	// Summary 是给调试、回显和后续 prompt 装配用的可读摘要。
	// 它不是真相源，只是对 FactValueJSON 的人类可读表达。
	Summary string `json:"summary" gorm:"type:varchar(500);not null;default:'';comment:'事实摘要'"`
	// Confidence 表示这条事实的置信度。
	// 后续治理阶段会根据来源和置信度决定是否允许写入、覆盖或召回。
	Confidence float64 `json:"confidence" gorm:"type:decimal(5,4);not null;default:0;comment:'事实置信度'"`
	// SourceKind 表示事实来源类型，例如 conversation、tool_result、admin_set。
	SourceKind string `json:"source_kind" gorm:"type:varchar(64);not null;default:'';comment:'事实来源类型'"`
	// SourceID 记录来源对象主键或消息 ID，便于追溯事实是从哪一次输入或事件抽取出来的。
	SourceID string `json:"source_id" gorm:"type:varchar(128);not null;default:'';comment:'事实来源ID'"`
	// EffectiveAt 表示这条事实从什么时间开始生效；为空时表示立即生效。
	EffectiveAt *time.Time `json:"effective_at,omitempty" gorm:"type:datetime;comment:'生效时间'"`
	// ExpiresAt 表示这条事实的失效时间。
	// repository 默认会过滤过期记录，避免旧目标、旧状态继续污染后续召回。
	ExpiresAt *time.Time `json:"expires_at,omitempty" gorm:"type:datetime;index:idx_ai_memory_facts_expires_at;comment:'过期时间'"`
	// CreatedAt 表示首次写入时间。
	CreatedAt time.Time `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	// UpdatedAt 表示最近一次覆盖更新时间。
	// facts 不做软删除，后续同唯一键写入会直接覆盖并刷新这个时间。
	UpdatedAt time.Time `json:"updated_at" gorm:"type:datetime;not null;index:idx_ai_memory_facts_scope_key_updated_at,priority:2;comment:'更新时间'"`
}

// TableName 返回事实记忆表名。
func (AIMemoryFact) TableName() string {
	return "ai_memory_facts"
}
