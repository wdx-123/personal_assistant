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

type ojTaskExecutionTriggerEventPublisher interface {
	PublishInTx(ctx context.Context, tx any, event *eventdto.OJTaskExecutionTriggerEvent) error
}

type ojTaskExecutionTriggerOutboxPublisher struct {
	outboxRepo interfaces.OutboxRepository
}

func newOJTaskExecutionTriggerOutboxPublisher(
	outboxRepo interfaces.OutboxRepository,
) ojTaskExecutionTriggerEventPublisher {
	return &ojTaskExecutionTriggerOutboxPublisher{outboxRepo: outboxRepo}
}

func (p *ojTaskExecutionTriggerOutboxPublisher) PublishInTx(
	ctx context.Context,
	tx any,
	event *eventdto.OJTaskExecutionTriggerEvent,
) error {
	txDB, ok := tx.(*gorm.DB)
	if !ok || txDB == nil {
		return errors.New("invalid transaction for oj task trigger outbox")
	}
	outboxEvent, err := buildOJTaskExecutionTriggerOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.CreateInTx(txDB, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *ojTaskExecutionTriggerOutboxPublisher) notify(ctx context.Context) {
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil && global.Log != nil {
		global.Log.Warn("oj task trigger notify outbox failed", zap.Error(err))
	}
}

func buildOJTaskExecutionTriggerOutboxEvent(
	ctx context.Context,
	event *eventdto.OJTaskExecutionTriggerEvent,
) (*entity.OutboxEvent, error) {
	if event == nil || event.ExecutionID == 0 || event.TaskID == 0 {
		return nil, errors.New("invalid oj task execution trigger event")
	}
	if global.Config == nil {
		return nil, errors.New("global config is nil")
	}

	topic := strings.TrimSpace(global.Config.Messaging.OJTaskExecutionTriggerTopic)
	if topic == "" {
		return nil, errors.New("oj task execution trigger topic config is empty")
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)
	return &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(event.ExecutionID), 10),
		AggregateType: "oj_task_execution",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}, nil
}
