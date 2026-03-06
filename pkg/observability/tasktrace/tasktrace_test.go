package tasktrace

import (
	"context"
	"errors"
	"testing"
	"time"

	obstrace "personal_assistant/pkg/observability/trace"
	"personal_assistant/pkg/redislock"
)

type stubTraceBackend struct {
	spans []*obstrace.Span
}

type stubTaskLock struct {
	tryErr    error
	unlockErr error
	tryCalled int
}

func (s *stubTaskLock) TryLock() error {
	s.tryCalled++
	return s.tryErr
}

func (s *stubTaskLock) Unlock() error {
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

func TestWrap_RecordsTaskSpan(t *testing.T) {
	backend := &stubTraceBackend{}
	called := 0
	lock := &stubTaskLock{}
	setTaskLockerFactory(t, func(ctx context.Context, key string, ttl time.Duration) taskLocker {
		return lock
	})

	Wrap("RankingSyncTask", Options{
		Backend:     backend,
		ServiceName: "personal_assistant",
		LockEnabled: true,
		LockKey:     "task:cron:RankingSyncTask",
		LockTTL:     30 * time.Second,
	}, func(ctx context.Context) error {
		called++
		return nil
	})()

	if called != 1 {
		t.Fatalf("expected task handler to be called once, got %d", called)
	}
	if len(backend.spans) != 1 {
		t.Fatalf("expected one task span, got %d", len(backend.spans))
	}
	span := backend.spans[0]
	if span.Stage != "task" || span.Kind != "cron" || span.Status != obstrace.SpanStatusOK {
		t.Fatalf("unexpected span metadata: stage=%s kind=%s status=%s", span.Stage, span.Kind, span.Status)
	}
	if span.Tags["task"] != "RankingSyncTask" || span.Tags["trigger"] != "cron" {
		t.Fatalf("unexpected task tags: %+v", span.Tags)
	}
	if span.Tags["lock_acquired"] != "true" || span.Tags["execution_result"] != "success" {
		t.Fatalf("unexpected lock tags: %+v", span.Tags)
	}
	if lock.tryCalled != 1 {
		t.Fatalf("expected TryLock called once, got %d", lock.tryCalled)
	}
}

func TestWrap_RecordsTaskErrorSpan(t *testing.T) {
	backend := &stubTraceBackend{}
	lock := &stubTaskLock{}
	setTaskLockerFactory(t, func(ctx context.Context, key string, ttl time.Duration) taskLocker {
		return lock
	})

	Wrap("ImageOrphanCleanupTask", Options{
		Backend:     backend,
		ServiceName: "personal_assistant",
		LockEnabled: true,
		LockKey:     "task:cron:ImageOrphanCleanupTask",
		LockTTL:     30 * time.Second,
	}, func(ctx context.Context) error {
		return errors.New("boom")
	})()

	if len(backend.spans) != 1 {
		t.Fatalf("expected one task span, got %d", len(backend.spans))
	}
	span := backend.spans[0]
	if span.Status != obstrace.SpanStatusError || span.ErrorCode != "task_error" {
		t.Fatalf("unexpected error span metadata: status=%s code=%s", span.Status, span.ErrorCode)
	}
	if span.Tags["execution_result"] != "error" {
		t.Fatalf("unexpected execution_result: %+v", span.Tags)
	}
}

func TestWrap_SkipsWhenLockNotAcquired(t *testing.T) {
	backend := &stubTraceBackend{}
	lock := &stubTaskLock{tryErr: redislock.ErrLockFailed}
	setTaskLockerFactory(t, func(ctx context.Context, key string, ttl time.Duration) taskLocker {
		return lock
	})

	called := 0
	Wrap("LuoguSyncTask", Options{
		Backend:     backend,
		ServiceName: "personal_assistant",
		LockEnabled: true,
		LockKey:     "task:cron:LuoguSyncTask",
		LockTTL:     30 * time.Second,
	}, func(ctx context.Context) error {
		called++
		return nil
	})()

	if called != 0 {
		t.Fatalf("expected task handler not to run, got %d", called)
	}
	if len(backend.spans) != 1 {
		t.Fatalf("expected one span, got %d", len(backend.spans))
	}
	span := backend.spans[0]
	if span.Status != obstrace.SpanStatusOK || span.Message != "lock not acquired" {
		t.Fatalf("unexpected skipped span: status=%s message=%s", span.Status, span.Message)
	}
	if span.Tags["execution_result"] != "skipped" || span.Tags["lock_acquired"] != "false" {
		t.Fatalf("unexpected skipped tags: %+v", span.Tags)
	}
}

func TestWrap_RecordsLockErrorSpan(t *testing.T) {
	backend := &stubTraceBackend{}
	lock := &stubTaskLock{tryErr: errors.New("redis down")}
	setTaskLockerFactory(t, func(ctx context.Context, key string, ttl time.Duration) taskLocker {
		return lock
	})

	Wrap("RankingSyncTask", Options{
		Backend:     backend,
		ServiceName: "personal_assistant",
		LockEnabled: true,
		LockKey:     "task:cron:RankingSyncTask",
		LockTTL:     30 * time.Second,
	}, func(ctx context.Context) error {
		return nil
	})()

	if len(backend.spans) != 1 {
		t.Fatalf("expected one span, got %d", len(backend.spans))
	}
	span := backend.spans[0]
	if span.Status != obstrace.SpanStatusError || span.ErrorCode != "task_lock_error" {
		t.Fatalf("unexpected lock error span: status=%s code=%s", span.Status, span.ErrorCode)
	}
}

func TestWrap_RecordsUnlockErrorSpan(t *testing.T) {
	backend := &stubTraceBackend{}
	lock := &stubTaskLock{unlockErr: errors.New("unlock failed")}
	setTaskLockerFactory(t, func(ctx context.Context, key string, ttl time.Duration) taskLocker {
		return lock
	})

	Wrap("RankingSyncTask", Options{
		Backend:     backend,
		ServiceName: "personal_assistant",
		LockEnabled: true,
		LockKey:     "task:cron:RankingSyncTask",
		LockTTL:     30 * time.Second,
	}, func(ctx context.Context) error {
		return nil
	})()

	if len(backend.spans) != 1 {
		t.Fatalf("expected one span, got %d", len(backend.spans))
	}
	span := backend.spans[0]
	if span.Status != obstrace.SpanStatusError || span.ErrorCode != "task_lock_release_error" {
		t.Fatalf("unexpected unlock error span: status=%s code=%s", span.Status, span.ErrorCode)
	}
}

func setTaskLockerFactory(
	t *testing.T,
	factory func(ctx context.Context, key string, ttl time.Duration) taskLocker,
) {
	t.Helper()

	oldFactory := newTaskLocker
	newTaskLocker = factory
	t.Cleanup(func() {
		newTaskLocker = oldFactory
	})
}
