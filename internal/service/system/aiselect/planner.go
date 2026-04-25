package aiselect

import (
	"context"
	"strings"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/service/system/aitool"
)

// Selector 定义渐进式两阶段工具选择器能力。
type Selector interface {
	SelectGroup(ctx context.Context, input aidomain.ToolGroupSelectionInput) (aidomain.ToolGroupSelection, error)
	SelectTools(ctx context.Context, input aidomain.ToolSelectionInput) (aidomain.ToolSelection, error)
}

// ExecutionPlan 表示本轮最终要暴露给 runtime 的工具计划。
type ExecutionPlan struct {
	Tools               []aidomain.Tool
	DynamicSystemPrompt string
}

// Planner 负责把可见工具、selector 和 prompt builder 组装成本轮执行计划。
type Planner struct {
	registry      *aitool.Registry
	selector      Selector
	promptBuilder PromptBuilder
}

// NewPlanner 创建渐进式工具计划器。
func NewPlanner(registry *aitool.Registry, selector Selector, promptBuilder PromptBuilder) *Planner {
	return &Planner{
		registry:      registry,
		selector:      selector,
		promptBuilder: normalizePromptBuilder(promptBuilder),
	}
}

// BuildExecutionPlan 按渐进式选择逻辑生成本轮工具执行计划。
func (p *Planner) BuildExecutionPlan(
	ctx context.Context,
	query string,
	history []aidomain.Message,
	visibleTools []aidomain.Tool,
	principal aidomain.AIToolPrincipal,
) (ExecutionPlan, error) {
	promptBuilder := normalizePromptBuilder(p.promptBuilder)
	defaultPlan := ExecutionPlan{
		Tools:               visibleTools,
		DynamicSystemPrompt: promptBuilder.BuildDynamicPrompt(visibleTools, principal),
	}
	if len(visibleTools) == 0 || p == nil || p.selector == nil || p.registry == nil {
		return defaultPlan, nil
	}

	groupBriefs := p.registry.ListVisibleToolGroupBriefs(visibleTools)
	if len(groupBriefs) == 0 {
		return defaultPlan, nil
	}

	groupSelection, err := p.selector.SelectGroup(ctx, aidomain.ToolGroupSelectionInput{
		Query:   strings.TrimSpace(query),
		History: history,
		Groups:  groupBriefs,
	})
	if err != nil {
		return defaultPlan, nil
	}

	switch groupSelection.Decision {
	case aidomain.ToolSelectionDecisionDirectAnswer, aidomain.ToolSelectionDecisionAskUser:
		return ExecutionPlan{
			Tools: nil,
			DynamicSystemPrompt: promptBuilder.BuildDecisionPrompt(
				groupSelection.Decision,
				groupSelection.Reason,
				groupSelection.MissingSlots,
			),
		}, nil
	case aidomain.ToolSelectionDecisionSelectGroup:
		// continue
	default:
		return defaultPlan, nil
	}

	groupBrief, ok := findToolGroupBrief(groupBriefs, groupSelection.GroupID)
	if !ok {
		return defaultPlan, nil
	}
	toolBriefs := p.registry.ListVisibleToolBriefsByGroup(visibleTools, groupSelection.GroupID)
	if len(toolBriefs) == 0 {
		return defaultPlan, nil
	}

	toolSelection, err := p.selector.SelectTools(ctx, aidomain.ToolSelectionInput{
		Query:   strings.TrimSpace(query),
		History: history,
		Group:   groupBrief,
		Tools:   toolBriefs,
	})
	if err != nil {
		return defaultPlan, nil
	}

	selectedTools := p.registry.ExpandVisibleToolsByNames(visibleTools, toolSelection.SelectedToolNames)
	if toolSelection.Confidence == aidomain.ToolSelectionConfidenceLow || len(selectedTools) == 0 {
		selectedTools = p.registry.ExpandVisibleToolsByGroup(visibleTools, groupSelection.GroupID)
	}
	if len(selectedTools) == 0 {
		return defaultPlan, nil
	}

	return ExecutionPlan{
		Tools:               selectedTools,
		DynamicSystemPrompt: promptBuilder.BuildDynamicPrompt(selectedTools, principal),
	}, nil
}

func findToolGroupBrief(items []aidomain.ToolGroupBrief, groupID aidomain.ToolGroupID) (aidomain.ToolGroupBrief, bool) {
	for _, item := range items {
		if item.ID == groupID {
			return item, true
		}
	}
	return aidomain.ToolGroupBrief{}, false
}
