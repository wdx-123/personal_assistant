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

type permissionProjectionEventPublisher interface {
	Publish(ctx context.Context, event *eventdto.PermissionProjectionEvent) error
	PublishInTx(ctx context.Context, tx any, event *eventdto.PermissionProjectionEvent) error
}

type permissionProjectionOutboxPublisher struct {
	outboxRepo interfaces.OutboxRepository
}

func newPermissionProjectionOutboxPublisher(
	outboxRepo interfaces.OutboxRepository,
) permissionProjectionEventPublisher {
	return &permissionProjectionOutboxPublisher{outboxRepo: outboxRepo}
}

func (p *permissionProjectionOutboxPublisher) Publish(
	ctx context.Context,
	event *eventdto.PermissionProjectionEvent,
) error {
	outboxEvent, err := buildPermissionProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.Create(ctx, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *permissionProjectionOutboxPublisher) PublishInTx(
	ctx context.Context,
	tx any,
	event *eventdto.PermissionProjectionEvent,
) error {
	txDB, ok := tx.(*gorm.DB)
	if !ok || txDB == nil {
		return errors.New("invalid transaction for permission projection outbox")
	}
	outboxEvent, err := buildPermissionProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.CreateInTx(txDB, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *permissionProjectionOutboxPublisher) notify(ctx context.Context) {
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil && global.Log != nil {
		global.Log.Warn("permission projection notify outbox failed", zap.Error(err))
	}
}

// buildPermissionProjectionOutboxEvent 构建权限投影事件的OutboxEvent
func buildPermissionProjectionOutboxEvent(
	ctx context.Context,
	event *eventdto.PermissionProjectionEvent,
) (*entity.OutboxEvent, error) {
	if event == nil {
		return nil, errors.New("invalid permission projection event")
	}
	topic := strings.TrimSpace(global.Config.Messaging.PermissionProjectionTopic)
	if topic == "" {
		return nil, errors.New("permission projection topic config is empty")
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	aggregateType := strings.TrimSpace(event.AggregateType)
	if aggregateType == "" {
		aggregateType = "permission_projection"
	}
	aggregateID := strconv.FormatUint(uint64(event.AggregateID), 10)
	if event.UserID > 0 {
		aggregateID = strconv.FormatUint(uint64(event.UserID), 10)
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)
	return &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}, nil
}


type permissionPolicyReloadBroadcaster interface {
	// Broadcast 广播权限策略变更通知
	Broadcast(ctx context.Context) error
}

type redisPermissionPolicyReloadBroadcaster struct{}

// newPermissionPolicyReloadBroadcaster 创建基于Redis的权限策略变更广播器
func newPermissionPolicyReloadBroadcaster() permissionPolicyReloadBroadcaster {
	return &redisPermissionPolicyReloadBroadcaster{}
}

// Broadcast 通过Redis发布权限策略变更通知
func (b *redisPermissionPolicyReloadBroadcaster) Broadcast(ctx context.Context) error {
	channel := strings.TrimSpace(global.Config.Messaging.PermissionPolicyReloadChannel)
	if global.Redis == nil || channel == "" {
		return nil
	}
	return global.Redis.Publish(ctx, channel, "1").Err()
}
