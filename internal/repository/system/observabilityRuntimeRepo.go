package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

// observabilityRuntimeRepository 运行时观测仓储实现
// 主要基于 observability_trace_spans 表进行聚合查询
type observabilityRuntimeRepository struct {
	db *gorm.DB
}

// NewObservabilityRuntimeRepository 创建运行时观测仓储实例
func NewObservabilityRuntimeRepository(db *gorm.DB) interfaces.ObservabilityRuntimeRepository {
	return &observabilityRuntimeRepository{db: db}
}

// QueryTaskExecutionSeries 查询后台任务执行次数的时序聚合
// 实现逻辑：
// 1. 根据 Granularity 将 start_at 对齐到时间桶 (Bucket)
// 2. 按 Bucket、Status、Name 进行分组 (GROUP BY)
// 3. 计算 COUNT(1) 作为执行次数，SUM(duration_ms) 作为总耗时
func (r *observabilityRuntimeRepository) QueryTaskExecutionSeries(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	return r.queryTaskSeries(ctx, q)
}

// QueryTaskDurationSeries 查询后台任务耗时的时序聚合
// 逻辑同 QueryTaskExecutionSeries，复用 queryTaskSeries
func (r *observabilityRuntimeRepository) QueryTaskDurationSeries(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	return r.queryTaskSeries(ctx, q)
}

// ListTaskDurations 获取指定时间范围内所有任务执行的原始耗时列表
// 用途：用于在 Service 层计算 P50/P95/P99 分位数
// 注意：未做 LIMIT 限制，调用方需自行控制时间范围，避免拉取过多数据
func (r *observabilityRuntimeRepository) ListTaskDurations(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]int64, error) {
	sqlBuilder := strings.Builder{}
	// 直接从 trace 表中拉取 duration_ms
	sqlBuilder.WriteString(`
SELECT duration_ms
FROM observability_trace_spans
WHERE deleted_at IS NULL
  AND stage = 'task'
  AND start_at >= ?
  AND start_at < ?`)

	args := []any{q.StartAt, q.EndAt}
	if taskName := strings.TrimSpace(q.TaskName); taskName != "" {
		sqlBuilder.WriteString(" AND name = ?")
		args = append(args, taskName)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		sqlBuilder.WriteString(" AND ")
		// 动态生成状态提取表达式（优先取 tags.execution_result，兜底取 status）
		sqlBuilder.WriteString(taskExecutionResultExpr())
		sqlBuilder.WriteString(" = ?")
		args = append(args, status)
	}
	sqlBuilder.WriteString(" ORDER BY duration_ms ASC")

	return r.queryDurations(ctx, sqlBuilder.String(), args...)
}

// QueryPublishSeries 查询消息发布（Redis Stream Publish）的时序聚合
// Stage 固定为 "event.publish"，Name 固定为 "redis.stream.publish"
func (r *observabilityRuntimeRepository) QueryPublishSeries(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	return r.queryEventSeries(ctx, q, "event.publish", "redis.stream.publish")
}

// ListPublishDurations 获取消息发布操作的原始耗时列表
func (r *observabilityRuntimeRepository) ListPublishDurations(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]int64, error) {
	return r.listEventDurations(ctx, q, "event.publish", "redis.stream.publish")
}

// QueryConsumeSeries 查询消息消费（Consumer）的时序聚合
// Stage 固定为 "consumer"，Name 固定为 "redis.stream.consume"
func (r *observabilityRuntimeRepository) QueryConsumeSeries(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	return r.queryEventSeries(ctx, q, "consumer", "redis.stream.consume")
}

// ListConsumeDurations 获取消息消费操作的原始耗时列表
func (r *observabilityRuntimeRepository) ListConsumeDurations(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]int64, error) {
	return r.listEventDurations(ctx, q, "consumer", "redis.stream.consume")
}

// QueryOutboxStatusSeries 查询 Outbox 表（outbox_events）的状态时序聚合
// 注意：数据源是 outbox_events 业务表，而非 trace 表
// 逻辑：
// 1. 根据状态选择时间列：failed -> updated_at, 其他 -> published_at
// 2. 按时间桶和状态分组统计
func (r *observabilityRuntimeRepository) QueryOutboxStatusSeries(
	ctx context.Context,
	q *interfaces.RuntimeOutboxQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	// 根据状态决定聚合的时间字段（失败看更新时间，成功看发布时间）
	timestampColumn := outboxSeriesTimeColumn(q.Status)
	bucketExpr, err := runtimeBucketExpr(timestampColumn, q.Granularity)
	if err != nil {
		return nil, err
	}

	// 构造聚合查询 SQL
	sql := fmt.Sprintf(`
SELECT %s AS bucket_start,
       status,
       '' AS name,
       '' AS topic,
       COUNT(1) AS count,
       0 AS total_duration_ms,
       0 AS max_duration_ms
FROM outbox_events
WHERE deleted_at IS NULL
  AND status = ?
  AND %s IS NOT NULL
  AND %s >= ?
  AND %s < ?
GROUP BY %s, status
ORDER BY bucket_start ASC
LIMIT ?`, bucketExpr, timestampColumn, timestampColumn, timestampColumn, bucketExpr)

	rows := make([]*interfaces.RuntimeSeriesPoint, 0)
	if err := r.db.WithContext(ctx).Raw(sql, q.Status, q.StartAt, q.EndAt, q.Limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetOutboxStatusSnapshot 获取 Outbox 表当前的实时状态快照
// 直接对全表进行 GROUP BY status 统计，不带时间范围过滤
func (r *observabilityRuntimeRepository) GetOutboxStatusSnapshot(
	ctx context.Context,
) (*interfaces.RuntimeOutboxSnapshot, error) {
	type statusRow struct {
		Status string
		Count  int64
	}

	var rows []*statusRow
	if err := r.db.WithContext(ctx).
		Table("outbox_events").
		Select("status, COUNT(1) AS count").
		Where("deleted_at IS NULL").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	snapshot := &interfaces.RuntimeOutboxSnapshot{
		SnapshotAt: time.Now().UTC(),
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		switch strings.TrimSpace(row.Status) {
		case "pending":
			snapshot.Pending = row.Count
		case "published":
			snapshot.Published = row.Count
		case "failed":
			snapshot.Failed = row.Count
		}
	}
	return snapshot, nil
}

// queryTaskSeries 内部复用逻辑：查询任务相关的聚合数据
// 获取任务执行总数和耗时的时序数据，支持按任务名称和状态过滤
func (r *observabilityRuntimeRepository) queryTaskSeries(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	// 生成时间桶表达式 (MySQL FROM_UNIXTIME ...)
	bucketExpr, err := runtimeBucketExpr("start_at", q.Granularity)
	if err != nil {
		return nil, err
	}

	statusExpr := taskExecutionResultExpr()
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(` AS bucket_start,
       `)
	sqlBuilder.WriteString(statusExpr)
	sqlBuilder.WriteString(` AS status,
       name,
       '' AS topic,
       COUNT(1) AS count,
       SUM(duration_ms) AS total_duration_ms,
       MAX(duration_ms) AS max_duration_ms
FROM observability_trace_spans
WHERE deleted_at IS NULL
  AND stage = 'task'
  AND start_at >= ?
  AND start_at < ?`)

	args := []any{q.StartAt, q.EndAt}
	if taskName := strings.TrimSpace(q.TaskName); taskName != "" {
		sqlBuilder.WriteString(" AND name = ?")
		args = append(args, taskName)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		sqlBuilder.WriteString(" AND ")
		sqlBuilder.WriteString(statusExpr)
		sqlBuilder.WriteString(" = ?")
		args = append(args, status)
	}
	sqlBuilder.WriteString(" GROUP BY ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(", ")
	sqlBuilder.WriteString(statusExpr)
	sqlBuilder.WriteString(", name ORDER BY bucket_start ASC LIMIT ?")
	args = append(args, q.Limit)

	rows := make([]*interfaces.RuntimeSeriesPoint, 0)
	if err := r.db.WithContext(ctx).Raw(sqlBuilder.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// queryEventSeries 内部复用逻辑：查询事件（发布/消费）相关的聚合数据
// 获取事件执行总数和耗时的时序数据，支持按消息 Topic 和状态过滤
func (r *observabilityRuntimeRepository) queryEventSeries(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
	stage string,
	spanName string,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	bucketExpr, err := runtimeBucketExpr("start_at", q.Granularity)
	if err != nil {
		return nil, err
	}

	statusExpr := spanOutcomeExpr()
	topicExpr := traceTopicExpr()
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString("SELECT ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(` AS bucket_start,
       `)
	sqlBuilder.WriteString(statusExpr)
	sqlBuilder.WriteString(` AS status,
       '' AS name,
       `)
	sqlBuilder.WriteString(topicExpr)
	sqlBuilder.WriteString(` AS topic,
       COUNT(1) AS count,
       SUM(duration_ms) AS total_duration_ms,
       MAX(duration_ms) AS max_duration_ms
FROM observability_trace_spans
WHERE deleted_at IS NULL
  AND stage = ?
  AND name = ?
  AND start_at >= ?
  AND start_at < ?`)

	args := []any{stage, spanName, q.StartAt, q.EndAt}
	if topic := strings.TrimSpace(q.Topic); topic != "" {
		sqlBuilder.WriteString(" AND ")
		sqlBuilder.WriteString(topicExpr)
		sqlBuilder.WriteString(" = ?")
		args = append(args, topic)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		sqlBuilder.WriteString(" AND ")
		sqlBuilder.WriteString(statusExpr)
		sqlBuilder.WriteString(" = ?")
		args = append(args, status)
	}
	sqlBuilder.WriteString(" GROUP BY ")
	sqlBuilder.WriteString(bucketExpr)
	sqlBuilder.WriteString(", ")
	sqlBuilder.WriteString(statusExpr)
	sqlBuilder.WriteString(", ")
	sqlBuilder.WriteString(topicExpr)
	sqlBuilder.WriteString(" ORDER BY bucket_start ASC LIMIT ?")
	args = append(args, q.Limit)

	rows := make([]*interfaces.RuntimeSeriesPoint, 0)
	if err := r.db.WithContext(ctx).Raw(sqlBuilder.String(), args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// listEventDurations 内部复用逻辑：获取事件耗时列表
func (r *observabilityRuntimeRepository) listEventDurations(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
	stage string,
	spanName string,
) ([]int64, error) {
	sqlBuilder := strings.Builder{}
	sqlBuilder.WriteString(`
SELECT duration_ms
FROM observability_trace_spans
WHERE deleted_at IS NULL
  AND stage = ?
  AND name = ?
  AND start_at >= ?
  AND start_at < ?`)

	args := []any{stage, spanName, q.StartAt, q.EndAt}
	if topic := strings.TrimSpace(q.Topic); topic != "" {
		sqlBuilder.WriteString(" AND ")
		sqlBuilder.WriteString(traceTopicExpr())
		sqlBuilder.WriteString(" = ?")
		args = append(args, topic)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		sqlBuilder.WriteString(" AND ")
		sqlBuilder.WriteString(spanOutcomeExpr())
		sqlBuilder.WriteString(" = ?")
		args = append(args, status)
	}
	sqlBuilder.WriteString(" ORDER BY duration_ms ASC")

	return r.queryDurations(ctx, sqlBuilder.String(), args...)
}

// queryDurations 通用辅助函数：执行 SQL 并返回 int64 数组
func (r *observabilityRuntimeRepository) queryDurations(
	ctx context.Context,
	sql string,
	args ...any,
) ([]int64, error) {
	type durationRow struct {
		DurationMs int64
	}

	var rows []*durationRow
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	values := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		values = append(values, row.DurationMs)
	}
	return values, nil
}

// runtimeBucketExpr 生成 MySQL 的时间桶分组表达式
// 示例 (1m): FROM_UNIXTIME(UNIX_TIMESTAMP(col) - MOD(UNIX_TIMESTAMP(col), 60))
func runtimeBucketExpr(column string, granularity string) (string, error) {
	seconds := 0
	switch strings.TrimSpace(granularity) {
	case "1m":
		seconds = 60
	case "5m":
		seconds = 300
	case "1h":
		seconds = 3600
	case "1d":
		seconds = 86400
	default:
		return "", fmt.Errorf("unsupported runtime granularity: %s", granularity)
	}

	return fmt.Sprintf(
		"FROM_UNIXTIME(UNIX_TIMESTAMP(%s) - MOD(UNIX_TIMESTAMP(%s), %d))",
		column,
		column,
		seconds,
	), nil
}

// taskExecutionResultExpr 提取任务执行结果表达式
// 优先从 tags_json->'$.execution_result' 取值，如果为空则降级使用 status 字段
func taskExecutionResultExpr() string {
	return "COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(tags_json, '$.execution_result')), ''), CASE WHEN status = 'error' THEN 'error' ELSE 'success' END)"
}

// spanOutcomeExpr 提取通用 Span 结果表达式
// 简单根据 status 字段判断 (error/ok)
func spanOutcomeExpr() string {
	return "CASE WHEN status = 'error' THEN 'error' ELSE 'success' END"
}

// traceTopicExpr 提取消息 Topic 表达式
// 从 tags_json->'$.topic' 获取
func traceTopicExpr() string {
	return "COALESCE(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(tags_json, '$.topic')), ''), '')"
}

// outboxSeriesTimeColumn 根据状态决定聚合使用的时间列
// failed -> 使用 updated_at (因为发布失败会更新该字段)
// 其他 -> 使用 published_at
func outboxSeriesTimeColumn(status string) string {
	if strings.TrimSpace(status) == "failed" {
		return "updated_at"
	}
	return "published_at"
}

var _ interfaces.ObservabilityRuntimeRepository = (*observabilityRuntimeRepository)(nil)
