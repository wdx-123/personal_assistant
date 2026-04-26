package ai

import (
	"fmt"
	"strings"
)

// MemoryScopeType 表示记忆数据的归属作用域。
type MemoryScopeType string

const (
	MemoryScopeSelf        MemoryScopeType = "self"
	MemoryScopeOrg         MemoryScopeType = "org"
	MemoryScopePlatformOps MemoryScopeType = "platform_ops"
)

// MemoryType 表示长期记忆文档的分类。
type MemoryType string

const (
	MemoryTypeEntity         MemoryType = "entity"
	MemoryTypeSessionSummary MemoryType = "session_summary"
	MemoryTypeEpisodic       MemoryType = "episodic"
	MemoryTypeSemantic       MemoryType = "semantic"
	MemoryTypeProcedural     MemoryType = "procedural"
	MemoryTypeIncident       MemoryType = "incident"
	MemoryTypeFAQ            MemoryType = "faq"
)

// MemoryVisibility 表示记忆的访问等级。
type MemoryVisibility string

const (
	MemoryVisibilitySelf       MemoryVisibility = "self"
	MemoryVisibilityOrg        MemoryVisibility = "org"
	MemoryVisibilitySuperAdmin MemoryVisibility = "super_admin"
)

const (
	MemoryNamespaceUserPreference = "user_preference"
	MemoryNamespaceOJProfile      = "oj_profile"
	MemoryNamespaceOJGoal         = "oj_goal"
	MemoryNamespaceOrgProfile     = "org_profile"
	MemoryNamespaceOrgLearning    = "org_learning_pattern"
	MemoryNamespaceOpsIncident    = "ops_incident"
	MemoryNamespaceOpsRunbook     = "ops_runbook"
)

// BuildSelfMemoryScopeKey 统一生成个人作用域的 scope_key。
func BuildSelfMemoryScopeKey(userID uint) string {
	return fmt.Sprintf("self:user:%d", userID)
}

// BuildOrgMemoryScopeKey 统一生成组织作用域的 scope_key。
func BuildOrgMemoryScopeKey(orgID uint) string {
	return fmt.Sprintf("org:%d", orgID)
}

// BuildPlatformOpsMemoryScopeKey 返回平台运维级记忆作用域。
func BuildPlatformOpsMemoryScopeKey() string {
	return string(MemoryScopePlatformOps)
}

// BuildMemoryScopeKey 按固定规则生成 scope_key，禁止在业务代码中手写格式。
func BuildMemoryScopeKey(scopeType MemoryScopeType, userID uint, orgID *uint) (string, error) {
	switch scopeType {
	case MemoryScopeSelf:
		if userID == 0 {
			return "", fmt.Errorf("user_id is required for self scope")
		}
		return BuildSelfMemoryScopeKey(userID), nil
	case MemoryScopeOrg:
		if orgID == nil || *orgID == 0 {
			return "", fmt.Errorf("org_id is required for org scope")
		}
		return BuildOrgMemoryScopeKey(*orgID), nil
	case MemoryScopePlatformOps:
		return BuildPlatformOpsMemoryScopeKey(), nil
	default:
		return "", fmt.Errorf("unsupported memory scope type: %s", scopeType)
	}
}

// BuildConversationMemoryScopeKey 根据会话所属用户和当前组织快照生成 summary scope_key。
func BuildConversationMemoryScopeKey(userID uint, orgID *uint) string {
	if orgID != nil && *orgID > 0 {
		return BuildOrgMemoryScopeKey(*orgID)
	}
	return BuildSelfMemoryScopeKey(userID)
}

// MemoryFactQuery 描述 facts 的读取条件。
type MemoryFactQuery struct {
	// ScopeKeys 是允许读取的记忆归属集合。
	// 调用方必须先完成主体解析与 scope 计算，再把最终允许访问的 scope_key 传入仓储层。
	ScopeKeys []string
	// AllowedVisibilities 是本次调用允许暴露给当前主体的访问等级集合。
	// scope 负责“属于谁”，visibility 负责“谁能看”，查询时必须同时满足两者。
	AllowedVisibilities []MemoryVisibility
	// Namespace 用于按业务域过滤结构化事实，例如 user_preference、oj_goal。
	Namespace string
	// FactKeys 用于在同一 namespace 下进一步收窄到具体事实键。
	FactKeys []string
	// Limit 控制本次读取最多返回多少条记录；小于等于 0 表示不主动限制。
	Limit int
}

// MemoryDocumentQuery 描述 documents 的读取条件。
type MemoryDocumentQuery struct {
	// ScopeKeys 是允许访问的文档归属集合。
	ScopeKeys []string
	// AllowedVisibilities 是当前主体允许读取的访问等级集合。
	AllowedVisibilities []MemoryVisibility
	// MemoryTypes 用于限定文档分类，例如 semantic、episodic、incident。
	MemoryTypes []MemoryType
	// Topic 用于对同类文档做轻量主题过滤；后续可扩展为更细的业务标签。
	Topic string
	// Limit 控制最大返回条数；小于等于 0 表示不主动限制。
	Limit int
}

// NormalizeMemoryVisibilities 把 visibility 列表转换为稳定字符串切片。
func NormalizeMemoryVisibilities(visibilities []MemoryVisibility) []string {
	if len(visibilities) == 0 {
		return nil
	}
	items := make([]string, 0, len(visibilities))
	for _, item := range visibilities {
		value := strings.TrimSpace(string(item))
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	return items
}

// NormalizeMemoryTypes 把 memory type 列表转换为稳定字符串切片。
func NormalizeMemoryTypes(types []MemoryType) []string {
	if len(types) == 0 {
		return nil
	}
	items := make([]string, 0, len(types))
	for _, item := range types {
		value := strings.TrimSpace(string(item))
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	return items
}
