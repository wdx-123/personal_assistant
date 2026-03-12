package system

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/outbox"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type cacheProjectionOutboxPublisher struct {
	outboxRepo interfaces.OutboxRepository
}

type cacheProjectionEventPublisher interface {
	Publish(ctx context.Context, event *eventdto.CacheProjectionEvent) error
	PublishInTx(ctx context.Context, tx any, event *eventdto.CacheProjectionEvent) error
}

func newCacheProjectionOutboxPublisher(
	outboxRepo interfaces.OutboxRepository,
) cacheProjectionEventPublisher {
	return &cacheProjectionOutboxPublisher{outboxRepo: outboxRepo}
}

// Publish 发布事件到 Outbox，供异步处理
func (p *cacheProjectionOutboxPublisher) Publish(
	ctx context.Context,
	event *eventdto.CacheProjectionEvent,
) error {
	outboxEvent, err := buildCacheProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.Create(ctx, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

// PublishInTx 在事务中发布事件，确保与业务操作原子性一致
func (p *cacheProjectionOutboxPublisher) PublishInTx(
	ctx context.Context,
	tx any,
	event *eventdto.CacheProjectionEvent,
) error {
	txDB, ok := tx.(*gorm.DB)
	if !ok || txDB == nil {
		return errors.New("invalid transaction for cache projection outbox")
	}
	outboxEvent, err := buildCacheProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.CreateInTx(txDB, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

// 通知 Outbox 处理器有新的事件需要处理
func (p *cacheProjectionOutboxPublisher) notify(ctx context.Context) {
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil && global.Log != nil {
		global.Log.Warn("cache projection notify outbox failed", zap.Error(err))
	}
}

// 构建缓存投影事件的 OutboxEvent 实体
func buildCacheProjectionOutboxEvent(
	ctx context.Context,
	event *eventdto.CacheProjectionEvent,
) (*entity.OutboxEvent, error) {
	if event == nil || event.UserID == 0 {
		return nil, errors.New("invalid cache projection event")
	}
	if global.Config == nil {
		return nil, errors.New("global config is nil")
	}

	// 从配置中获取事件类型（topic），并进行基本验证
	topic := strings.TrimSpace(global.Config.Messaging.CacheProjectionTopic)
	if topic == "" {
		return nil, errors.New("cache projection topic config is empty")
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)
	return &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(event.UserID), 10),
		AggregateType: "user_projection",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}, nil
}
