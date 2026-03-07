package system

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

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

func (r *outboxRepository) GetPendingEvents(ctx context.Context, limit int, maxRetries int) ([]*entity.OutboxEvent, error) {
	var events []*entity.OutboxEvent
	err := r.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", entity.OutboxEventStatusPending, maxRetries).
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

func (r *outboxRepository) MarkAsFailed(
	ctx context.Context,
	eventID string,
	errorMsg string,
	maxRetries int,
) error {
	return r.db.WithContext(ctx).
		Model(&entity.OutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]interface{}{
			"status": gorm.Expr(
				"CASE WHEN retry_count + 1 >= ? THEN ? ELSE ? END",
				maxRetries,
				entity.OutboxEventStatusFailed,
				entity.OutboxEventStatusPending,
			),
			"error_message": errorMsg,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}

func (r *outboxRepository) DeletePublishedBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("status = ? AND published_at IS NOT NULL AND published_at < ?", entity.OutboxEventStatusPublished, before).
		Delete(&entity.OutboxEvent{}).Error
}

func (r *outboxRepository) DeleteFailedBefore(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", entity.OutboxEventStatusFailed, before).
		Delete(&entity.OutboxEvent{}).Error
}

func (r *outboxRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	type statusRow struct {
		Status string
		Count  int64
	}

	var rows []*statusRow
	if err := r.db.WithContext(ctx).
		Model(&entity.OutboxEvent{}).
		Select("status, COUNT(1) AS count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		result[row.Status] = row.Count
	}
	return result, nil
}
