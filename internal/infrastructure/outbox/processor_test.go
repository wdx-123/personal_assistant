package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/model/entity"

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

type stubPublisher struct {
	err error
}

func (s *stubPublisher) Publish(ctx context.Context, msg *messaging.Message) error {
	return s.err
}

func (s *stubPublisher) Close() error {
	return nil
}

func TestRelayProcessor_Process_PublishFail_MarkFailed(t *testing.T) {
	repo := &stubOutboxRepo{
		events: []*entity.OutboxEvent{
			{
				MODEL:         entity.MODEL{CreatedAt: time.Now()},
				EventID:       "e1",
				EventType:     "t1",
				AggregateID:   "a1",
				AggregateType: "at1",
				Payload:       `{"k":"v"}`,
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
}

func TestRelayProcessor_Process_PublishOK_MarkPublished(t *testing.T) {
	repo := &stubOutboxRepo{
		events: []*entity.OutboxEvent{
			{
				MODEL:         entity.MODEL{CreatedAt: time.Now()},
				EventID:       "e1",
				EventType:     "t1",
				AggregateID:   "a1",
				AggregateType: "at1",
				Payload:       `{"k":"v"}`,
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
	if repo.markPublishedEventID != "e1" {
		t.Fatalf("unexpected eventID: %s", repo.markPublishedEventID)
	}
	if repo.markFailedCalled != 0 {
		t.Fatalf("expected MarkAsFailed not called, got %d", repo.markFailedCalled)
	}
}
