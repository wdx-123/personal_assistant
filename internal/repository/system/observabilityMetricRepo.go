package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type observabilityMetricRepository struct {
	db *gorm.DB
}

func NewObservabilityMetricRepository(db *gorm.DB) interfaces.ObservabilityMetricRepository {
	return &observabilityMetricRepository{db: db}
}

func (r *observabilityMetricRepository) IncrementBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error {
	for _, row := range rows {
		if row == nil {
			continue
		}
		err := r.db.WithContext(ctx).Exec(`
INSERT INTO observability_metrics (
	granularity, bucket_start, service, route_template, method, status_class, error_code,
	request_count, error_count, total_latency_ms, max_latency_ms,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
ON DUPLICATE KEY UPDATE
	request_count = request_count + VALUES(request_count),
	error_count = error_count + VALUES(error_count),
	total_latency_ms = total_latency_ms + VALUES(total_latency_ms),
	max_latency_ms = GREATEST(max_latency_ms, VALUES(max_latency_ms)),
	updated_at = NOW()
`,
			row.Granularity,
			row.BucketStart,
			row.Service,
			row.RouteTemplate,
			row.Method,
			row.StatusClass,
			row.ErrorCode,
			row.RequestCount,
			row.ErrorCount,
			row.TotalLatencyMs,
			row.MaxLatencyMs,
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *observabilityMetricRepository) UpsertAbsoluteBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error {
	for _, row := range rows {
		if row == nil {
			continue
		}
		err := r.db.WithContext(ctx).Exec(`
INSERT INTO observability_metrics (
	granularity, bucket_start, service, route_template, method, status_class, error_code,
	request_count, error_count, total_latency_ms, max_latency_ms,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
ON DUPLICATE KEY UPDATE
	request_count = VALUES(request_count),
	error_count = VALUES(error_count),
	total_latency_ms = VALUES(total_latency_ms),
	max_latency_ms = VALUES(max_latency_ms),
	updated_at = NOW()
`,
			row.Granularity,
			row.BucketStart,
			row.Service,
			row.RouteTemplate,
			row.Method,
			row.StatusClass,
			row.ErrorCode,
			row.RequestCount,
			row.ErrorCount,
			row.TotalLatencyMs,
			row.MaxLatencyMs,
		).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *observabilityMetricRepository) Query(
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
) ([]*entity.ObservabilityMetric, error) {
	var rows []*entity.ObservabilityMetric
	query := r.db.WithContext(ctx).Model(&entity.ObservabilityMetric{}).
		Where("granularity = ?", granularity).
		Where("bucket_start >= ? AND bucket_start < ?", start, end)

	if strings.TrimSpace(service) != "" {
		query = query.Where("service = ?", strings.TrimSpace(service))
	}
	if strings.TrimSpace(routeTemplate) != "" {
		query = query.Where("route_template = ?", strings.TrimSpace(routeTemplate))
	}
	if strings.TrimSpace(method) != "" {
		query = query.Where("method = ?", strings.ToUpper(strings.TrimSpace(method)))
	}
	if statusClass > 0 {
		query = query.Where("status_class = ?", statusClass)
	}
	if errorCode != nil {
		query = query.Where("error_code = ?", strings.TrimSpace(*errorCode))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Order("bucket_start ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *observabilityMetricRepository) Aggregate(
	ctx context.Context,
	fromGranularity string,
	toGranularity string,
	start time.Time,
	end time.Time,
) ([]*entity.ObservabilityMetric, error) {
	bucketExpr := aggregateBucketExpr(toGranularity)
	if bucketExpr == "" {
		return nil, fmt.Errorf("unsupported aggregate granularity: %s", toGranularity)
	}

	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("SELECT ? AS granularity, ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(` AS bucket_start,
	service,
	route_template,
	method,
	status_class,
	error_code,
	SUM(request_count) AS request_count,
	SUM(error_count) AS error_count,
	SUM(total_latency_ms) AS total_latency_ms,
	MAX(max_latency_ms) AS max_latency_ms
FROM observability_metrics
WHERE granularity = ? AND bucket_start < ?`)

	args := []interface{}{toGranularity, fromGranularity, end}
	if !start.IsZero() {
		sqlBuilder.WriteString(" AND bucket_start >= ?")
		args = append(args, start)
	}
	sqlBuilder.WriteString(" GROUP BY ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(", service, route_template, method, status_class, error_code")

	var rows []*entity.ObservabilityMetric
	if err := r.db.WithContext(ctx).Raw(sqlBuilder.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *observabilityMetricRepository) DeleteBeforeByGranularity(
	ctx context.Context,
	granularity string,
	before time.Time,
) error {
	granularity = strings.TrimSpace(granularity)
	if granularity == "" || before.IsZero() {
		return nil
	}

	// 使用默认作用域先筛出“仍活跃”的目标行，再按 ID 分批 Unscoped 物理删除，
	// 避免大表保留清理落成单次长事务，同时不误删历史已软删除数据。
	for {
		var ids []uint
		if err := r.db.WithContext(ctx).
			Model(&entity.ObservabilityMetric{}).
			Where("granularity = ? AND bucket_start < ?", granularity, before).
			Order("bucket_start ASC").
			Order("id ASC").
			Limit(observabilityDeleteBatchSize).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := r.db.WithContext(ctx).
			Unscoped().
			Where("id IN ?", ids).
			Delete(&entity.ObservabilityMetric{}).Error; err != nil {
			return err
		}
	}
}

func aggregateBucketExpr(granularity string) string {
	switch granularity {
	case "1d":
		return "DATE(bucket_start)"
	case "1w":
		return "DATE_SUB(DATE(bucket_start), INTERVAL WEEKDAY(bucket_start) DAY)"
	default:
		return ""
	}
}
