package models

import (
	"time"

	"gorm.io/gorm"
)

// OutboxEvent 事件发件箱模型
// 用于实现 Transactional Outbox Pattern，保证事件零丢失
// nolint
type OutboxEvent struct {
	ID            uint           `gorm:"primaryKey;autoIncrement"                                             json:"id"`
	EventID       string         `gorm:"type:varchar(36);uniqueIndex;not null"                                json:"event_id"`                // UUID
	EventType     string         `gorm:"type:varchar(100);not null;index:idx_event_type"                      json:"event_type"`              // 事件类型，如 "order.payment.succeeded"
	AggregateID   string         `gorm:"type:varchar(100);not null"                                           json:"aggregate_id"`            // 聚合根 ID（如订单 ID）
	AggregateType string         `gorm:"type:varchar(50);not null"                                            json:"aggregate_type"`          // 聚合根类型（如 Order）
	Payload       string         `gorm:"type:json;not null"                                                   json:"payload"`                 // 事件数据（JSON 格式）
	Status        string         `gorm:"type:varchar(20);not null;default:'pending';index:idx_status_created" json:"status"`                  // pending, published, failed, dq_pending, dq_failed, dq_published
	RetryCount    int            `gorm:"type:int;default:0"                                                   json:"retry_count"`             // 重试次数
	ErrorMessage  string         `gorm:"type:text"                                                            json:"error_message,omitempty"` // 错误信息（发布失败时记录）
	CreatedAt     time.Time      `gorm:"index:idx_status_created"                                             json:"created_at"`              // 创建时间
	PublishedAt   *time.Time     `gorm:"index"                                                                json:"published_at,omitempty"`  // 发布时间（可为空）
	DeletedAt     gorm.DeletedAt `gorm:"index"                                                                json:"-"`                       // 软删除
}

// TableName 指定表名
func (OutboxEvent) TableName() string {
	return "outbox_events"
}

// OutboxEventStatus 事件状态常量
// 其中以dq_ 开头的状态将会被投入到延迟队列中
const (
	OutboxEventStatusPending   = "pending"   // 待发布
	OutboxEventStatusPublished = "published" // 已发布
	OutboxEventStatusFailed    = "failed"    // 发布失败

	OutboxEventStatusDQPending   = "dq_pending"   // 待发布
	OutboxEventStatusDQFailed    = "dq_failed"    // 发布失败，未达上限前，仍可重试
	OutboxEventStatusDQPublished = "dq_published" // 已发布
)
