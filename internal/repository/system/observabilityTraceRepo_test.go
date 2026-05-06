package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"personal_assistant/internal/model/entity"
)

func TestObservabilityTraceRepositoryDeleteBeforeByStatusHardDeletesExpiredRows(t *testing.T) {
	db := newObservabilityTraceRepositoryTestDB(t)
	repo := NewObservabilityTraceRepository(db)
	ctx := context.Background()
	cutoff := time.Date(2026, 4, 24, 2, 30, 0, 0, time.UTC)

	expiredOK := seedObservabilityTraceSpan(t, db, traceSeedOptions{
		status:  "ok",
		startAt: cutoff.Add(-2 * time.Hour),
		suffix:  "expired-ok",
	})
	freshOK := seedObservabilityTraceSpan(t, db, traceSeedOptions{
		status:  "ok",
		startAt: cutoff.Add(20 * time.Minute),
		suffix:  "fresh-ok",
	})
	expiredError := seedObservabilityTraceSpan(t, db, traceSeedOptions{
		status:  "error",
		startAt: cutoff.Add(-90 * time.Minute),
		suffix:  "expired-error",
	})
	expiredSoftDeletedOK := seedObservabilityTraceSpan(t, db, traceSeedOptions{
		status:  "ok",
		startAt: cutoff.Add(-3 * time.Hour),
		suffix:  "expired-soft-deleted-ok",
	})
	if err := db.Delete(expiredSoftDeletedOK).Error; err != nil {
		t.Fatalf("soft delete trace: %v", err)
	}

	if err := repo.DeleteBeforeByStatus(ctx, "ok", cutoff); err != nil {
		t.Fatalf("DeleteBeforeByStatus() error = %v", err)
	}

	assertTraceMissingUnscoped(t, db, expiredOK.ID)
	assertTraceExists(t, db, freshOK.ID)
	assertTraceExists(t, db, expiredError.ID)

	var softDeleted entity.ObservabilityTraceSpan
	if err := db.Unscoped().First(&softDeleted, expiredSoftDeletedOK.ID).Error; err != nil {
		t.Fatalf("load soft-deleted trace: %v", err)
	}
	if !softDeleted.DeletedAt.Valid {
		t.Fatalf("soft-deleted trace deleted_at valid = false, want true")
	}
}

func TestObservabilityTraceRepositoryDeleteBeforeByStatusDeletesInBatches(t *testing.T) {
	db := newObservabilityTraceRepositoryTestDB(t)
	repo := NewObservabilityTraceRepository(db)
	ctx := context.Background()
	cutoff := time.Date(2026, 4, 24, 2, 30, 0, 0, time.UTC)

	totalExpired := observabilityDeleteBatchSize + 25
	for i := 0; i < totalExpired; i++ {
		seedObservabilityTraceSpan(t, db, traceSeedOptions{
			status:  "error",
			startAt: cutoff.Add(-time.Duration(i+1) * time.Second),
			suffix:  fmt.Sprintf("expired-error-%d", i),
		})
	}
	freshError := seedObservabilityTraceSpan(t, db, traceSeedOptions{
		status:  "error",
		startAt: cutoff.Add(5 * time.Minute),
		suffix:  "fresh-error",
	})

	if err := repo.DeleteBeforeByStatus(ctx, "error", cutoff); err != nil {
		t.Fatalf("DeleteBeforeByStatus() error = %v", err)
	}

	var expiredRemaining int64
	if err := db.Model(&entity.ObservabilityTraceSpan{}).
		Where("status = ? AND start_at < ?", "error", cutoff).
		Count(&expiredRemaining).Error; err != nil {
		t.Fatalf("count expired traces: %v", err)
	}
	if expiredRemaining != 0 {
		t.Fatalf("expiredRemaining = %d, want 0", expiredRemaining)
	}
	assertTraceExists(t, db, freshError.ID)
}

type traceSeedOptions struct {
	status  string
	startAt time.Time
	suffix  string
}

func newObservabilityTraceRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.ObservabilityTraceSpan{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedObservabilityTraceSpan(t *testing.T, db *gorm.DB, opt traceSeedOptions) *entity.ObservabilityTraceSpan {
	t.Helper()

	row := &entity.ObservabilityTraceSpan{
		SpanID:          "span-" + opt.suffix,
		ParentSpanID:    "",
		TraceID:         "trace-" + opt.suffix,
		RequestID:       "request-" + opt.suffix,
		Service:         "personal_assistant",
		Stage:           "controller",
		Name:            "controller.test." + opt.suffix,
		Kind:            "internal",
		Status:          opt.status,
		StartAt:         opt.startAt,
		EndAt:           opt.startAt.Add(2 * time.Millisecond),
		DurationMs:      2,
		ErrorCode:       "",
		Message:         "",
		TagsJSON:        "{}",
		ErrorDetailJSON: "{}",
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create trace span: %v", err)
	}
	return row
}

func assertTraceExists(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()

	var row entity.ObservabilityTraceSpan
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("trace %d should exist: %v", id, err)
	}
}

func assertTraceMissingUnscoped(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()

	var row entity.ObservabilityTraceSpan
	err := db.Unscoped().First(&row, id).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("trace %d should be physically deleted, got err = %v", id, err)
	}
}
