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

func TestObservabilityMetricRepositoryDeleteBeforeByGranularityHardDeletesExpiredRows(t *testing.T) {
	db := newObservabilityRepositoryTestDB(t, &entity.ObservabilityMetric{})
	repo := NewObservabilityMetricRepository(db)
	ctx := context.Background()
	cutoff := time.Date(2026, 4, 24, 2, 10, 0, 0, time.UTC)

	expiredActive := seedObservabilityMetric(t, db, metricSeedOptions{
		granularity: "1m",
		bucketStart: cutoff.Add(-2 * time.Hour),
		suffix:      "expired-active",
	})
	freshActive := seedObservabilityMetric(t, db, metricSeedOptions{
		granularity: "1m",
		bucketStart: cutoff.Add(30 * time.Minute),
		suffix:      "fresh-active",
	})
	expiredSoftDeleted := seedObservabilityMetric(t, db, metricSeedOptions{
		granularity: "1m",
		bucketStart: cutoff.Add(-3 * time.Hour),
		suffix:      "expired-soft-deleted",
	})
	if err := db.Delete(expiredSoftDeleted).Error; err != nil {
		t.Fatalf("soft delete metric: %v", err)
	}

	if err := repo.DeleteBeforeByGranularity(ctx, "1m", cutoff); err != nil {
		t.Fatalf("DeleteBeforeByGranularity() error = %v", err)
	}

	assertMetricMissingUnscoped(t, db, expiredActive.ID)
	assertMetricExists(t, db, freshActive.ID)

	var softDeleted entity.ObservabilityMetric
	if err := db.Unscoped().First(&softDeleted, expiredSoftDeleted.ID).Error; err != nil {
		t.Fatalf("load soft-deleted metric: %v", err)
	}
	if !softDeleted.DeletedAt.Valid {
		t.Fatalf("soft-deleted metric deleted_at valid = false, want true")
	}
}

func TestObservabilityMetricRepositoryDeleteBeforeByGranularityDeletesInBatches(t *testing.T) {
	db := newObservabilityRepositoryTestDB(t, &entity.ObservabilityMetric{})
	repo := NewObservabilityMetricRepository(db)
	ctx := context.Background()
	cutoff := time.Date(2026, 4, 24, 2, 10, 0, 0, time.UTC)

	totalExpired := observabilityDeleteBatchSize + 25
	for i := 0; i < totalExpired; i++ {
		seedObservabilityMetric(t, db, metricSeedOptions{
			granularity: "5m",
			bucketStart: cutoff.Add(-time.Duration(i+1) * time.Minute),
			suffix:      fmt.Sprintf("expired-%d", i),
		})
	}
	freshActive := seedObservabilityMetric(t, db, metricSeedOptions{
		granularity: "5m",
		bucketStart: cutoff.Add(10 * time.Minute),
		suffix:      "fresh-active",
	})

	if err := repo.DeleteBeforeByGranularity(ctx, "5m", cutoff); err != nil {
		t.Fatalf("DeleteBeforeByGranularity() error = %v", err)
	}

	var expiredRemaining int64
	if err := db.Model(&entity.ObservabilityMetric{}).
		Where("granularity = ? AND bucket_start < ?", "5m", cutoff).
		Count(&expiredRemaining).Error; err != nil {
		t.Fatalf("count expired metrics: %v", err)
	}
	if expiredRemaining != 0 {
		t.Fatalf("expiredRemaining = %d, want 0", expiredRemaining)
	}
	assertMetricExists(t, db, freshActive.ID)
}

type metricSeedOptions struct {
	granularity string
	bucketStart time.Time
	suffix      string
}

func newObservabilityRepositoryTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedObservabilityMetric(t *testing.T, db *gorm.DB, opt metricSeedOptions) *entity.ObservabilityMetric {
	t.Helper()

	row := &entity.ObservabilityMetric{
		Granularity:    opt.granularity,
		BucketStart:    opt.bucketStart,
		Service:        "personal_assistant",
		RouteTemplate:  "/test/" + opt.suffix,
		Method:         "GET",
		StatusClass:    2,
		ErrorCode:      "",
		RequestCount:   1,
		ErrorCount:     0,
		TotalLatencyMs: 20,
		MaxLatencyMs:   20,
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create metric: %v", err)
	}
	return row
}

func assertMetricExists(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()

	var row entity.ObservabilityMetric
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("metric %d should exist: %v", id, err)
	}
}

func assertMetricMissingUnscoped(t *testing.T, db *gorm.DB, id uint) {
	t.Helper()

	var row entity.ObservabilityMetric
	err := db.Unscoped().First(&row, id).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("metric %d should be physically deleted, got err = %v", id, err)
	}
}
