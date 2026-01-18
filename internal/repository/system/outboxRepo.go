package system

import (
	"context"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"time"

	"gorm.io/gorm"
)

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) interfaces.OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Create(ctx context.Context, event *entity.OutboxEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *outboxRepository) CreateInTx(tx *gorm.DB, event *entity.OutboxEvent) error {
	return tx.Create(event).Error
}

func (r *outboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*entity.OutboxEvent, error) {
	var events []*entity.OutboxEvent
	// 查询 status=pending 且 retry_count < 3 的事件，按创建时间排序
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", entity.OutboxEventStatusPending, 3).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

func (r *outboxRepository) MarkAsPublished(ctx context.Context, eventID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&entity.OutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{
			"status":       entity.OutboxEventStatusPublished,
			"published_at": &now,
		}).Error
}

func (r *outboxRepository) MarkAsFailed(ctx context.Context, eventID string, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&entity.OutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{
			"status":        entity.OutboxEventStatusFailed,
			"error_message": errorMsg,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}
