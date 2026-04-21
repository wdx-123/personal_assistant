package system

import (
	"encoding/json"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
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
		TraceItems: decodeAssistantTraceItems(message.TraceItemsJSON),
		UIBlocks:   decodeAssistantUIBlocks(message.UIBlocksJSON),
		Scope:      decodeAssistantScope(message.ScopeJSON),
		ErrorText:  message.ErrorText,
	}, nil
}

func messagesToRuntimeHistory(messages []*entity.AIMessage) []aidomain.Message {
	items := make([]aidomain.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil || strings.TrimSpace(message.Content) == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role != aidomain.RoleAssistant {
			role = aidomain.RoleUser
		}
		items = append(items, aidomain.Message{
			ID:      message.ID,
			Role:    role,
			Content: message.Content,
		})
	}
	return items
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
