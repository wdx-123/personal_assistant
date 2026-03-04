package system

import (
	"context"
	"strings"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type observabilityTraceRepository struct {
	db *gorm.DB
}

func NewObservabilityTraceRepository(db *gorm.DB) interfaces.ObservabilityTraceRepository {
	return &observabilityTraceRepository{db: db}
}

func (r *observabilityTraceRepository) BatchCreateIgnoreDup(
	ctx context.Context,
	rows []*entity.ObservabilityTraceSpan,
) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "span_id"}},
			DoNothing: true,
		}).
		CreateInBatches(rows, 200).Error
}

func (r *observabilityTraceRepository) ListByRequestID(
	ctx context.Context,
	requestID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	q := &interfaces.ObservabilityTraceQuery{
		RequestID:          strings.TrimSpace(requestID),
		Limit:              limit,
		Offset:             offset,
		IncludePayload:     includePayload,
		IncludeErrorDetail: includeErrorDetail,
	}
	return r.Query(ctx, q)
}

func (r *observabilityTraceRepository) ListByTraceID(
	ctx context.Context,
	traceID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	q := &interfaces.ObservabilityTraceQuery{
		TraceID:            strings.TrimSpace(traceID),
		Limit:              limit,
		Offset:             offset,
		IncludePayload:     includePayload,
		IncludeErrorDetail: includeErrorDetail,
	}
	return r.Query(ctx, q)
}

func (r *observabilityTraceRepository) Query(
	ctx context.Context,
	q *interfaces.ObservabilityTraceQuery,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	if q == nil {
		q = &interfaces.ObservabilityTraceQuery{}
	}
	if q.Limit <= 0 {
		q.Limit = 200
	}
	if q.Limit > 1000 {
		q.Limit = 1000
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	base := r.db.WithContext(ctx).Model(&entity.ObservabilityTraceSpan{})

	if v := strings.TrimSpace(q.TraceID); v != "" {
		base = base.Where("trace_id = ?", v)
	}
	if v := strings.TrimSpace(q.RequestID); v != "" {
		base = base.Where("request_id = ?", v)
	}
	if v := strings.TrimSpace(q.Service); v != "" {
		base = base.Where("service = ?", v)
	}
	if v := strings.TrimSpace(q.Stage); v != "" {
		base = base.Where("stage = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		base = base.Where("status = ?", v)
	}
	if !q.StartAt.IsZero() {
		base = base.Where("start_at >= ?", q.StartAt)
	}
	if !q.EndAt.IsZero() {
		base = base.Where("start_at < ?", q.EndAt)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := base
	if !q.IncludePayload || !q.IncludeErrorDetail {
		selectCols := []string{
			"id", "created_at", "updated_at", "deleted_at",
			"span_id", "parent_span_id", "trace_id", "request_id",
			"service", "stage", "name", "kind", "status",
			"start_at", "end_at", "duration_ms", "error_code",
			"message", "tags_json",
		}
		if q.IncludePayload {
			selectCols = append(selectCols, "request_snippet", "response_snippet")
		}
		if q.IncludeErrorDetail {
			selectCols = append(selectCols, "error_stack", "error_detail_json")
		}
		query = query.Select(selectCols)
	}

	var rows []*entity.ObservabilityTraceSpan
	if err := query.Order("start_at ASC").Limit(q.Limit).Offset(q.Offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *observabilityTraceRepository) DeleteBeforeByStatus(
	ctx context.Context,
	status string,
	before time.Time,
) error {
	status = strings.TrimSpace(status)
	if status == "" || before.IsZero() {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("status = ? AND start_at < ?", status, before).
		Delete(&entity.ObservabilityTraceSpan{}).Error
}
