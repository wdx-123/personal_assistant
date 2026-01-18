package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"manpao-service/internal/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository Outbox 仓库接口
// 定义事件发件箱的所有数据访问操作
type Repository interface {
	// SaveEvent 在事务中保存事件到 outbox 表
	// aggregateID: 服务 ID（如订单 ID）
	// aggregateType: 服务类型（如 "Order"）
	// payload: 事件数据（会被序列化为 JSON）
	SaveEvent(tx *gorm.DB, eventType, aggregateID, aggregateType string,
		payload any) error

	SaveEventWithStatus(
		tx *gorm.DB,
		eventType, aggregateID, aggregateType string,
		payload any,
		status string,
	) error

	// Create 创建 Outbox 事件（接受已构建的 OutboxEvent）
	Create(ctx context.Context, event *models.OutboxEvent) error

	// GetPendingEvents 获取待发布的事件
	// limit: 最多返回多少条
	GetPendingEvents(ctx context.Context, limit int) ([]*models.OutboxEvent, error)

	GetRetryableEvents(
		ctx context.Context,
		statuses []string,
		limit int,
		maxRetry int,
	) ([]*models.OutboxEvent, error)

	// MarkAsPublished 标记事件为已发布
	// eventID: 事件的唯一 ID
	MarkAsPublished(ctx context.Context, eventID string) error

	MarkAsPublishedWithStatus(ctx context.Context, eventID string, status string) error

	// MarkAsFailed 标记事件为发布失败
	// eventID: 事件的唯一 ID
	MarkAsFailed(ctx context.Context, eventID string, errorMsg string) error

	MarkAsFailedWithStatus(
		ctx context.Context,
		eventID string,
		errorMsg string,
		status string,
	) error

	// PublishInTransaction 在事务中发布事件
	PublishInTransaction(fn func(tx *gorm.DB) error) error
}

// repositoryImpl Repository 接口实现
type repositoryImpl struct {
	db *gorm.DB
}

// NewRepository 创建 Outbox Repository 实例
func NewRepository(db *gorm.DB) Repository {
	return &repositoryImpl{db: db}
}

// SaveEvent 实现保存事件到 outbox 表
func (r *repositoryImpl) SaveEvent(
	tx *gorm.DB,
	eventType, aggregateID, aggregateType string,
	payload any,
) error {
	return r.SaveEventWithStatus(
		tx,
		eventType,
		aggregateID,
		aggregateType,
		payload,
		models.OutboxEventStatusPending,
	)
}

func (r *repositoryImpl) SaveEventWithStatus(
	tx *gorm.DB,
	eventType, aggregateID, aggregateType string,
	payload any,
	status string,
) error {
	// 1. 序列化payload 为 JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// 2.创建 OutboxEvent 对象
	event := &models.OutboxEvent{
		EventID:       uuid.New().String(), // 生成 UUID
		EventType:     eventType,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Payload:       string(payloadJSON),
		Status:        status,
		RetryCount:    0,
		CreatedAt:     time.Now(),
	}

	// 3.在事务中保存
	return tx.Create(event).Error
}

// Create 创建 Outbox 事件（接受已构建的 OutboxEvent）
func (r *repositoryImpl) Create(ctx context.Context, event *models.OutboxEvent) error {
	// 1. 参数验证（防御性编程）
	if event == nil {
		return errors.New("event 不能为 nil")
	}
	if event.EventID == "" {
		return errors.New("EventID 不能为空")
	}
	if event.EventType == "" {
		return errors.New("EventType 不能为空")
	}
	if event.Payload == "" {
		return errors.New("payload 不能为空")
	}

	// 2. 设置默认值（如果未设置）
	if event.Status == "" {
		event.Status = models.OutboxEventStatusPending
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if event.RetryCount == 0 {
		// 确保 retry_count 初始化为 0（避免数据库默认值问题）
		event.RetryCount = 0
	}

	// 3. 持久化到数据库
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("创建 Outbox 事件失败: %w", err)
	}

	return nil
}

// GetPendingEvents 获取待发布的事件
func (r *repositoryImpl) GetPendingEvents(
	ctx context.Context,
	limit int,
) ([]*models.OutboxEvent, error) {
	return r.GetRetryableEvents(
		ctx,
		[]string{models.OutboxEventStatusPending},
		limit,
		-1,
	)
}

// GetRetryableEvents 获取可处理/可重试的 outbox 事件列表
func (r *repositoryImpl) GetRetryableEvents(
	ctx context.Context,
	statuses []string, // 允许查询的事件状态集合；为空表示不过滤状态（不建议在生产中留空，可能扫全表）
	limit int, // 本次最多拉取多少条；用于控制单次处理耗时与 DB 压力
	maxRetry int, // 最大重试次数上限；>=0 启用 retry_count < maxRetry 过滤，<0 表示不限制重试
) ([]*models.OutboxEvent, error) {
	var events []*models.OutboxEvent // 承载查询结果（按 created_at 升序）

	// 构造基础查询：绑定 ctx（支持超时/取消），指定模型为 OutboxEvent
	db := r.db.WithContext(ctx).Model(&models.OutboxEvent{})

	// 按状态过滤：仅拉取目标状态（如 Pending / Failed / DQPending / DQFailed）
	if len(statuses) > 0 {
		db = db.Where("status IN ?", statuses)
	}

	// 按重试次数过滤：只取还能重试的事件（避免无限重试）
	if maxRetry >= 0 {
		db = db.Where("retry_count < ?", maxRetry)
	}
	// 稳定顺序：按创建时间从旧到新处理；limit 控制批量大小
	err := db.Order("created_at ASC").Limit(limit).Find(&events).Error

	// 返回结果与可能的查询错误（上层决定是否重试/告警）
	return events, err
}

// MarkAsPublished 标记事件为已发布
func (r *repositoryImpl) MarkAsPublished(ctx context.Context,
	eventID string) error {
	return r.MarkAsPublishedWithStatus(ctx, eventID, models.OutboxEventStatusPublished)
}

func (r *repositoryImpl) MarkAsPublishedWithStatus(
	ctx context.Context,
	eventID string,
	status string,
) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]any{
			"status":       status,
			"published_at": &now,
		}).Error
}

// MarkAsFailed 标记事件为发布失败
func (r *repositoryImpl) MarkAsFailed(
	ctx context.Context,
	eventID string,
	errorMsg string,
) error {
	return r.MarkAsFailedWithStatus(ctx, eventID, errorMsg, models.OutboxEventStatusFailed)
}

func (r *repositoryImpl) MarkAsFailedWithStatus(
	ctx context.Context,
	eventID string,
	errorMsg string,
	status string,
) error {
	return r.db.WithContext(ctx).
		Model(&models.OutboxEvent{}).
		Where("event_id = ?", eventID).
		Updates(map[string]any{
			"status":        status,
			"error_message": errorMsg,
			"retry_count":   gorm.Expr("retry_count + 1"),
		}).Error
}

// PublishInTransaction 在事务中发布事件的高级封装
func (r *repositoryImpl) PublishInTransaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}
