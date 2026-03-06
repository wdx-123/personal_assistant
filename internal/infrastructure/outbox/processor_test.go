package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/entity"
	obstrace "personal_assistant/pkg/observability/trace"
	"personal_assistant/pkg/observability/w3c"
	"personal_assistant/pkg/redislock"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type stubOutboxRepo struct {
	getPendingLimit      int
	getPendingMaxRetries int
	getPendingCalled     int

	markFailedEventID     string
	markFailedErrorMsg    string
	markFailedMaxRetries  int
	markFailedCalled      int
	markPublishedEventID  string
	markPublishedCalled   int
	deletePublishedBefore time.Time
	deletePublishedCalled int
	deleteFailedBefore    time.Time
	deleteFailedCalled    int
	countByStatusCalled   int

	events []*entity.OutboxEvent
}

func (s *stubOutboxRepo) Create(ctx context.Context, event *entity.OutboxEvent) error {
	return nil
}

func (s *stubOutboxRepo) CreateInTx(tx *gorm.DB, event *entity.OutboxEvent) error {
	return nil
}

func (s *stubOutboxRepo) GetPendingEvents(ctx context.Context, limit int, maxRetries int) ([]*entity.OutboxEvent, error) {
	s.getPendingCalled++
	s.getPendingLimit = limit
	s.getPendingMaxRetries = maxRetries
	return s.events, nil
}

func (s *stubOutboxRepo) MarkAsPublished(ctx context.Context, eventID string) error {
	s.markPublishedCalled++
	s.markPublishedEventID = eventID
	return nil
}

func (s *stubOutboxRepo) MarkAsFailed(ctx context.Context, eventID string, errorMsg string, maxRetries int) error {
	s.markFailedCalled++
	s.markFailedEventID = eventID
	s.markFailedErrorMsg = errorMsg
	s.markFailedMaxRetries = maxRetries
	return nil
}

func (s *stubOutboxRepo) DeletePublishedBefore(ctx context.Context, before time.Time) error {
	s.deletePublishedCalled++
	s.deletePublishedBefore = before
	return nil
}

func (s *stubOutboxRepo) DeleteFailedBefore(ctx context.Context, before time.Time) error {
	s.deleteFailedCalled++
	s.deleteFailedBefore = before
	return nil
}

func (s *stubOutboxRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	s.countByStatusCalled++
	return map[string]int64{}, nil
}

type stubPublisher struct {
	err error
	msg *messaging.Message
}

func (s *stubPublisher) Publish(ctx context.Context, msg *messaging.Message) error {
	if msg != nil {
		clone := *msg
		if msg.Metadata != nil {
			clone.Metadata = make(map[string]string, len(msg.Metadata))
			for k, v := range msg.Metadata {
				clone.Metadata[k] = v
			}
		}
		s.msg = &clone
	}
	return s.err
}

func (s *stubPublisher) Close() error {
	return nil
}

type stubTraceBackend struct {
	spans []*obstrace.Span
}

type stubRelayLock struct {
	tryErr    error
	unlockErr error
	tryCalled int
}

func (s *stubRelayLock) TryLock() error {
	s.tryCalled++
	return s.tryErr
}

func (s *stubRelayLock) Unlock() error {
	return s.unlockErr
}

func (s *stubTraceBackend) RecordSpan(ctx context.Context, span *obstrace.Span) error {
	if span != nil {
		copySpan := *span
		s.spans = append(s.spans, &copySpan)
	}
	return nil
}

func (s *stubTraceBackend) ListByRequestID(
	ctx context.Context,
	requestID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*obstrace.Span, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceBackend) ListByTraceID(
	ctx context.Context,
	traceID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*obstrace.Span, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceBackend) Query(ctx context.Context, q *obstrace.Query) ([]*obstrace.Span, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceBackend) CleanupBeforeByStatus(ctx context.Context, status string, before time.Time) error {
	return nil
}

func (s *stubTraceBackend) Start(ctx context.Context) error {
	return nil
}

func (s *stubTraceBackend) StartConsumer(ctx context.Context) error {
	return nil
}

func TestRelayProcessor_Process_PublishFail_MarkFailed(t *testing.T) {
	traceBackend := &stubTraceBackend{}
	setTraceGlobals(t, traceBackend)
	lock := &stubRelayLock{}
	setRelayLockFactory(t, func(ctx context.Context, key string, ttl time.Duration) relayLocker {
		return lock
	})

	traceID := "0123456789abcdef0123456789abcdef"
	parentSpanID := "1111111111111111"
	repo := &stubOutboxRepo{
		events: []*entity.OutboxEvent{
			{
				MODEL:         entity.MODEL{CreatedAt: time.Now()},
				EventID:       "e1",
				EventType:     "t1",
				AggregateID:   "a1",
				AggregateType: "at1",
				Payload:       `{"k":"v"}`,
				TraceID:       traceID,
				RequestID:     "req-1",
				TraceParent:   w3c.BuildTraceparent(w3c.TraceContext{TraceID: traceID, SpanID: parentSpanID, TraceFlags: "01"}),
				Status:        entity.OutboxEventStatusPending,
			},
		},
	}
	pub := &stubPublisher{err: errors.New("fail")}
	p := NewRelayProcessor(repo, pub, zap.NewNop())

	_ = p.Process(context.Background())

	if repo.getPendingCalled != 1 {
		t.Fatalf("expected GetPendingEvents called once, got %d", repo.getPendingCalled)
	}
	if lock.tryCalled != 1 {
		t.Fatalf("expected TryLock called once, got %d", lock.tryCalled)
	}
	if repo.getPendingLimit != 100 {
		t.Fatalf("unexpected limit: %d", repo.getPendingLimit)
	}
	if repo.getPendingMaxRetries != 3 {
		t.Fatalf("unexpected maxRetries: %d", repo.getPendingMaxRetries)
	}
	if repo.markFailedCalled != 1 {
		t.Fatalf("expected MarkAsFailed called once, got %d", repo.markFailedCalled)
	}
	if repo.markFailedEventID != "e1" {
		t.Fatalf("unexpected eventID: %s", repo.markFailedEventID)
	}
	if repo.markFailedMaxRetries != 3 {
		t.Fatalf("unexpected maxRetries: %d", repo.markFailedMaxRetries)
	}
	if repo.markPublishedCalled != 0 {
		t.Fatalf("expected MarkAsPublished not called, got %d", repo.markPublishedCalled)
	}
	if len(traceBackend.spans) != 1 {
		t.Fatalf("expected one publish span, got %d", len(traceBackend.spans))
	}
	span := traceBackend.spans[0]
	if span.Stage != "event.publish" || span.Status != obstrace.SpanStatusError {
		t.Fatalf("unexpected span stage/status: %s %s", span.Stage, span.Status)
	}
	if span.ErrorCode != "event_publish_error" {
		t.Fatalf("unexpected span error code: %s", span.ErrorCode)
	}
}

func TestRelayProcessor_Process_PublishOK_MarkPublished(t *testing.T) {
	traceBackend := &stubTraceBackend{}
	setTraceGlobals(t, traceBackend)
	lock := &stubRelayLock{}
	setRelayLockFactory(t, func(ctx context.Context, key string, ttl time.Duration) relayLocker {
		return lock
	})

	traceID := "0123456789abcdef0123456789abcdef"
	parentSpanID := "1111111111111111"
	repo := &stubOutboxRepo{
		events: []*entity.OutboxEvent{
			{
				MODEL:         entity.MODEL{CreatedAt: time.Now()},
				EventID:       "e1",
				EventType:     "t1",
				AggregateID:   "a1",
				AggregateType: "at1",
				Payload:       `{"k":"v"}`,
				TraceID:       traceID,
				RequestID:     "req-1",
				TraceParent:   w3c.BuildTraceparent(w3c.TraceContext{TraceID: traceID, SpanID: parentSpanID, TraceFlags: "01"}),
				TraceState:    "vendor=test",
				Status:        entity.OutboxEventStatusPending,
			},
		},
	}
	pub := &stubPublisher{}
	p := NewRelayProcessor(repo, pub, zap.NewNop())

	_ = p.Process(context.Background())

	if repo.markPublishedCalled != 1 {
		t.Fatalf("expected MarkAsPublished called once, got %d", repo.markPublishedCalled)
	}
	if lock.tryCalled != 1 {
		t.Fatalf("expected TryLock called once, got %d", lock.tryCalled)
	}
	if repo.markPublishedEventID != "e1" {
		t.Fatalf("unexpected eventID: %s", repo.markPublishedEventID)
	}
	if repo.markFailedCalled != 0 {
		t.Fatalf("expected MarkAsFailed not called, got %d", repo.markFailedCalled)
	}
	if pub.msg == nil {
		t.Fatalf("expected published message to be captured")
	}
	if pub.msg.Metadata["request_id"] != "req-1" {
		t.Fatalf("unexpected request_id metadata: %s", pub.msg.Metadata["request_id"])
	}
	if len(traceBackend.spans) != 1 {
		t.Fatalf("expected one publish span, got %d", len(traceBackend.spans))
	}
	span := traceBackend.spans[0]
	if span.Stage != "event.publish" || span.Kind != "producer" || span.Status != obstrace.SpanStatusOK {
		t.Fatalf("unexpected span metadata: stage=%s kind=%s status=%s", span.Stage, span.Kind, span.Status)
	}
	if span.ParentSpanID != parentSpanID {
		t.Fatalf("expected parent span %s, got %s", parentSpanID, span.ParentSpanID)
	}

	traceparent := pub.msg.Metadata["traceparent"]
	parsed, ok := w3c.ParseTraceparent(traceparent)
	if !ok {
		t.Fatalf("expected valid traceparent metadata, got %q", traceparent)
	}
	if parsed.TraceID != traceID {
		t.Fatalf("expected trace id %s, got %s", traceID, parsed.TraceID)
	}
	if parsed.SpanID != span.SpanID {
		t.Fatalf("expected metadata span id %s, got %s", span.SpanID, parsed.SpanID)
	}
}

func TestRelayProcessor_Process_LockNotAcquired_Skip(t *testing.T) {
	lock := &stubRelayLock{tryErr: redislock.ErrLockFailed}
	setRelayLockFactory(t, func(ctx context.Context, key string, ttl time.Duration) relayLocker {
		return lock
	})
	setTraceGlobals(t, &stubTraceBackend{})

	repo := &stubOutboxRepo{
		events: []*entity.OutboxEvent{
			{EventID: "e1", Status: entity.OutboxEventStatusPending},
		},
	}
	p := NewRelayProcessor(repo, &stubPublisher{}, zap.NewNop())

	if err := p.Process(context.Background()); err != nil {
		t.Fatalf("expected no error when lock not acquired, got %v", err)
	}
	if repo.getPendingCalled != 0 {
		t.Fatalf("expected GetPendingEvents not called, got %d", repo.getPendingCalled)
	}
}

func TestRelayProcessor_Process_LockError_ReturnsError(t *testing.T) {
	lock := &stubRelayLock{tryErr: errors.New("redis down")}
	setRelayLockFactory(t, func(ctx context.Context, key string, ttl time.Duration) relayLocker {
		return lock
	})
	setTraceGlobals(t, &stubTraceBackend{})

	p := NewRelayProcessor(&stubOutboxRepo{}, &stubPublisher{}, zap.NewNop())
	if err := p.Process(context.Background()); err == nil {
		t.Fatalf("expected Process to return lock error")
	}
}

func setTraceGlobals(t *testing.T, traceBackend obstrace.TraceBackend) {
	t.Helper()

	oldConfig := global.Config
	oldTraceBackend := global.ObservabilityTraces
	global.Config = &config.Config{
		Messaging: config.Messaging{
			OutboxRelayLockEnabled:    true,
			OutboxRelayLockTTLSeconds: 15,
		},
		Observability: config.Observability{
			ServiceName: "personal_assistant",
		},
	}
	global.ObservabilityTraces = traceBackend
	t.Cleanup(func() {
		global.Config = oldConfig
		global.ObservabilityTraces = oldTraceBackend
	})
}

func setRelayLockFactory(
	t *testing.T,
	factory func(ctx context.Context, key string, ttl time.Duration) relayLocker,
) {
	t.Helper()

	oldFactory := newRelayLocker
	newRelayLocker = factory
	t.Cleanup(func() {
		newRelayLocker = oldFactory
	})
}
