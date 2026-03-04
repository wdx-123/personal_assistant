package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

// ObservabilityMetricRepository 指标聚合仓储
// 只负责 MySQL 读写，不承载业务编排
type ObservabilityMetricRepository interface {
	IncrementBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error
	UpsertAbsoluteBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error
	Query(
		ctx context.Context,
		granularity string,
		start time.Time,
		end time.Time,
		service string,
		routeTemplate string,
		method string,
		statusClass int,
		errorCode *string,
		limit int,
	) ([]*entity.ObservabilityMetric, error)
	Aggregate(
		ctx context.Context,
		fromGranularity string,
		toGranularity string,
		start time.Time,
		end time.Time,
	) ([]*entity.ObservabilityMetric, error)
	DeleteBeforeByGranularity(ctx context.Context, granularity string, before time.Time) error
}
