package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

type ProgressiveToolSelector struct {
	model        einomodel.BaseChatModel
	systemPrompt string
}

func NewProgressiveToolSelector(ctx context.Context, opt Options) (*ProgressiveToolSelector, error) {
	model, err := NewChatModel(ctx, opt)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(opt.SystemPrompt)
	if prompt == "" {
		prompt = "你是 personal_assistant 的内部工具选择器。你只输出 JSON，不输出自然语言解释。"
	}
	return &ProgressiveToolSelector{
		model:        model,
		systemPrompt: prompt,
	}, nil
}

func (s *ProgressiveToolSelector) SelectGroup(
	ctx context.Context,
	input aidomain.ToolGroupSelectionInput,
) (aidomain.ToolGroupSelection, error) {
	payload, err := json.Marshal(input.Groups)
	if err != nil {
		return aidomain.ToolGroupSelection{}, err
	}

	response, err := s.generateJSON(ctx, buildGroupSelectionPrompt(input.Query, string(payload)), input.History, input.Query)
	if err != nil {
		return aidomain.ToolGroupSelection{}, err
	}

	var result aidomain.ToolGroupSelection
	if err := unmarshalSelectorJSON(response, &result); err != nil {
		return aidomain.ToolGroupSelection{}, err
	}
	switch result.Decision {
	case aidomain.ToolSelectionDecisionDirectAnswer, aidomain.ToolSelectionDecisionAskUser, aidomain.ToolSelectionDecisionSelectGroup:
	default:
		return aidomain.ToolGroupSelection{}, fmt.Errorf("invalid selector decision: %s", result.Decision)
	}
	return result, nil
}

func (s *ProgressiveToolSelector) SelectTools(
	ctx context.Context,
	input aidomain.ToolSelectionInput,
) (aidomain.ToolSelection, error) {
	payload, err := json.Marshal(input.Tools)
	if err != nil {
		return aidomain.ToolSelection{}, err
	}

	response, err := s.generateJSON(
		ctx,
		buildToolSelectionPrompt(input.Query, input.Group, string(payload)),
		input.History,
		input.Query,
	)
	if err != nil {
		return aidomain.ToolSelection{}, err
	}

	var result aidomain.ToolSelection
	if err := unmarshalSelectorJSON(response, &result); err != nil {
		return aidomain.ToolSelection{}, err
	}
	switch result.Confidence {
	case aidomain.ToolSelectionConfidenceHigh, aidomain.ToolSelectionConfidenceLow:
	default:
		return aidomain.ToolSelection{}, fmt.Errorf("invalid selector confidence: %s", result.Confidence)
	}
	return result, nil
}

func (s *ProgressiveToolSelector) generateJSON(
	ctx context.Context,
	prompt string,
	history []aidomain.Message,
	query string,
) (string, error) {
	if s == nil || s.model == nil {
		return "", fmt.Errorf("progressive tool selector model is nil")
	}
	messages := make([]*schema.Message, 0, len(history)+3)
	if strings.TrimSpace(s.systemPrompt) != "" {
		messages = append(messages, schema.SystemMessage(strings.TrimSpace(s.systemPrompt)))
	}
	messages = append(messages, schema.SystemMessage(strings.TrimSpace(prompt)))
	for _, item := range history {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if strings.TrimSpace(item.Role) == aidomain.RoleAssistant {
			messages = append(messages, schema.AssistantMessage(content, nil))
			continue
		}
		messages = append(messages, schema.UserMessage(content))
	}
	if strings.TrimSpace(query) != "" {
		messages = append(messages, schema.UserMessage(strings.TrimSpace(query)))
	}

	msg, err := s.model.Generate(ctx, messages)
	if err != nil {
		return "", err
	}
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return "", fmt.Errorf("selector returned empty content")
	}
	return strings.TrimSpace(msg.Content), nil
}

func buildGroupSelectionPrompt(query string, groupsJSON string) string {
	return strings.TrimSpace(fmt.Sprintf(`
你正在执行第一阶段工具选择。任务是只根据用户问题和历史上下文，从给定工具组中做一个决策。

用户当前问题：%s

候选工具组（JSON 数组）：
%s

只输出一个 JSON 对象，不要输出 markdown、不要输出解释。
输出格式：
{
  "decision": "direct_answer | ask_user | select_group",
  "group_id": "仅在 decision=select_group 时填写",
  "reason": "可选，简短说明内部判断依据",
  "missing_slots": ["仅在 decision=ask_user 时可选填写缺失字段"]
}

决策规则：
1. 如果完全不需要工具即可回答，输出 direct_answer。
2. 如果明显缺少关键信息，且在进入工具前应先追问用户，输出 ask_user。
3. 如果需要工具，输出最合适的一个 group_id，不要返回多个组。
4. 除 JSON 外不要输出任何内容。`, query, groupsJSON))
}

func buildToolSelectionPrompt(query string, group aidomain.ToolGroupBrief, toolsJSON string) string {
	return strings.TrimSpace(fmt.Sprintf(`
你正在执行第二阶段工具选择。用户问题已经被路由到工具组 %s。

工具组摘要：
%s

用户当前问题：%s

候选工具（JSON 数组）：
%s

只输出一个 JSON 对象，不要输出 markdown、不要输出解释。
输出格式：
{
  "selected_tool_names": ["最多 3 个工具名"],
  "confidence": "high | low",
  "reason": "可选，简短说明内部判断依据"
}

规则：
1. 最多选择 3 个工具。
2. 如果能明确判断需要哪些工具，confidence=high。
3. 如果无法稳定判断或工具可能不够，confidence=low。
4. 只输出 JSON。`, group.ID, group.Summary, query, toolsJSON))
}

func unmarshalSelectorJSON(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
		trimmed = strings.TrimSpace(trimmed)
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	if trimmed == "" {
		return fmt.Errorf("selector output is empty")
	}
	if err := json.Unmarshal([]byte(trimmed), target); err != nil {
		return fmt.Errorf("invalid selector json: %w", err)
	}
	return nil
}
