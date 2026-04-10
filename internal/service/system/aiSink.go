package system

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	streamsse "personal_assistant/internal/infrastructure/sse"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

// aiStreamSink 负责把运行时事件同时写到 SSE 输出与数据库消息状态中。
// 它是 AIService 和 AIRuntime 之间的“状态汇聚层”，保证前端看到的事件序列与库内消息快照尽量一致。
type aiStreamSink struct {
	repo       interfaces.AIRepository
	writer     streamsse.StreamWriter
	message    *entity.AIMessage
	interrupt  *entity.AIInterrupt
	traceItems []resp.AssistantTraceItem
	uiBlocks   []resp.AssistantA2UIBlock
	scope      *resp.AssistantScopeInfo
}

// newAIStreamSink 负责创建一条流式执行期间使用的 sink。
// 创建一个 aiStreamSink 实例。
func newAIStreamSink(
	repo interfaces.AIRepository,
	writer streamsse.StreamWriter,
	message *entity.AIMessage,
	interrupt *entity.AIInterrupt,
) *aiStreamSink {
	return &aiStreamSink{
		repo:       repo,
		writer:     writer,
		message:    message,
		interrupt:  interrupt,
		traceItems: []resp.AssistantTraceItem{},
		uiBlocks:   []resp.AssistantA2UIBlock{},
	}
}

// Emit 负责发出一个运行时事件，并把事件影响同步折叠到消息快照。
// 参数：
//   - ctx：链路上下文。
//   - eventName：事件名。
//   - payload：事件载荷。
//
// 作用：发出一个运行时事件，并把这个事件的影响同步到内存快照和数据库。
func (s *aiStreamSink) Emit(ctx context.Context, eventName string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if err := s.writer.WriteEvent(ctx, &streamsse.StreamEvent{
		StreamKind: streamsse.StreamKindSession,
		EventName:  eventName,
		Data:       raw,
		OccurredAt: time.Now(),
	}); err != nil {
		return err
	}

	s.applyEvent(eventName, payload)
	return s.persistMessage(ctx)
}

// Heartbeat 负责向客户端发送 keepalive 心跳。
// 作用：给客户端发心跳，只用于保活。
// 核心流程：
//  1. 直接复用底层 writer 的心跳能力。
func (s *aiStreamSink) Heartbeat(ctx context.Context) error {
	return s.writer.WriteHeartbeat(ctx)
}

// setStopped 负责把当前消息标记为“已中断”。
// 作用：把当前消息状态改成“已停止 / 已中断”。
func (s *aiStreamSink) setStopped() {
	s.message.Status = aiMessageStatusStopped
}

// setError 负责把当前消息标记为“失败”并记录错误文案。
// 作用：把当前消息标记为失败，并记录错误文案。
func (s *aiStreamSink) setError(message string) {
	s.message.Status = aiMessageStatusError
	s.message.ErrorText = message
}

// persistMessage 负责把当前内存态消息快照写回数据库。
// 作用：把当前 sink 内存里的最新状态写回数据库。
func (s *aiStreamSink) persistMessage(ctx context.Context) error {
	s.message.TraceItemsJSON = encodeJSON(s.traceItems, "[]")
	s.message.UIBlocksJSON = encodeJSON(s.uiBlocks, "[]")
	if s.scope == nil {
		s.message.ScopeJSON = "{}"
	} else {
		s.message.ScopeJSON = encodeJSON(s.scope, "{}")
	}
	s.message.UpdatedAt = time.Now()

	if err := s.repo.UpdateMessage(ctx, s.message); err != nil {
		return err
	}
	if s.interrupt != nil {
		if err := s.repo.UpdateInterrupt(ctx, s.interrupt); err != nil {
			return err
		}
	}
	return nil
}

// applyEvent 负责把单个运行时事件折叠到消息快照中。
// 作用：根据事件类型，更新内存中的消息、轨迹、UI、scope、interrupt 状态。
func (s *aiStreamSink) applyEvent(eventName string, payload any) {
	switch eventName {
	case "assistant_token":
		// token 事件只追加正文内容，并在必要时把 idle 重新切回 loading。
		if item, ok := payload.(resp.AssistantTokenPayload); ok {
			s.message.Content += item.Token
			if s.message.Status == aiMessageStatusIdle {
				s.message.Status = aiMessageStatusLoading
			}
		}

	case "tool_call_started":
		// 工具开始时先写一条 pending trace，方便前端立刻显示“正在执行”。
		if item, ok := payload.(resp.AssistantToolCallStartedPayload); ok {
			s.traceItems = upsertTraceItem(s.traceItems, resp.AssistantTraceItem{
				Key:         item.Key,
				Title:       item.Title,
				Description: item.Description,
				Status:      "pending",
			})
		}

	case "tool_call_finished":
		// 工具结束时用最新执行结果覆盖 trace，并在命中 interrupt 工具时推进 interrupt 状态。
		if item, ok := payload.(resp.AssistantToolCallFinishedPayload); ok {
			s.traceItems = upsertTraceItem(s.traceItems, resp.AssistantTraceItem{
				Key:            item.Key,
				Title:          existingTraceTitle(s.traceItems, item.Key, item.Key),
				Description:    item.Description,
				Status:         item.Status,
				DurationMS:     item.DurationMS,
				Content:        item.Content,
				DetailMarkdown: item.DetailMarkdown,
			})
			if s.interrupt != nil && s.interrupt.ToolKey == item.Key {
				s.interrupt.Status = aiInterruptStatusDone
				s.interrupt.UpdatedAt = time.Now()
			}
		}

	case "tool_call_waiting_confirmation":
		// 等待确认时把消息切到 idle，表示生成暂时停住，等待用户显式决策。
		if item, ok := payload.(resp.AssistantToolCallWaitingConfirmationPayload); ok {
			s.message.Status = aiMessageStatusIdle
			s.traceItems = upsertTraceItem(s.traceItems, resp.AssistantTraceItem{
				Key:                     item.Key,
				Title:                   item.Title,
				Description:             item.Description,
				Status:                  "awaiting_confirmation",
				InterruptID:             item.InterruptID,
				DetailMarkdown:          item.DetailMarkdown,
				RequiresConfirmation:    true,
				ConfirmationTitle:       item.ConfirmationTitle,
				ConfirmationDescription: item.ConfirmationDescription,
				Actions:                 item.Actions,
			})
		}

	case "tool_call_confirmation_result":
		if item, ok := payload.(resp.AssistantToolCallConfirmationResultPayload); ok {
			// confirm 后会重新进入加载态；skip 则保持后续由最终消息完成态覆盖。
			if item.Status == "pending" {
				s.message.Status = aiMessageStatusLoading
			}

			// waiting_user_block 只在等待阶段展示，一旦用户已决策就应移除，避免 UI 残留误导。
			filtered := make([]resp.AssistantA2UIBlock, 0, len(s.uiBlocks))
			for _, block := range s.uiBlocks {
				if block.Type != "waiting_user_block" {
					filtered = append(filtered, block)
				}
			}
			s.uiBlocks = filtered

			// trace 里保留最终确认结果，供消息详情回放这次 interrupt 的决策路径。
			s.traceItems = upsertTraceItem(s.traceItems, resp.AssistantTraceItem{
				Key:            item.Key,
				Title:          existingTraceTitle(s.traceItems, item.Key, item.Key),
				Description:    item.Description,
				Status:         item.Status,
				InterruptID:    item.InterruptID,
				DetailMarkdown: item.DetailMarkdown,
			})
			if s.interrupt != nil && s.interrupt.InterruptID == item.InterruptID && item.Status == "skipped" {
				s.interrupt.Status = aiInterruptStatusSkipped
				s.interrupt.UpdatedAt = time.Now()
			}
		}

	case "structured_block":
		// 结构化块会影响 UI 呈现和 scope 信息，需要并入消息快照供后续列表/详情重放。
		if item, ok := payload.(resp.AssistantStructuredBlockPayload); ok {
			if item.UIBlock != nil {
				s.uiBlocks = upsertUIBlock(s.uiBlocks, *item.UIBlock)
			}
			if item.Scope != nil {
				s.scope = item.Scope
			}
		}

	case "message_completed":
		// completed 事件以最终正文为准，避免 token 逐步追加过程中出现尾部不一致。
		if item, ok := payload.(resp.AssistantMessageCompletedPayload); ok {
			s.message.Content = item.Content
			s.message.Status = aiMessageStatusSuccess
		}

	case "error":
		// error 事件直接覆盖错误状态与文案，保证数据库里能看到最终失败原因。
		if item, ok := payload.(resp.AssistantErrorPayload); ok {
			s.message.Status = aiMessageStatusError
			s.message.ErrorText = item.Message
		}
	}
}

// existingTraceTitle 负责为 trace 更新场景找到一个稳定标题。
// 作用：为 trace 更新场景找到一个稳定标题。
func existingTraceTitle(items []resp.AssistantTraceItem, key string, fallback string) string {
	for _, item := range items {
		if item.Key == key && strings.TrimSpace(item.Title) != "" {
			return item.Title
		}
	}
	return fallback
}
