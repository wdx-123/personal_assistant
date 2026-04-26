package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MemorySourceKind 表示记忆候选内容的来源类型。
type MemorySourceKind string

const (
	MemorySourceExplicitUserStatement MemorySourceKind = "explicit_user_statement"
	MemorySourceAdminSet              MemorySourceKind = "admin_set"
	MemorySourceToolVerifiedSummary   MemorySourceKind = "tool_verified_summary"
	MemorySourceModelInferred         MemorySourceKind = "model_inferred"
	MemorySourceRawTracePayload       MemorySourceKind = "raw_trace_payload"
	MemorySourceFullToolOutput        MemorySourceKind = "full_tool_output"
)

const (
	MemoryReasonAllowSelfScope        = "allow_self_scope"
	MemoryReasonAllowOrgScope         = "allow_org_scope"
	MemoryReasonAllowPlatformOpsScope = "allow_platform_ops_scope"

	MemoryReasonDenyPermissionDependencyMissing = "deny_permission_dependency_missing"
	MemoryReasonDenyScopeMismatch               = "deny_scope_mismatch"
	MemoryReasonDenyVisibilityMismatch          = "deny_visibility_mismatch"
	MemoryReasonDenyNotSuperAdmin               = "deny_not_super_admin"
	MemoryReasonDenyNotInApprovedOrgScope       = "deny_not_in_approved_org_scope"
	MemoryReasonDenyForbiddenSource             = "deny_forbidden_source"
	MemoryReasonDenyLowValueContent             = "deny_low_value_content"
	MemoryReasonDenyTruthConflict               = "deny_truth_conflict"
	MemoryReasonDenyDuplicateDocument           = "deny_duplicate_document"

	MemoryReasonOverrideExplicitUserStatement = "override_explicit_user_statement"
	MemoryReasonOverrideAdminSet              = "override_admin_set"
	MemoryReasonSkipLowerPrioritySource       = "skip_lower_priority_source"
	MemoryReasonSkipSameValue                 = "skip_same_value"

	MemoryReasonAllowStoreFact       = "allow_store_fact"
	MemoryReasonAllowStoreDocument   = "allow_store_document"
	MemoryReasonAllowTTLExpiring     = "allow_ttl_expiring"
	MemoryReasonAllowTTLPersistent   = "allow_ttl_persistent"
	MemoryReasonAllowSamePriority    = "allow_same_priority_source"
	MemoryReasonAllowDocumentRefresh = "allow_document_refresh"
)

// MemoryAccessContext 表示 policy 可消费的显式授权上下文。
type MemoryAccessContext struct {
	Principal            AIToolPrincipal
	ApprovedOrgScopeKeys []string
	ApprovedOrgIDs       []uint
	AllowPlatformOps     bool
}

// MemoryDecision 是所有 policy 决策的基础返回结构。
type MemoryDecision struct {
	Allowed    bool
	ReasonCode string
	Reason     string
}

// MemoryScopeDecision 表示 scope 解析结果。
type MemoryScopeDecision struct {
	MemoryDecision
	ScopeType MemoryScopeType
	ScopeKey  string
	UserID    *uint
	OrgID     *uint
}

// MemoryVisibilityDecision 表示 visibility 解析结果。
type MemoryVisibilityDecision struct {
	MemoryDecision
	Visibility MemoryVisibility
}

// MemoryTTLDecision 表示 TTL 解析结果。
type MemoryTTLDecision struct {
	MemoryDecision
	ExpiresAt *time.Time
}

// MemoryDocumentDecision 表示文档写入决策及去重元数据。
type MemoryDocumentDecision struct {
	MemoryDecision
	ContentHash string
	SummaryHash string
	DedupKey    string
}

// MemoryAccessTarget 表示可参与读写校验的记忆目标。
type MemoryAccessTarget struct {
	ScopeType  MemoryScopeType
	ScopeKey   string
	Visibility MemoryVisibility
	UserID     *uint
	OrgID      *uint
}

// MemoryScopeInput 描述待解析的作用域输入。
type MemoryScopeInput struct {
	ScopeType MemoryScopeType
	UserID    *uint
	OrgID     *uint
}

// MemoryFactCandidate 表示待写入的事实候选项。
type MemoryFactCandidate struct {
	ScopeType     MemoryScopeType
	UserID        *uint
	OrgID         *uint
	Namespace     string
	FactKey       string
	FactValueJSON string
	Summary       string
	SourceKind    MemorySourceKind
	SourceID      string
	LowValue      bool
	TruthConflict bool
}

// MemoryDocumentCandidate 表示待写入的文档候选项。
type MemoryDocumentCandidate struct {
	ScopeType     MemoryScopeType
	UserID        *uint
	OrgID         *uint
	MemoryType    MemoryType
	Topic         string
	Title         string
	Summary       string
	ContentText   string
	SourceKind    MemorySourceKind
	SourceID      string
	LowValue      bool
	TruthConflict bool
}

// MemoryFactVersion 描述事实覆盖比较所需的最小信息。
type MemoryFactVersion struct {
	ValueJSON   string
	SourceKind  MemorySourceKind
	Namespace   string
	ScopeType   MemoryScopeType
	Description string
}

// MemoryConversationSummaryQuery 收紧会话摘要读取契约。
type MemoryConversationSummaryQuery struct {
	ConversationID string
	UserID         uint
	OrgID          *uint
	ScopeKey       string
}

// NormalizeMemoryText 统一压平空白，保证哈希和去重键稳定。
func NormalizeMemoryText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

// BuildMemoryDocumentContentHash 基于规范化内容生成内容哈希。
func BuildMemoryDocumentContentHash(content string) string {
	return hashMemoryValue(NormalizeMemoryText(content))
}

// BuildMemoryDocumentSummaryHash 基于规范化摘要生成摘要哈希。
func BuildMemoryDocumentSummaryHash(summary string) string {
	return hashMemoryValue(NormalizeMemoryText(summary))
}

// BuildMemoryDocumentDedupKey 生成第一版规则去重键。
func BuildMemoryDocumentDedupKey(sourceKind MemorySourceKind, sourceID string, topic string, summary string, content string) string {
	normalizedSourceKind := NormalizeMemoryText(string(sourceKind))
	normalizedSourceID := NormalizeMemoryText(sourceID)
	normalizedTopic := NormalizeMemoryText(topic)
	if normalizedSourceKind != "" && normalizedSourceID != "" && normalizedTopic != "" {
		return "src:" + hashMemoryValue(
			strings.ToLower(strings.Join([]string{normalizedSourceKind, normalizedSourceID, normalizedTopic}, "\n")),
		)
	}

	summaryHash := BuildMemoryDocumentSummaryHash(summary)
	if summaryHash != "" {
		return "summary:" + summaryHash
	}
	contentHash := BuildMemoryDocumentContentHash(content)
	if contentHash != "" {
		return "content:" + contentHash
	}
	return ""
}

func hashMemoryValue(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// DescribeMemoryScope 返回便于日志和测试阅读的 scope 文本。
func DescribeMemoryScope(scopeType MemoryScopeType, scopeKey string) string {
	return fmt.Sprintf("scope_type=%s scope_key=%s", scopeType, scopeKey)
}
