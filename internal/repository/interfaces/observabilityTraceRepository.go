package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

// ObservabilityTraceQuery 追踪查询条件
type ObservabilityTraceQuery struct {
	TraceID   string
	RequestID string
	Service   string
	Stage     string
	Status    string
	StartAt   time.Time
	EndAt     time.Time
	Limit     int
	Offset    int

	IncludePayload     bool
	IncludeErrorDetail bool
}

// ObservabilityTraceRepository 追踪 Span 仓储
type ObservabilityTraceRepository interface {
	BatchCreateIgnoreDup(ctx context.Context, rows []*entity.ObservabilityTraceSpan) error
	ListByRequestID(ctx context.Context, requestID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*entity.ObservabilityTraceSpan, int64, error)
	ListByTraceID(ctx context.Context, traceID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*entity.ObservabilityTraceSpan, int64, error)
	Query(ctx context.Context, q *ObservabilityTraceQuery) ([]*entity.ObservabilityTraceSpan, int64, error)
	DeleteBeforeByStatus(ctx context.Context, status string, before time.Time) error
}
