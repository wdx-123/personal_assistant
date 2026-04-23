package system

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

// aiMessageProjector 负责把 runtime 事件折叠成 assistant 消息快照。
// 当前阶段维护文本输出和 tool trace，不恢复 interrupt / approval 状态机。
type aiMessageProjector struct {
	mu      sync.Mutex
	repo    interfaces.AIRepository
	message *entity.AIMessage
	// traceItems 保存当前 assistant 消息已折叠出的 trace 列表快照。
	traceItems []resp.AssistantTraceItem
	// traceIndex 维护 trace key 到切片下标的映射，便于 started/finished 事件合并。
	traceIndex map[string]int
}

// newAIMessageProjector 创建消息投影器。
// 参数：
//   - repo：AI 仓储，用于持久化消息快照。
//   - message：当前 assistant 消息实体。
//
// 返回值：
//   - *aiMessageProjector：绑定消息和仓储后的投影器。
func newAIMessageProjector(repo interfaces.AIRepository, message *entity.AIMessage) *aiMessageProjector {
	return &aiMessageProjector{
		// repo 负责把折叠后的消息快照持久化到数据库。
		repo: repo,
		// message 是当前要被实时更新的 assistant 消息实体。
		message: message,
		// traceItems 从空切片起步，后续按 tool 事件逐步追加或更新。
		traceItems: []resp.AssistantTraceItem{},
		// traceIndex 用于快速定位同一个 tool call 对应的 trace 项。
		traceIndex: make(map[string]int),
	}
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
//   - `scope_json` 当前阶段仍固定写空对象；`trace_items_json` 则根据 tool 事件实时折叠。
func (p *aiMessageProjector) persistMessage(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// message 为空时说明当前 projector 还没有可持久化的目标消息。
	if p.message == nil {
		return nil
	}

	// trace_items_json 折叠保存工具轨迹，供消息详情和 trace 展示复用。
	p.message.TraceItemsJSON = encodeAssistantTraceItems(p.traceItems)
	// 当前阶段 scope 仍未恢复，因此固定写空对象占位。
	p.message.ScopeJSON = "{}"
	// 每次持久化都刷新消息更新时间，保证列表页状态一致。
	p.message.UpdatedAt = time.Now()
	return p.repo.UpdateMessage(ctx, p.message)
}

// applyEvent 根据 runtime 事件更新 assistant 消息内存态。
// 参数：
//   - event：domain/ai 定义的最小事件。
//
// 核心流程：
//  1. `assistant_token` 追加到消息正文。
//  2. `tool_call_started` / `tool_call_finished` 维护 trace_items。
//  3. `message_completed` 覆盖最终正文并标记成功。
//  4. `error` 写入错误文案并标记失败。
//
// 注意事项：
//   - 本函数只更新内存态，真正落库由 persistMessage 统一完成。
func (p *aiMessageProjector) applyEvent(event aidomain.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// message 不存在时无需继续折叠事件。
	if p.message == nil {
		return
	}

	switch event.Name {
	case aidomain.EventAssistantToken:
		if payload, ok := event.Payload.(aidomain.AssistantTokenPayload); ok {
			// assistant_token 只负责追加正文内容。
			p.message.Content += payload.Token
			// 已进入 stopped / error 终态后，不再把状态改回 loading。
			if p.message.Status == aiMessageStatusStopped || p.message.Status == aiMessageStatusError {
				return
			}
			p.message.Status = aiMessageStatusLoading
		}
	case aidomain.EventMessageCompleted:
		if payload, ok := event.Payload.(aidomain.MessageCompletedPayload); ok {
			// message_completed 使用最终正文覆盖累积文本，避免 chunk 合并误差。
			p.message.Content = payload.Content
		}
		// 收到完成事件后统一切到 success。
		p.message.Status = aiMessageStatusSuccess
	case aidomain.EventToolCallStarted:
		if payload, ok := event.Payload.(aidomain.ToolCallStartedPayload); ok {
			// started 事件先生成 running 态 trace 项，占住稳定 key。
			item := resp.AssistantTraceItem{
				Key:         payload.Key,
				Title:       payload.Title,
				Description: payload.Description,
				Status:      "running",
			}
			p.upsertTraceItem(item)
		}
	case aidomain.EventToolCallFinished:
		if payload, ok := event.Payload.(aidomain.ToolCallFinishedPayload); ok {
			// finished 事件补充执行结果、耗时和详情，复用 started 时的同一个 key。
			item := resp.AssistantTraceItem{
				Key:            payload.Key,
				Description:    payload.Description,
				Status:         payload.Status,
				DurationMS:     payload.DurationMS,
				Content:        payload.Content,
				DetailMarkdown: payload.DetailMarkdown,
			}
			if payload.ToolName != "" {
				// Title 为空时用统一标题回填，避免前端看到匿名 trace 卡片。
				item.Title = "调用工具 " + payload.ToolName
			}
			p.upsertTraceItem(item)
		}
	case aidomain.EventError:
		if payload, ok := event.Payload.(aidomain.ErrorPayload); ok {
			// error 事件把用户可见错误文案同步到消息实体。
			p.message.ErrorText = payload.Message
		}
		// 一旦 runtime 报错，消息状态立即切到 error。
		p.message.Status = aiMessageStatusError
	}
}

// upsertTraceItem 负责按 key 合并同一次工具调用的 started/finished trace。
func (p *aiMessageProjector) upsertTraceItem(item resp.AssistantTraceItem) {
	if stringsIndex, ok := p.traceIndex[item.Key]; ok {
		// 已存在同 key trace 时，只覆盖本次事件提供的增量字段。
		current := p.traceItems[stringsIndex]
		if item.Title != "" {
			current.Title = item.Title
		}
		if item.Description != "" {
			current.Description = item.Description
		}
		if item.Status != "" {
			current.Status = item.Status
		}
		if item.DurationMS > 0 {
			current.DurationMS = item.DurationMS
		}
		if item.Content != "" {
			current.Content = item.Content
		}
		if item.DetailMarkdown != "" {
			current.DetailMarkdown = item.DetailMarkdown
		}
		p.traceItems[stringsIndex] = current
		return
	}

	// 首次出现的 key 直接写入索引并追加到 trace 列表末尾。
	p.traceIndex[item.Key] = len(p.traceItems)
	p.traceItems = append(p.traceItems, item)
}

// buildAITraceIndex 根据已有 trace_items 构建 key 到下标的索引。
func buildAITraceIndex(items []resp.AssistantTraceItem) map[string]int {
	// 预分配容量，避免后续重复扩容。
	index := make(map[string]int, len(items))
	for i, item := range items {
		// 空 key 不能参与 started/finished 合并，直接跳过。
		if item.Key == "" {
			continue
		}
		index[item.Key] = i
	}
	return index
}

// encodeAssistantTraceItems 负责把 trace_items 安全编码成 JSON 字符串。
func encodeAssistantTraceItems(items []resp.AssistantTraceItem) string {
	// 空结果固定返回 []，保持数据库字段格式稳定。
	if len(items) == 0 {
		return "[]"
	}

	// JSON 编码失败时回退为空数组，避免消息持久化被 trace 展示字段拖垮。
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
