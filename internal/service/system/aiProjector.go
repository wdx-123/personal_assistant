package system

import (
	"context"
	"sync"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
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
	}
}

// persistMessage 将当前内存中的 assistant 消息快照写回数据库。
// 注意事项：
//   - `trace_items_json`、`ui_blocks_json`、`scope_json` 本阶段保留兼容字段，但固定写空值。
func (p *aiMessageProjector) persistMessage(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.message == nil {
		return nil
	}
	p.message.TraceItemsJSON = "[]"
	p.message.UIBlocksJSON = "[]"
	p.message.ScopeJSON = "{}"
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
	case aidomain.EventError:
		if payload, ok := event.Payload.(aidomain.ErrorPayload); ok {
			p.message.ErrorText = payload.Message
		}
		p.message.Status = aiMessageStatusError
	}
}
