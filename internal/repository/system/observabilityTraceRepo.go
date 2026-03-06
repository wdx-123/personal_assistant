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

const rootTraceStage = "http.request"

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

func (r *observabilityTraceRepository) QueryRootSummaries(
	ctx context.Context,
	q *interfaces.ObservabilityTraceRootSummaryQuery,
) ([]*interfaces.ObservabilityTraceRootSummary, int64, error) {
	if q == nil {
		q = &interfaces.ObservabilityTraceRootSummaryQuery{}
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

	rootBase := r.buildRootSummaryBaseQuery(ctx, q)
	groupedRoots := rootBase.
		Select("trace_id, request_id").
		Group("trace_id, request_id")

	var total int64
	if err := r.db.WithContext(ctx).
		Table("(?) AS grouped_roots", groupedRoots).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*interfaces.ObservabilityTraceRootSummary{}, 0, nil
	}

	latestStartPerGroup := rootBase.
		Select("trace_id, request_id, MAX(start_at) AS max_start_at").
		Group("trace_id, request_id")

	latestIDPerGroup := r.db.WithContext(ctx).
		Table("observability_trace_spans AS root").
		Where("root.deleted_at IS NULL").
		Select("root.trace_id, root.request_id, MAX(root.id) AS latest_id").
		Joins(
			"JOIN (?) AS latest_start ON latest_start.trace_id = root.trace_id AND latest_start.request_id = root.request_id AND latest_start.max_start_at = root.start_at",
			latestStartPerGroup,
		).
		Group("root.trace_id, root.request_id")

	spanStats := r.db.WithContext(ctx).
		Table("observability_trace_spans AS span").
		Where("span.deleted_at IS NULL").
		Select(
			"span.trace_id, span.request_id, COUNT(1) AS span_total, SUM(CASE WHEN span.status = ? THEN 1 ELSE 0 END) AS error_span_total",
			"error",
		).
		Group("span.trace_id, span.request_id")

	var rows []*interfaces.ObservabilityTraceRootSummary
	if err := r.db.WithContext(ctx).
		Table("observability_trace_spans AS root").
		Select(strings.Join([]string{
			"root.trace_id",
			"root.request_id",
			"root.service",
			"root.name",
			"root.status",
			"root.error_code",
			"root.message",
			"root.start_at",
			"root.end_at",
			"root.duration_ms",
			"root.tags_json",
			"COALESCE(stat.span_total, 0) AS span_total",
			"COALESCE(stat.error_span_total, 0) AS error_span_total",
		}, ", ")).
		Joins("JOIN (?) AS latest ON latest.latest_id = root.id", latestIDPerGroup).
		Joins("LEFT JOIN (?) AS stat ON stat.trace_id = root.trace_id AND stat.request_id = root.request_id", spanStats).
		Order("root.start_at DESC").
		Order("root.id DESC").
		Limit(q.Limit).
		Offset(q.Offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *observabilityTraceRepository) buildRootSummaryBaseQuery(
	ctx context.Context,
	q *interfaces.ObservabilityTraceRootSummaryQuery,
) *gorm.DB {
	base := r.db.WithContext(ctx).
		Table("observability_trace_spans").
		Where("deleted_at IS NULL").
		Where("stage = ?", rootTraceStage)

	if v := strings.TrimSpace(q.TraceID); v != "" {
		base = base.Where("trace_id = ?", v)
	}
	if v := strings.TrimSpace(q.RequestID); v != "" {
		base = base.Where("request_id = ?", v)
	}
	if v := strings.TrimSpace(q.Service); v != "" {
		base = base.Where("service = ?", v)
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
	return base
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
