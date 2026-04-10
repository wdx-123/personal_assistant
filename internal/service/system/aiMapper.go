package system

import (
	"encoding/json"
	"strings"
	"time"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"

	"github.com/google/uuid"
)

// newAIID 负责执行当前函数对应的核心逻辑。
// 参数：
//   - prefix：当前函数需要消费的输入参数。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func newAIID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

// deriveConversationTitle 生成会话标题
func deriveConversationTitle(existingTitle string, content string) string {
	title := strings.TrimSpace(existingTitle)
	if title != "" && title != "新建会话" {
		return truncateRunes(title, 100)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "新建会话"
	}
	return truncateRunes(content, 24)
}

// truncateRunes 负责执行当前函数对应的核心逻辑。
// 参数：
//   - input：当前阶段输入对象。
//   - limit：当前函数需要消费的输入参数。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func truncateRunes(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

// buildConversationPreview 负责执行当前函数对应的核心逻辑。
// 参数：
//   - content：当前函数需要消费的输入参数。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func buildConversationPreview(content string) string {
	return truncateRunes(content, 120)
}

// deriveConversationGroup 负责执行当前函数对应的核心逻辑。
// 参数：
//   - ts：当前函数需要消费的输入参数。
//
// 返回值：
//   - resp.AssistantConversationGroup：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func deriveConversationGroup(ts time.Time) resp.AssistantConversationGroup {
	now := time.Now()
	y1, m1, d1 := ts.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return resp.AssistantConversationGroupToday
	}
	if now.Sub(ts) <= 7*24*time.Hour {
		return resp.AssistantConversationGroupRecent
	}
	return resp.AssistantConversationGroupOlder
}

// conversationToResp 负责执行当前函数对应的核心逻辑。
// 作用：把数据库里的会话实体，转成返回给前端的响应对象。
func conversationToResp(conversation *entity.AIConversation) *resp.AssistantConversationResp {
	if conversation == nil {
		return nil
	}
	updatedAt := conversation.UpdatedAt
	if conversation.LastMessageAt != nil && conversation.LastMessageAt.After(updatedAt) {
		updatedAt = *conversation.LastMessageAt
	}
	return &resp.AssistantConversationResp{
		ID:           conversation.ID,
		Title:        conversation.Title,
		Preview:      conversation.Preview,
		UpdatedAt:    updatedAt.Format(time.RFC3339),
		Timestamp:    updatedAt.UnixMilli(),
		Group:        deriveConversationGroup(updatedAt),
		IsGenerating: conversation.IsGenerating,
	}
}

// messageToResp 负责执行当前函数对应的核心逻辑。
// 作用：把消息实体转成前端响应对象。
func messageToResp(message *entity.AIMessage) (*resp.AssistantMessageResp, error) {
	if message == nil {
		return nil, nil
	}
	return &resp.AssistantMessageResp{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Content:        message.Content,
		CreatedAt:      message.CreatedAt.Format(time.RFC3339),
		Status:         message.Status,
		// 从 JSON 字符串解码成结构化对象
		TraceItems:     decodeAssistantTraceItems(message.TraceItemsJSON), 
		UIBlocks:       decodeAssistantUIBlocks(message.UIBlocksJSON),
		Scope:          decodeAssistantScope(message.ScopeJSON),
		ErrorText:      message.ErrorText,
	}, nil
}

// encodeJSON 负责执行当前函数对应的核心逻辑。
// 作用：把结构体/数组编码成 JSON 字符串。
func encodeJSON(value any, emptyFallback string) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return emptyFallback
	}
	return string(raw)
}

// decodeAssistantTraceItems 负责执行当前函数对应的核心逻辑。
// 作用：把 traceItems 的 JSON 字符串解码成数组。
func decodeAssistantTraceItems(raw string) []resp.AssistantTraceItem {
	if strings.TrimSpace(raw) == "" {
		return []resp.AssistantTraceItem{}
	}
	items := make([]resp.AssistantTraceItem, 0)
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []resp.AssistantTraceItem{}
	}
	return items
}

// decodeAssistantUIBlocks 负责执行当前函数对应的核心逻辑。
// 作用：把 UI block 的 JSON 字符串解码成数组。
func decodeAssistantUIBlocks(raw string) []resp.AssistantA2UIBlock {
	if strings.TrimSpace(raw) == "" {
		return []resp.AssistantA2UIBlock{}
	}
	items := make([]resp.AssistantA2UIBlock, 0)
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []resp.AssistantA2UIBlock{}
	}
	return items
}

// decodeAssistantScope 负责执行当前函数对应的核心逻辑。
// 作用：把 scope 的 JSON 字符串解码成作用域信息。
func decodeAssistantScope(raw string) *resp.AssistantScopeInfo {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return nil
	}
	var item resp.AssistantScopeInfo
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return nil
	}
	if item.ScopeLabel == "" && item.UserName == "" && item.OrgName == "" {
		return nil
	}
	return &item
}

/*
	四、流式输出与上下文构造类
*/

// splitReplyChunks 负责执行当前函数对应的核心逻辑。
// 作用：把一整段回复拆成多个小块。
func splitReplyChunks(content string, size int) []string {
	if size <= 0 {
		size = 48
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]string, 0, (len(runes)/size)+1)
	for index := 0; index < len(runes); index += size {
		end := index + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[index:end]))
	}
	return chunks
}

// buildScopeInfo 负责执行当前函数对应的核心逻辑。
// 作用：根据请求参数，构造作用域信息。
func buildScopeInfo(req *request.StreamAssistantMessageReq) *resp.AssistantScopeInfo {
	if req == nil {
		return nil
	}
	return &resp.AssistantScopeInfo{
		UserName:      req.ContextUserName,
		OrgName:       req.ContextOrgName,
		ScopeLabel:    "当前用户 + 当前组织 + 最近任务 + 当前文档范围",
		TaskName:      "OJ 任务闭环联调 V2",
		DocScopeLabel: "README、架构设计方案、AI UI 改造说明",
	}
}

/*
	五、UI 组件构造类
*/

// textComponent 负责执行当前函数对应的核心逻辑。
// 作用：构造一个文本组件。
func textComponent(id string, value string, usageHint string, tone string) resp.AssistantA2UIComponent {
	return resp.AssistantA2UIComponent{ID: id, Type: "Text", Value: value, UsageHint: usageHint, Tone: tone}
}

// badgeComponent 负责执行当前函数对应的核心逻辑。
// 作用：构造一个徽标组件。
func badgeComponent(id string, label string, tone string) resp.AssistantA2UIComponent {
	return resp.AssistantA2UIComponent{ID: id, Type: "Badge", Label: label, Tone: tone}
}

// bulletListComponent 负责执行当前函数对应的核心逻辑。
// 作用：构造一个项目符号列表组件。
func bulletListComponent(id string, items []string) resp.AssistantA2UIComponent {
	return resp.AssistantA2UIComponent{ID: id, Type: "BulletList", Items: items}
}

// cardComponent 负责执行当前函数对应的核心逻辑。
// 作用：构造一个卡片组件。
func cardComponent(id string, tone string, children ...string) resp.AssistantA2UIComponent {
	return resp.AssistantA2UIComponent{ID: id, Type: "Card", Tone: tone, Children: children}
}

/*
	六、具体 UI Block 组装类
		这些函数就不是“基础组件”了，而是更高一层：
		直接拼出一块完整的业务 UI block。
*/

// buildThinkingSummaryBlock 负责执行当前函数对应的核心逻辑。
// 作用：生成“当前判断与下一步”的思考摘要卡片。
func buildThinkingSummaryBlock(plan *AIRuntimePlan) *resp.AssistantA2UIBlock {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	items := []string{
		"当前判断：这个问题需要结合业务上下文和已有结果来组织答案。",
		"当前动作：先汇总可直接使用的信息，再决定是否需要额外工具确认。",
		"下一步：在拿到确认结果后输出最终正文。",
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if plan != nil && plan.DocTool == nil {
		items[1] = "当前动作：当前问题不需要用户确认，可以直接整理结果。"
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return &resp.AssistantA2UIBlock{
		Key:  "block_thinking_summary",
		Type: "thinking_summary_block",
		Surface: resp.AssistantA2UISurface{
			ID:   "surface_thinking_summary",
			Root: "thinking_card_root",
			Components: []resp.AssistantA2UIComponent{
				cardComponent("thinking_card_root", "muted", "thinking_title", "thinking_points"),
				textComponent("thinking_title", "当前判断与下一步", "title", ""),
				bulletListComponent("thinking_points", items),
			},
		},
	}
}

// buildToolIntentBlock 负责执行当前函数对应的核心逻辑。
// 作用：生成“某个工具即将调用，需要用户确认”的意图卡片。
func buildToolIntentBlock(tool *AIToolBlueprint) *resp.AssistantA2UIBlock {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if tool == nil {
		return nil
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	return &resp.AssistantA2UIBlock{
		Key:  "block_tool_intent",
		Type: "tool_intent_block",
		Surface: resp.AssistantA2UISurface{
			ID:   "surface_tool_intent",
			Root: "tool_intent_card_root",
			Components: []resp.AssistantA2UIComponent{
				cardComponent("tool_intent_card_root", "warning", "tool_badge", "tool_title", "tool_points"),
				badgeComponent("tool_badge", "等待确认", "warning"),
				textComponent("tool_title", tool.Title+"需要你的确认", "title", ""),
				bulletListComponent("tool_points", []string{
					"目的：补充当前回答需要的正式依据或范围说明。",
					"必要性：已有上下文能给初步回答，但缺少更稳的支撑信息。",
					"确认要求：是否继续调用该工具由你决定。",
				}),
			},
		},
	}
}

// buildWaitingUserBlock 负责执行当前函数对应的核心逻辑。
// 作用：生成“当前正在等待用户决策”的卡片。
func buildWaitingUserBlock(tool *AIToolBlueprint) *resp.AssistantA2UIBlock {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if tool == nil {
		return nil
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	return &resp.AssistantA2UIBlock{
		Key:  "block_waiting_user",
		Type: "waiting_user_block",
		Surface: resp.AssistantA2UISurface{
			ID:   "surface_waiting_user",
			Root: "waiting_card_root",
			Components: []resp.AssistantA2UIComponent{
				cardComponent("waiting_card_root", "warning", "waiting_title", "waiting_description", "waiting_points"),
				textComponent("waiting_title", tool.ConfirmationTitle, "title", ""),
				textComponent("waiting_description", tool.ConfirmationDescription, "body", ""),
				bulletListComponent("waiting_points", []string{
					"继续后：会补充正式依据，再输出更完整的最终回答。",
					"跳过后：只基于当前已有上下文继续输出。",
				}),
			},
		},
	}
}

/*
	七、trace 与 UI block 的辅助更新类
*/

// assistantTraceActions 负责执行当前函数对应的核心逻辑。
// 作用：给 trace 生成两个标准操作按钮。
func assistantTraceActions(toolKey string) []resp.AssistantTraceAction {
	return []resp.AssistantTraceAction{
		{Key: toolKey + "_confirm", Label: "继续使用", Action: "confirm", Style: "primary"},
		{Key: toolKey + "_skip", Label: "跳过此工具", Action: "skip", Style: "default"},
	}
}

// upsertTraceItem 负责执行当前函数对应的核心逻辑。
// 作用：按 Key 更新或插入 trace item。
func upsertTraceItem(items []resp.AssistantTraceItem, item resp.AssistantTraceItem) []resp.AssistantTraceItem {
	for idx := range items {
		if items[idx].Key == item.Key {
			items[idx] = item
			return items
		}
	}
	return append(items, item)
}

// upsertUIBlock 负责执行当前函数对应的核心逻辑。
// 作用：按 Key 更新或插入 UI block。
func upsertUIBlock(items []resp.AssistantA2UIBlock, block resp.AssistantA2UIBlock) []resp.AssistantA2UIBlock {
	for idx := range items {
		if items[idx].Key == block.Key {
			items[idx] = block
			return items
		}
	}
	return append(items, block)
}
