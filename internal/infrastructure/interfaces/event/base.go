package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event 事件接口 - 所有领域事件都应实现此接口
type Event interface {
	// GetEventID 获取事件唯一标识
	GetEventID() string

	// GetEventType 获取事件类型（如 "order.payment.succeeded"）
	GetEventType() string

	// GetAggregateID 获取聚合根ID（如订单ID）
	GetAggregateID() string

	// GetOccurredAt 获取事件发生时间
	GetOccurredAt() time.Time

	// ToJSON 序列化为 JSON
	ToJSON() ([]byte, error)
}

// BaseEvent 事件基类 - 包含所有事件的通用字段
// 所有领域事件都应嵌入此结构体
type BaseEvent struct {
	EventID     string    `json:"event_id"`     // 事件唯一标识（UUID）
	EventType   string    `json:"event_type"`   // 事件类型（如 order.created）
	AggregateID string    `json:"aggregate_id"` // 服务ID（如订单ID）
	OccurredAt  time.Time `json:"occurred_at"`  // 事件发生时间
	Version     int       `json:"version"`      // 事件版本号（用于兼容性）
}

// NewBaseEvent 创建事件基类实例
// eventType: 事件类型，如 "order.payment.succeeded"
// aggregateID: 聚合根ID，如订单ID "ORD123456"
func NewBaseEvent(eventType, aggregateID string) BaseEvent {
	return BaseEvent{
		EventID:     uuid.New().String(),
		EventType:   eventType,
		AggregateID: aggregateID,
		OccurredAt:  time.Now(),
		Version:     1, // 默认版本号为 1
	}
}

// GetEventID 实现 Event 接口
func (e BaseEvent) GetEventID() string {
	return e.EventID
}

// GetEventType 实现 Event 接口
func (e BaseEvent) GetEventType() string {
	return e.EventType
}

// GetAggregateID 实现 Event 接口
func (e BaseEvent) GetAggregateID() string {
	return e.AggregateID
}

// GetOccurredAt 实现 Event 接口
func (e BaseEvent) GetOccurredAt() time.Time {
	return e.OccurredAt
}

// ToJSON 实现 Event 接口
func (e BaseEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}
