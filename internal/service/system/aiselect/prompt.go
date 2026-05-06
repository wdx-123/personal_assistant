package aiselect

import (
	"fmt"
	"strings"

	aidomain "personal_assistant/internal/domain/ai"
)

// PromptBuilder 负责生成本轮 runtime 所需的动态 system prompt。
type PromptBuilder interface {
	BuildDynamicPrompt(tools []aidomain.Tool, principal aidomain.AIToolPrincipal) string
	BuildDecisionPrompt(
		decision aidomain.ToolSelectionDecision,
		reason string,
		missingSlots []string,
	) string
}

type defaultPromptBuilder struct{}

func normalizePromptBuilder(builder PromptBuilder) PromptBuilder {
	if builder != nil {
		return builder
	}
	return defaultPromptBuilder{}
}

func (defaultPromptBuilder) BuildDynamicPrompt(
	tools []aidomain.Tool,
	principal aidomain.AIToolPrincipal,
) string {
	return buildToolDynamicPrompt(tools, principal)
}

func (defaultPromptBuilder) BuildDecisionPrompt(
	decision aidomain.ToolSelectionDecision,
	reason string,
	missingSlots []string,
) string {
	return buildDecisionPrompt(decision, reason, missingSlots)
}

func buildToolDynamicPrompt(tools []aidomain.Tool, principal aidomain.AIToolPrincipal) string {
	var builder strings.Builder
	builder.WriteString("你是 personal_assistant 的 AI 助手。\n")
	builder.WriteString("本轮只能使用当前注入的工具；不要假设还有其他工具。\n")
	builder.WriteString("如果用户请求需要 org_id、task_id、execution_id、request_id 等精确标识，而上下文里没有，不要猜测，直接向用户索取。\n")
	builder.WriteString("工具可见性已经按当前授权事实过滤，但真正执行时仍会再次鉴权；如果工具报权限错误，直接向用户说明。\n")
	builder.WriteString("如果工具 observation 的 classification=missing_user_input，不要继续调用工具，直接向用户追问缺失信息。\n")
	builder.WriteString("如果工具 observation 的 classification=repairable_invalid_param，先修正同一工具参数再重试，不要重复提交完全相同的参数。\n")
	builder.WriteString("时间字段必须使用 RFC3339，例如 2026-04-24T09:20:00Z。\n")
	if principal.CurrentOrgID != nil && *principal.CurrentOrgID > 0 {
		builder.WriteString(fmt.Sprintf("当前组织上下文 org_id=%d。\n", *principal.CurrentOrgID))
	}
	if len(tools) == 0 {
		builder.WriteString("本轮没有可用工具，请直接基于已有上下文回答，无法确认的数据不要编造。")
		return builder.String()
	}

	builder.WriteString("本轮可用工具：\n")
	for idx, tool := range tools {
		spec := tool.Spec()
		builder.WriteString(fmt.Sprintf("%d. %s: %s\n", idx+1, spec.Name, spec.Description))
	}
	return strings.TrimSpace(builder.String())
}

func buildDecisionPrompt(
	decision aidomain.ToolSelectionDecision,
	reason string,
	missingSlots []string,
) string {
	var builder strings.Builder
	builder.WriteString("你是 personal_assistant 的 AI 助手。\n")
	builder.WriteString("不要提及任何内部工具选择或路由过程。\n")
	switch decision {
	case aidomain.ToolSelectionDecisionAskUser:
		builder.WriteString("当前缺少继续处理所需的关键信息。请只向用户提出一个简洁、具体、自然的问题，不要直接回答未确认的信息。\n")
		if len(missingSlots) > 0 {
			builder.WriteString("缺少字段：")
			builder.WriteString(strings.Join(missingSlots, ", "))
			builder.WriteString("\n")
		}
	case aidomain.ToolSelectionDecisionDirectAnswer:
		builder.WriteString("当前无需调用工具。请直接基于已有上下文回答用户，不要编造未确认的数据。\n")
	default:
		builder.WriteString("请直接基于已有上下文回答用户。\n")
	}
	if strings.TrimSpace(reason) != "" {
		builder.WriteString("内部判定依据：")
		builder.WriteString(strings.TrimSpace(reason))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}
