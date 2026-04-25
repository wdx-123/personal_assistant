package ai

// ToolGroupID 表示渐进式工具选择中的工具组标识。
type ToolGroupID string

// 渐进式工具选择支持的固定工具组。
const (
	ToolGroupOJPersonal           ToolGroupID = "oj_personal"
	ToolGroupOJOrg                ToolGroupID = "oj_org"
	ToolGroupOJTask               ToolGroupID = "oj_task"
	ToolGroupObservabilityTrace   ToolGroupID = "observability_trace"
	ToolGroupObservabilityMetrics ToolGroupID = "observability_metrics"
)

// ToolBrief 表示第一、二阶段选择器使用的轻量工具简介。
type ToolBrief struct {
	Name          string   `json:"name"`
	Summary       string   `json:"summary"`
	WhenToUse     string   `json:"when_to_use"`
	RequiredSlots []string `json:"required_slots,omitempty"`
	DomainTags    []string `json:"domain_tags,omitempty"`
}

// ToolGroupBrief 表示第一阶段选择器消费的工具组简介。
type ToolGroupBrief struct {
	ID         ToolGroupID `json:"id"`
	Summary    string      `json:"summary"`
	WhenToUse  string      `json:"when_to_use"`
	ToolNames  []string    `json:"tool_names,omitempty"`
	DomainTags []string    `json:"domain_tags,omitempty"`
}

// ToolSelectionDecision 表示第一阶段 selector 的决策结果。
type ToolSelectionDecision string

// 第一阶段 selector 的标准决策枚举。
const (
	ToolSelectionDecisionDirectAnswer ToolSelectionDecision = "direct_answer"
	ToolSelectionDecisionAskUser      ToolSelectionDecision = "ask_user"
	ToolSelectionDecisionSelectGroup  ToolSelectionDecision = "select_group"
)

// ToolSelectionConfidence 表示第二阶段 selector 选择结果的置信度。
type ToolSelectionConfidence string

// 第二阶段工具选择的置信度枚举。
const (
	ToolSelectionConfidenceHigh ToolSelectionConfidence = "high"
	ToolSelectionConfidenceLow  ToolSelectionConfidence = "low"
)

// ToolGroupSelectionInput 表示第一阶段选择器输入。
type ToolGroupSelectionInput struct {
	Query   string           `json:"query"`
	History []Message        `json:"history,omitempty"`
	Groups  []ToolGroupBrief `json:"groups"`
}

// ToolGroupSelection 表示第一阶段选择器输出。
type ToolGroupSelection struct {
	Decision     ToolSelectionDecision `json:"decision"`
	GroupID      ToolGroupID           `json:"group_id,omitempty"`
	Reason       string                `json:"reason,omitempty"`
	MissingSlots []string              `json:"missing_slots,omitempty"`
}

// ToolSelectionInput 表示第二阶段选择器输入。
type ToolSelectionInput struct {
	Query   string         `json:"query"`
	History []Message      `json:"history,omitempty"`
	Group   ToolGroupBrief `json:"group"`
	Tools   []ToolBrief    `json:"tools"`
}

// ToolSelection 表示第二阶段选择器输出。
type ToolSelection struct {
	SelectedToolNames []string                `json:"selected_tool_names,omitempty"`
	Confidence        ToolSelectionConfidence `json:"confidence"`
	Reason            string                  `json:"reason,omitempty"`
}
