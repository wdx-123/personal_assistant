package entity

import (
	"time"
)

// OutboxEvent 事件发件箱模型
// 用于实现 Transactional Outbox Pattern，保证事件零丢失
type OutboxEvent struct {
	MODEL
	EventID       string     `gorm:"type:varchar(36);uniqueIndex;not null;comment:'事件UUID'" json:"event_id"`
	EventType     string     `gorm:"type:varchar(100);not null;index:idx_event_type;comment:'事件类型'" json:"event_type"`
	AggregateID   string     `gorm:"type:varchar(100);not null;comment:'聚合根ID'" json:"aggregate_id"`
	AggregateType string     `gorm:"type:varchar(50);not null;comment:'聚合根类型'" json:"aggregate_type"`
	Payload       string     `gorm:"type:json;not null;comment:'事件数据'" json:"payload"`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_status_created;comment:'状态'" json:"status"`
	RetryCount    int        `gorm:"type:int;default:0;comment:'重试次数'" json:"retry_count"`
	ErrorMessage  string     `gorm:"type:text;comment:'错误信息'" json:"error_message,omitempty"`
	PublishedAt   *time.Time `gorm:"index;comment:'发布时间'" json:"published_at,omitempty"`
}

// OutboxEventStatus 事件状态常量
const (
	OutboxEventStatusPending   = "pending"   // 待发布
	OutboxEventStatusPublished = "published" // 已发布
	OutboxEventStatusFailed    = "failed"    // 发布失败
)
