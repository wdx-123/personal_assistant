package system

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

const (
	thinkingTraceKey         = "thinking_summary"
	thinkingTraceTitle       = "深度思考"
	thinkingTraceDescription = "正在整理当前判断和下一步。"
)

// aiMessageProjector 负责把最小 runtime 事件折叠成 assistant 消息快照。
// 它只处理基础流式文本状态，不再维护 A2UI、tool trace、interrupt 状态机。
type aiMessageProjector struct {
	mu      sync.Mutex
	repo    interfaces.AIRepository
	message *entity.AIMessage
}

// newAIMessageProjector 创建消息投影器。
// 参数：
//   - repo：AI 仓储，用于持久化消息快照。
//   - message：当前 assistant 消息实体。
//
// 返回值：
//   - *aiMessageProjector：绑定消息和仓储后的投影器。
func newAIMessageProjector(repo interfaces.AIRepository, message *entity.AIMessage) *aiMessageProjector {
	return &aiMessageProjector{repo: repo, message: message}
}

// setStopped 把 assistant 消息标记为停止态。
// 作用：用于请求取消、超时等非系统故障场景的收尾。
func (p *aiMessageProjector) setStopped() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.message != nil {
		p.message.Status = aiMessageStatusStopped
		p.updateThinkingTraceStatusLocked(aiMessageStatusStopped)
	}
}

// setError 把 assistant 消息标记为失败态并记录错误文案。
// 参数：
//   - message：可展示给用户或排障使用的错误说明。
func (p *aiMessageProjector) setError(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.message != nil {
		p.message.Status = aiMessageStatusError
		p.message.ErrorText = message
		p.updateThinkingTraceStatusLocked(aiMessageStatusError)
	}
}

// persistMessage 将当前内存中的 assistant 消息快照写回数据库。
// 参数：
//   - ctx：数据库操作上下文。
//
// 返回值：
//   - error：Repository 更新失败时返回原始错误。
//
// 注意事项：
//   - `trace_items_json`、`ui_blocks_json`、`scope_json` 本阶段保留兼容字段，但固定写空值。
func (p *aiMessageProjector) persistMessage(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.message == nil {
		return nil
	}
	if strings.TrimSpace(p.message.TraceItemsJSON) == "" {
		p.message.TraceItemsJSON = "[]"
	}
	if strings.TrimSpace(p.message.UIBlocksJSON) == "" {
		p.message.UIBlocksJSON = "[]"
	}
	if strings.TrimSpace(p.message.ScopeJSON) == "" {
		p.message.ScopeJSON = "{}"
	}
	p.message.UpdatedAt = time.Now()
	return p.repo.UpdateMessage(ctx, p.message)
}

// applyEvent 根据 runtime 事件更新 assistant 消息内存态。
// 参数：
//   - event：domain/ai 定义的最小事件。
//
// 核心流程：
//  1. `assistant_token` 追加到消息正文。
//  2. `message_completed` 覆盖最终正文并标记成功。
//  3. `error` 写入错误文案并标记失败。
//
// 注意事项：
//   - 本函数只更新内存态，真正落库由 persistMessage 统一完成。
func (p *aiMessageProjector) applyEvent(event aidomain.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.message == nil {
		return
	}

	switch event.Name {
	case aidomain.EventThinkingStarted:
		p.upsertThinkingTraceLocked(func(item *resp.AssistantTraceItem) {
			item.Title = thinkingTraceTitle
			item.Description = thinkingTraceDescription
			item.Status = aiMessageStatusLoading
		})
	case aidomain.EventThinkingDelta:
		if payload, ok := event.Payload.(aidomain.ThinkingDeltaPayload); ok {
			p.upsertThinkingTraceLocked(func(item *resp.AssistantTraceItem) {
				item.Title = thinkingTraceTitle
				item.Description = thinkingTraceDescription
				item.Status = aiMessageStatusLoading
				item.Content += payload.Delta
			})
		}
	case aidomain.EventThinkingCompleted:
		p.upsertThinkingTraceLocked(func(item *resp.AssistantTraceItem) {
			item.Title = thinkingTraceTitle
			item.Description = thinkingTraceDescription
			item.Status = aiMessageStatusSuccess
			if payload, ok := event.Payload.(aidomain.ThinkingCompletedPayload); ok && strings.TrimSpace(payload.Content) != "" {
				item.Content = payload.Content
			}
		})
	case aidomain.EventAssistantToken:
		if payload, ok := event.Payload.(aidomain.AssistantTokenPayload); ok {
			p.message.Content += payload.Token
			if p.message.Status == aiMessageStatusStopped || p.message.Status == aiMessageStatusError {
				return
			}
			p.message.Status = aiMessageStatusLoading
		}
	case aidomain.EventMessageCompleted:
		if payload, ok := event.Payload.(aidomain.MessageCompletedPayload); ok {
			p.message.Content = payload.Content
		}
		p.message.Status = aiMessageStatusSuccess
		p.updateThinkingTraceStatusLocked(aiMessageStatusSuccess)
	case aidomain.EventError:
		if payload, ok := event.Payload.(aidomain.ErrorPayload); ok {
			p.message.ErrorText = payload.Message
		}
		p.message.Status = aiMessageStatusError
		p.updateThinkingTraceStatusLocked(aiMessageStatusError)
	}
}

func (p *aiMessageProjector) decodeTraceItemsLocked() []resp.AssistantTraceItem {
	if p.message == nil || strings.TrimSpace(p.message.TraceItemsJSON) == "" {
		return []resp.AssistantTraceItem{}
	}
	items := make([]resp.AssistantTraceItem, 0)
	if err := json.Unmarshal([]byte(p.message.TraceItemsJSON), &items); err != nil {
		return []resp.AssistantTraceItem{}
	}
	return items
}

func (p *aiMessageProjector) encodeTraceItemsLocked(items []resp.AssistantTraceItem) {
	raw, err := json.Marshal(items)
	if err != nil {
		p.message.TraceItemsJSON = "[]"
		return
	}
	p.message.TraceItemsJSON = string(raw)
}

func (p *aiMessageProjector) upsertThinkingTraceLocked(mutator func(item *resp.AssistantTraceItem)) {
	if p.message == nil {
		return
	}
	items := p.decodeTraceItemsLocked()
	index := -1
	for idx, item := range items {
		if item.Key == thinkingTraceKey {
			index = idx
			break
		}
	}

	var target resp.AssistantTraceItem
	if index >= 0 {
		target = items[index]
	} else {
		target = resp.AssistantTraceItem{
			Key:         thinkingTraceKey,
			Title:       thinkingTraceTitle,
			Description: thinkingTraceDescription,
			Status:      aiMessageStatusLoading,
		}
	}

	mutator(&target)

	if index >= 0 {
		items[index] = target
	} else {
		items = append(items, target)
	}
	p.encodeTraceItemsLocked(items)
}

func (p *aiMessageProjector) updateThinkingTraceStatusLocked(status string) {
	if p.message == nil {
		return
	}
	items := p.decodeTraceItemsLocked()
	for idx, item := range items {
		if item.Key != thinkingTraceKey {
			continue
		}
		items[idx].Status = status
		p.encodeTraceItemsLocked(items)
		return
	}
}
