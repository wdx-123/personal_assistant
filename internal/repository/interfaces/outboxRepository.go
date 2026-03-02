package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"

	"gorm.io/gorm"
)

// OutboxRepository Outbox 仓库接口
type OutboxRepository interface {
	// Create 创建 Outbox 事件
	Create(ctx context.Context, event *entity.OutboxEvent) error

	// CreateInTx 在事务中创建事件
	CreateInTx(tx *gorm.DB, event *entity.OutboxEvent) error

	// GetPendingEvents 获取待发布的事件
	GetPendingEvents(ctx context.Context, limit int, maxRetries int) ([]*entity.OutboxEvent, error)

	// MarkAsPublished 标记事件为已发布
	MarkAsPublished(ctx context.Context, eventID string) error

	// MarkAsFailed 标记事件为失败
	MarkAsFailed(ctx context.Context, eventID string, errorMsg string, maxRetries int) error
	// DeletePublishedBefore 删除指定时间之前的已发布事件
	DeletePublishedBefore(ctx context.Context, before time.Time) error
}
