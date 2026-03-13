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

type ojDailyStatsProjectionEventPublisher interface {
	PublishOJDailyStatsProjectionEvent(ctx context.Context, event *eventdto.OJDailyStatsProjectionEvent) error
	PublishOJDailyStatsProjectionEventInTx(ctx context.Context, tx any, event *eventdto.OJDailyStatsProjectionEvent) error
}

type ojDailyStatsProjectionOutboxPublisher struct {
	outboxRepo interfaces.OutboxRepository
}

func newOJDailyStatsProjectionOutboxPublisher(
	outboxRepo interfaces.OutboxRepository,
) ojDailyStatsProjectionEventPublisher {
	return &ojDailyStatsProjectionOutboxPublisher{outboxRepo: outboxRepo}
}

func (p *ojDailyStatsProjectionOutboxPublisher) PublishOJDailyStatsProjectionEvent(
	ctx context.Context,
	event *eventdto.OJDailyStatsProjectionEvent,
) error {
	outboxEvent, err := buildOJDailyStatsProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.Create(ctx, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *ojDailyStatsProjectionOutboxPublisher) PublishOJDailyStatsProjectionEventInTx(
	ctx context.Context,
	tx any,
	event *eventdto.OJDailyStatsProjectionEvent,
) error {
	txDB, ok := tx.(*gorm.DB)
	if !ok || txDB == nil {
		return errors.New("invalid transaction for oj daily stats projection outbox")
	}
	outboxEvent, err := buildOJDailyStatsProjectionOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.CreateInTx(txDB, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *ojDailyStatsProjectionOutboxPublisher) notify(ctx context.Context) {
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil && global.Log != nil {
		global.Log.Warn("oj daily stats projection notify outbox failed", zap.Error(err))
	}
}

func buildOJDailyStatsProjectionOutboxEvent(
	ctx context.Context,
	event *eventdto.OJDailyStatsProjectionEvent,
) (*entity.OutboxEvent, error) {
	if event == nil || event.UserID == 0 {
		return nil, errors.New("invalid oj daily stats projection event")
	}
	if global.Config == nil {
		return nil, errors.New("global config is nil")
	}

	topic := strings.TrimSpace(global.Config.Messaging.OJDailyStatsProjectionTopic)
	if topic == "" {
		return nil, errors.New("oj daily stats projection topic config is empty")
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
		AggregateType: "oj_daily_stats_projection",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}, nil
}
