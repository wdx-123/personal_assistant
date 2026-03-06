package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/observability/contextid"
	obstrace "personal_assistant/pkg/observability/trace"
	"personal_assistant/pkg/observability/w3c"
	"personal_assistant/pkg/redislock"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RelayProcessor 负责将 Outbox 表中的事件转发到消息队列
type RelayProcessor struct {
	repo       interfaces.OutboxRepository
	publisher  messaging.Publisher
	logger     *zap.Logger
	batchSize  int
	maxRetries int

	pollInterval  time.Duration
	notifyChannel string
	lockEnabled   bool
	lockTTL       time.Duration
	lockKey       string
}

type relayLocker interface {
	TryLock() error
	Unlock() error
}

var newRelayLocker = func(ctx context.Context, key string, ttl time.Duration) relayLocker {
	return redislock.NewRedisLock(ctx, key, ttl)
}

func NewRelayProcessor(
	repo interfaces.OutboxRepository,
	publisher messaging.Publisher,
	logger *zap.Logger,
) *RelayProcessor {
	lockEnabled := true
	lockTTL := 15 * time.Second
	if global.Config != nil {
		lockEnabled = global.Config.Messaging.OutboxRelayLockEnabled
		if global.Config.Messaging.OutboxRelayLockTTLSeconds > 0 {
			lockTTL = time.Duration(global.Config.Messaging.OutboxRelayLockTTLSeconds) * time.Second
		}
	}
	return &RelayProcessor{
		repo:          repo,
		publisher:     publisher,
		logger:        logger,
		batchSize:     100, // 默认批次大小
		maxRetries:    3,
		pollInterval:  time.Second,
		notifyChannel: "outbox_new_event",
		lockEnabled:   lockEnabled,
		lockTTL:       lockTTL,
		lockKey:       redislock.LockKeyOutboxRelayProcess,
	}
}

// Process 执行一次轮询和转发
func (p *RelayProcessor) Process(ctx context.Context) error {
	var lock relayLocker
	if p.lockEnabled {
		lock = newRelayLocker(ctx, p.lockKey, p.lockTTL)
		if err := lock.TryLock(); err != nil {
			if errors.Is(err, redislock.ErrLockFailed) {
				return nil
			}
			p.logger.Error("OutboxRelay: 获取分布式锁失败", zap.String("lock_key", p.lockKey), zap.Error(err))
			return err
		}
		defer func() {
			if err := lock.Unlock(); err != nil {
				p.logger.Error("OutboxRelay: 释放分布式锁失败", zap.String("lock_key", p.lockKey), zap.Error(err))
			}
		}()
	}

	// 1. 获取待发布事件
	events, err := p.repo.GetPendingEvents(ctx, p.batchSize, p.maxRetries)
	if err != nil {
		p.logger.Error("OutboxRelay: 获取待发布事件失败", zap.Error(err))
		return err
	}

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		publishCtx := p.buildPublishContext(ctx, event)
		publishCtx, spanEvent := p.startPublishSpan(publishCtx, event)

		// 2. 构建消息
		msg := &messaging.Message{
			ID:          event.EventID,
			Topic:       event.EventType,
			Payload:     []byte(event.Payload),
			OccurredAt:  event.CreatedAt,
			PublishedAt: time.Now(),
			Metadata: map[string]string{
				"aggregate_id":   event.AggregateID,
				"aggregate_type": event.AggregateType,
				"source":         "outbox",
			},
		}
		publishCtx = p.injectTraceMetadata(publishCtx, msg)

		// 3. 发布消息
		if err := p.publisher.Publish(publishCtx, msg); err != nil {
			p.finishPublishSpan(publishCtx, event, spanEvent, err)
			p.logger.Error("OutboxRelay: 推送消息失败",
				zap.String("event_id", event.EventID),
				zap.Error(err))

			// 标记失败
			if markErr := p.repo.MarkAsFailed(publishCtx, event.EventID, err.Error(), p.maxRetries); markErr != nil {
				p.logger.Error("OutboxRelay: 标记失败状态出错", zap.Error(markErr))
			}
			continue
		}
		p.finishPublishSpan(publishCtx, event, spanEvent, nil)

		// 4. 标记成功
		if err := p.repo.MarkAsPublished(publishCtx, event.EventID); err != nil {
			p.logger.Error("OutboxRelay: 标记发布状态出错", zap.Error(err))
		}
	}

	return nil
}

func (p *RelayProcessor) Run(ctx context.Context, redisClient *redis.Client) error {
	pollTicker := time.NewTicker(p.pollInterval)
	defer pollTicker.Stop()

	var sub *redis.PubSub
	var subCh <-chan *redis.Message
	if redisClient != nil {
		sub = redisClient.Subscribe(ctx, p.notifyChannel)
		if _, err := sub.Receive(ctx); err != nil {
			_ = sub.Close()
			sub = nil
		} else {
			subCh = sub.Channel()
		}
	}
	if sub != nil {
		defer func() {
			_ = sub.Close()
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			_ = p.Process(ctx)
		case <-subCh:
			_ = p.Process(ctx)
		}
	}
}

func NotifyNewOutboxEvent(
	ctx context.Context,
	redisClient *redis.Client,
) error {
	if redisClient == nil {
		return nil
	}
	return redisClient.Publish(ctx, "outbox_new_event", "1").Err()
}

func (p *RelayProcessor) buildPublishContext(ctx context.Context, event *entity.OutboxEvent) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if event == nil {
		ctx, _ = contextid.EnsureIDs(ctx)
		return ctx
	}

	ids := contextid.IDs{
		RequestID: strings.TrimSpace(event.RequestID),
		TraceID:   strings.TrimSpace(event.TraceID),
	}
	ctx = contextid.IntoContext(ctx, ids)

	traceparent := strings.TrimSpace(event.TraceParent)
	if parsed, ok := w3c.ParseTraceparent(traceparent); ok {
		parsed.TraceState = strings.TrimSpace(event.TraceState)
		ids.TraceID = parsed.TraceID
		ctx = contextid.IntoContext(ctx, ids)
		ctx = contextid.IntoTraceContext(ctx, contextid.TraceContext(parsed))
		ctx = contextid.WithIncomingParentSpanID(ctx, parsed.SpanID)
	} else {
		ctx = contextid.WithIncomingParentSpanID(ctx, "")
	}

	ctx, _ = contextid.EnsureIDs(ctx)
	return ctx
}

func (p *RelayProcessor) startPublishSpan(
	ctx context.Context,
	event *entity.OutboxEvent,
) (context.Context, *obstrace.SpanEvent) {
	if global.ObservabilityTraces == nil || event == nil {
		return ctx, nil
	}

	return obstrace.StartSpan(ctx, obstrace.StartOptions{
		Service: resolveTraceServiceName(),
		Stage:   "event.publish",
		Name:    "redis.stream.publish",
		Kind:    "producer",
		Tags: map[string]string{
			"topic":          strings.TrimSpace(event.EventType),
			"event_id":       strings.TrimSpace(event.EventID),
			"aggregate_id":   strings.TrimSpace(event.AggregateID),
			"aggregate_type": strings.TrimSpace(event.AggregateType),
			"source":         "outbox",
		},
	})
}

func (p *RelayProcessor) finishPublishSpan(
	ctx context.Context,
	event *entity.OutboxEvent,
	spanEvent *obstrace.SpanEvent,
	err error,
) {
	if spanEvent == nil || global.ObservabilityTraces == nil || event == nil {
		return
	}

	status := obstrace.SpanStatusOK
	code := ""
	message := ""
	if err != nil {
		status = obstrace.SpanStatusError
		code = "event_publish_error"
		message = err.Error()
		spanEvent.WithErrorDetail(buildPublishErrorDetail(event, err))
	}
	span := spanEvent.End(status, code, message, nil)
	_ = global.ObservabilityTraces.RecordSpan(ctx, span)
}

func (p *RelayProcessor) injectTraceMetadata(ctx context.Context, msg *messaging.Message) context.Context {
	if msg == nil {
		return ctx
	}
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}

	ctx, ids := contextid.EnsureIDs(ctx)
	if ids.RequestID != "" {
		msg.Metadata["request_id"] = ids.RequestID
	}
	if ids.TraceID != "" {
		msg.Metadata["trace_id"] = ids.TraceID
	}

	ctx, tc := contextid.EnsureTraceContext(ctx)
	if traceparent := w3c.BuildTraceparent(tc); traceparent != "" {
		msg.Metadata["traceparent"] = traceparent
	}
	if tracestate := strings.TrimSpace(tc.TraceState); tracestate != "" {
		msg.Metadata["tracestate"] = tracestate
	}
	return ctx
}

func buildPublishErrorDetail(event *entity.OutboxEvent, err error) string {
	payload := map[string]string{
		"stage":          "event.publish",
		"source":         "outbox",
		"event_id":       strings.TrimSpace(event.EventID),
		"topic":          strings.TrimSpace(event.EventType),
		"aggregate_id":   strings.TrimSpace(event.AggregateID),
		"aggregate_type": strings.TrimSpace(event.AggregateType),
	}
	if err != nil {
		payload["error"] = strings.TrimSpace(err.Error())
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return ""
	}
	return string(data)
}

func resolveTraceServiceName() string {
	if global.Config == nil {
		return "personal_assistant"
	}
	serviceName := strings.TrimSpace(global.Config.Observability.ServiceName)
	if serviceName == "" {
		return "personal_assistant"
	}
	return serviceName
}
