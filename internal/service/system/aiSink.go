package system

import (
	"context"
	"encoding/json"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	streamsse "personal_assistant/internal/infrastructure/sse"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

// aiStreamSink 负责把运行时事件同时写到 SSE 输出与数据库消息状态中。
// 它是 AIService 和 AIRuntime 之间的“状态汇聚层”，保证前端看到的事件序列与库内消息快照尽量一致。
type aiStreamSink struct {
	writer    streamsse.StreamWriter
	projector *aiMessageProjector
}

// newAIStreamSink 负责创建一条流式执行期间使用的 sink。
// 创建一个 aiStreamSink 实例。
func newAIStreamSink(
	repo interfaces.AIRepository,
	writer streamsse.StreamWriter,
	message *entity.AIMessage,
) *aiStreamSink {
	return &aiStreamSink{
		writer:    writer,
		projector: newAIMessageProjector(repo, message),
	}
}

// Emit 负责发出一个运行时事件，并把事件影响同步折叠到消息快照。
// 参数：
//   - ctx：链路上下文。
//   - eventName：事件名。
//   - payload：事件载荷。
//
// 作用：发出一个运行时事件，并把这个事件的影响同步到内存快照和数据库。
func (s *aiStreamSink) Emit(ctx context.Context, event aidomain.Event) error {
	raw, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}

	if err := s.writer.WriteEvent(ctx, &streamsse.StreamEvent{
		StreamKind: streamsse.StreamKindSession,
		EventName:  string(event.Name),
		Data:       raw,
		OccurredAt: time.Now(),
	}); err != nil {
		return err
	}
	s.projector.applyEvent(event)
	return s.projector.persistMessage(ctx)
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
	s.projector.setStopped()
}

// setError 负责把当前消息标记为“失败”并记录错误文案。
// 作用：把当前消息标记为失败，并记录错误文案。
func (s *aiStreamSink) setError(message string) {
	s.projector.setError(message)
}

// persistMessage 负责把当前内存态消息快照写回数据库。
// 作用：把当前 sink 内存里的最新状态写回数据库。
func (s *aiStreamSink) persistMessage(ctx context.Context) error {
	return s.projector.persistMessage(ctx)
}
