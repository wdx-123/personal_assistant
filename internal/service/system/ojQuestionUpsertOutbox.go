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

type ojQuestionUpsertEventPublisher interface {
	Publish(ctx context.Context, event *eventdto.QuestionUpsertedEvent) error
	PublishInTx(ctx context.Context, tx any, event *eventdto.QuestionUpsertedEvent) error
}

type ojQuestionUpsertOutboxPublisher struct {
	outboxRepo interfaces.OutboxRepository
}

func newOJQuestionUpsertOutboxPublisher(
	outboxRepo interfaces.OutboxRepository,
) ojQuestionUpsertEventPublisher {
	return &ojQuestionUpsertOutboxPublisher{outboxRepo: outboxRepo}
}

func (p *ojQuestionUpsertOutboxPublisher) Publish(
	ctx context.Context,
	event *eventdto.QuestionUpsertedEvent,
) error {
	outboxEvent, err := buildOJQuestionUpsertOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.Create(ctx, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *ojQuestionUpsertOutboxPublisher) PublishInTx(
	ctx context.Context,
	tx any,
	event *eventdto.QuestionUpsertedEvent,
) error {
	txDB, ok := tx.(*gorm.DB)
	if !ok || txDB == nil {
		return errors.New("invalid transaction for oj question upsert outbox")
	}
	outboxEvent, err := buildOJQuestionUpsertOutboxEvent(ctx, event)
	if err != nil {
		return err
	}
	if err := p.outboxRepo.CreateInTx(txDB, outboxEvent); err != nil {
		return err
	}
	p.notify(ctx)
	return nil
}

func (p *ojQuestionUpsertOutboxPublisher) notify(ctx context.Context) {
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil && global.Log != nil {
		global.Log.Warn("oj question upsert notify outbox failed", zap.Error(err))
	}
}

func buildOJQuestionUpsertOutboxEvent(
	ctx context.Context,
	event *eventdto.QuestionUpsertedEvent,
) (*entity.OutboxEvent, error) {
	if event == nil || strings.TrimSpace(event.Platform) == "" || event.QuestionID == 0 || strings.TrimSpace(event.Title) == "" {
		return nil, errors.New("invalid oj question upsert event")
	}
	if global.Config == nil {
		return nil, errors.New("global config is nil")
	}

	topic := strings.TrimSpace(global.Config.Messaging.OJQuestionUpsertTopic)
	if topic == "" {
		return nil, errors.New("oj question upsert topic config is empty")
	}

	payloadBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)
	return &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(event.QuestionID), 10),
		AggregateType: "oj_question",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}, nil
}
