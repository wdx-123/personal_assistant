package eventhandlers

import (
	"encoding/json"
	"manpao-service/internal/infrastructure/messaging"
	"manpao-service/pkg/events"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// TestEventHandler 测试事件处理器
// 用于验证事件驱动架构是否正常工作
type TestEventHandler struct {
	subscriber *messaging.RedisStreamsSubscriber // ← 用于手动 ACK
	logger     watermill.LoggerAdapter
}

// NewTestEventHandler 创建测试事件处理器实例
func NewTestEventHandler(
	subscriber *messaging.RedisStreamsSubscriber,
	logger watermill.LoggerAdapter,
) *TestEventHandler {
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	return &TestEventHandler{
		subscriber: subscriber,
		logger:     logger,
	}
}

// Handle 处理测试事件
// msg: Watermill 消息（包含 Payload 和 Metadata）
func (h *TestEventHandler) Handle(msg *message.Message) error {
	// Step 1: 反序列化
	var event events.TestEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		h.logger.Error("Failed to unmarshal event", err,
			watermill.LogFields{
				"message_uuid": msg.UUID,
				"payload":      string(msg.Payload),
			})
		// 反序列化失败说明消息格式错误，应 该 ACK 丢弃（避免无限重试）
		return h.subscriber.AckMessage(msg)
	}

	// Step 2: 打印事件信息（模拟业务处理）
	h.logger.Info("Test event received", watermill.LogFields{
		"event_id":     event.EventID,
		"event_type":   event.EventType,
		"aggregate_id": event.AggregateID,
		"message":      event.Message,
		"counter":      event.Counter,
		"occurred_at":  event.OccurredAt,
	})

	// Step 3: 确认消息（告诉 Redis 已处理）
	if err := h.subscriber.AckMessage(msg); err != nil {
		h.logger.Error("Failed to ACK message", err, watermill.LogFields{
			"message_uuid": msg.UUID,
		})
		return err // ← 返回错误，消息会重 新投递
	}

	h.logger.Debug("Message acknowledged", watermill.LogFields{
		"message_uuid": msg.UUID,
	})

	return nil
}
