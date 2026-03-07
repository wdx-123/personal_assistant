package query

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	obsmetrics "personal_assistant/pkg/observability/metrics"
	obstrace "personal_assistant/pkg/observability/trace"
)

// Service 提供可观测性查询相关应用服务。
type Service struct {
	metricsBackend    obsmetrics.MetricsBackend
	traceBackend      obstrace.TraceBackend
	traceRepository   interfaces.ObservabilityTraceRepository
	runtimeRepository interfaces.ObservabilityRuntimeRepository
}

func NewQueryService(
	metricsBackend obsmetrics.MetricsBackend,
	traceBackend obstrace.TraceBackend,
	traceRepository interfaces.ObservabilityTraceRepository,
	runtimeRepository interfaces.ObservabilityRuntimeRepository,
) *Service {
	return &Service{
		metricsBackend:    metricsBackend,
		traceBackend:      traceBackend,
		traceRepository:   traceRepository,
		runtimeRepository: runtimeRepository,
	}
}

func (s *Service) QueryMetrics(
	ctx context.Context,
	req *request.ObservabilityMetricsQueryReq,
) (*resp.ObservabilityMetricsQueryResp, error) {
	if req == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "请求参数不能为空")
	}
	if s.metricsBackend == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "观测指标后端未初始化")
	}

	granularity := strings.TrimSpace(req.Granularity)
	if granularity != "1m" && granularity != "5m" && granularity != "1d" && granularity != "1w" {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "granularity 仅支持 1m/5m/1d/1w")
	}

	start, err := time.Parse(time.RFC3339, strings.TrimSpace(req.StartAt))
	if err != nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "start_at 需为 RFC3339 时间")
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(req.EndAt))
	if err != nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 需为 RFC3339 时间")
	}
	if !end.After(start) {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 必须大于 start_at")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 5000
	}
	if limit > 50000 {
		limit = 50000
	}

	rows, queryErr := s.metricsBackend.QueryMetrics(
		ctx,
		granularity,
		start.UTC(),
		end.UTC(),
		strings.TrimSpace(req.Service),
		strings.TrimSpace(req.RouteTemplate),
		strings.TrimSpace(req.Method),
		req.StatusClass,
		req.ErrorCode,
		limit,
	)
	if queryErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
	}

	list := make([]*resp.ObservabilityMetricPointResp, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		list = append(list, &resp.ObservabilityMetricPointResp{
			Granularity:    row.Granularity,
			BucketStart:    row.BucketStart.UTC().Format(time.RFC3339),
			Service:        row.Service,
			RouteTemplate:  row.RouteTemplate,
			Method:         row.Method,
			StatusClass:    row.StatusClass,
			ErrorCode:      row.ErrorCode,
			RequestCount:   row.RequestCount,
			ErrorCount:     row.ErrorCount,
			TotalLatencyMs: row.TotalLatencyMs,
			MaxLatencyMs:   row.MaxLatencyMs,
		})
	}

	return &resp.ObservabilityMetricsQueryResp{
		Granularity: granularity,
		List:        list,
	}, nil
}

func (s *Service) QueryTraceDetail(
	ctx context.Context,
	id string,
	idType string,
	limit int,
	offset int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "id 不能为空")
	}
	idType = request.NormalizeTraceDetailIDType(idType)
	if !request.IsValidTraceDetailIDType(idType) {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "id_type 仅支持 trace/request")
	}
	if s.traceBackend == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "追踪后端未初始化")
	}

	limit, offset = normalizeTracePage(limit, offset)
	var (
		spans []*obstrace.Span
		total int64
		err   error
	)
	switch idType {
	case request.TraceDetailIDTypeTrace:
		spans, total, err = s.traceBackend.ListByTraceID(
			ctx,
			id,
			limit,
			offset,
			includePayload,
			includeErrorDetail,
		)
	case request.TraceDetailIDTypeRequest:
		spans, total, err = s.traceBackend.ListByRequestID(
			ctx,
			id,
			limit,
			offset,
			includePayload,
			includeErrorDetail,
		)
	}
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return toTraceDetailQueryResp(spans, total, includePayload, includeErrorDetail), nil
}

func (s *Service) QueryTrace(
	ctx context.Context,
	req *request.ObservabilityTraceQueryReq,
) (*resp.ObservabilityTraceSummaryQueryResp, error) {
	if req == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "请求参数不能为空")
	}
	if s.traceRepository == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "追踪仓储未初始化")
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && status != obstrace.SpanStatusOK && status != obstrace.SpanStatusError {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "status 仅支持 ok/error")
	}
	rootStage := request.NormalizeTraceRootStage(req.RootStage)
	if rootStage == "" {
		rootStage = request.TraceRootStageHTTP
	}
	if !request.IsValidTraceRootStage(rootStage) {
		return nil, bizerrors.NewWithMsg(
			bizerrors.CodeInvalidParams,
			"root_stage 仅支持 http.request/task/all",
		)
	}
	start, err := parseTraceTime(req.StartAt)
	if err != nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "start_at 需为 RFC3339 时间")
	}
	end, err := parseTraceTime(req.EndAt)
	if err != nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 需为 RFC3339 时间")
	}
	if !start.IsZero() && !end.IsZero() && !end.After(start) {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 必须大于 start_at")
	}

	limit, offset := normalizeTracePage(req.Limit, req.Offset)
	rows, total, queryErr := s.traceRepository.QueryRootSummaries(ctx, &interfaces.ObservabilityTraceRootSummaryQuery{
		TraceID:   strings.TrimSpace(req.TraceID),
		RequestID: strings.TrimSpace(req.RequestID),
		Service:   strings.TrimSpace(req.Service),
		Status:    status,
		RootStage: rootStage,
		StartAt:   start.UTC(),
		EndAt:     end.UTC(),
		Limit:     limit,
		Offset:    offset,
	})
	if queryErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
	}
	return toTraceSummaryQueryResp(rows, total), nil
}

// QueryRuntimeMetrics 提供运行时
//
// 指标查询接口
// 支持任务执行总数
// 任务执行耗时
// 事件消费总数
// 事件消费耗时
//
// 等多种指标类型，满足不同维度的观测需求。
func (s *Service) QueryRuntimeMetrics(
	ctx context.Context,
	req *request.ObservabilityRuntimeMetricQueryReq,
) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
	if req == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "请求参数不能为空")
	}
	if s.runtimeRepository == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "运行时观测仓储未初始化")
	}

	metric := strings.TrimSpace(req.Metric)
	switch metric {
	case "task_execution_total":
		// 任务执行总数查询，包含成功/失败/跳过等状态，支持按任务名称和状态过滤
		start, end, err := parseRequiredRuntimeRange(req.StartAt, req.EndAt)
		if err != nil {
			return nil, err
		}
		granularity, err := normalizeRuntimeGranularity(req.Granularity)
		if err != nil {
			return nil, err
		}
		status, err := normalizeTaskRuntimeStatus(req.Status)
		if err != nil {
			return nil, err
		}
		rows, queryErr := s.runtimeRepository.QueryTaskExecutionSeries(ctx, &interfaces.RuntimeTaskQuery{
			TaskName:    strings.TrimSpace(req.TaskName),
			Status:      status,
			StartAt:     start,
			EndAt:       end,
			Granularity: granularity,
			Limit:       normalizeRuntimeLimit(req.Limit),
		})
		if queryErr != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
		}

		// 任务执行总数查询不包含耗时统计信息，因此传入 nil 构建响应
		return buildRuntimeSeriesResp(metric, rows, nil), nil
	case "task_duration_seconds":
		// 任务执行耗时查询，包含平均耗时、最大耗时和百分位数等统计信息，支持按任务名称和状态过滤
		start, end, err := parseRequiredRuntimeRange(req.StartAt, req.EndAt)
		if err != nil {
			return nil, err
		}
		if end.Sub(start) > 7*24*time.Hour {
			return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "duration 查询时间范围不能超过 7 天")
		}
		granularity, err := normalizeRuntimeGranularity(req.Granularity)
		if err != nil {
			return nil, err
		}
		status, err := normalizeTaskRuntimeStatus(req.Status)
		if err != nil {
			return nil, err
		}
		query := &interfaces.RuntimeTaskQuery{
			TaskName:    strings.TrimSpace(req.TaskName),
			Status:      status,
			StartAt:     start,
			EndAt:       end,
			Granularity: granularity,
			Limit:       normalizeRuntimeLimit(req.Limit),
		}
		rows, queryErr := s.runtimeRepository.QueryTaskDurationSeries(ctx, query)
		if queryErr != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
		}
		durations, listErr := s.runtimeRepository.ListTaskDurations(ctx, query)
		if listErr != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, listErr)
		}
		return buildRuntimeSeriesResp(metric, rows, durations), nil
	case "outbox_publish_duration_seconds":
		// Outbox 消息发布耗时查询，包含平均耗时、最大耗时和百分位数等统计信息，支持按状态过滤
		return s.queryRuntimeEventDuration(ctx, metric, req, true)
	case "event_consume_duration_seconds":
		// 事件消费耗时查询，包含平均耗时、最大耗时和百分位数等统计信息，支持按状态过滤
		return s.queryRuntimeEventDuration(ctx, metric, req, false)
	case "event_consume_total":
		// 事件消费总数查询，包含成功/失败等状态，支持按状态过滤
		return s.queryRuntimeEventCount(ctx, metric, req, false)
	case "outbox_events_total":
		// Outbox 消息发布总数查询，包含成功/失败等状态，支持按状态过滤
		status, err := normalizeOutboxRuntimeStatus(req.Status)
		if err != nil {
			return nil, err
		}
		if status == "" || status == entity.OutboxEventStatusPending {
			snapshot, snapshotErr := s.runtimeRepository.GetOutboxStatusSnapshot(ctx)
			if snapshotErr != nil {
				return nil, bizerrors.Wrap(bizerrors.CodeDBError, snapshotErr)
			}
			return &resp.ObservabilityRuntimeMetricQueryResp{
				Metric: metric,
				Mode:   "snapshot",
				List:   []*resp.ObservabilityRuntimeMetricPointResp{},
				Summary: &resp.ObservabilityRuntimeMetricSummaryResp{
					PendingTotal:   snapshot.Pending,
					PublishedTotal: snapshot.Published,
					FailedTotal:    snapshot.Failed,
					SnapshotAt:     snapshot.SnapshotAt.UTC().Format(time.RFC3339),
				},
			}, nil
		}
		start, end, rangeErr := parseRequiredRuntimeRange(req.StartAt, req.EndAt)
		if rangeErr != nil {
			return nil, rangeErr
		}
		granularity, granularityErr := normalizeRuntimeGranularity(req.Granularity)
		if granularityErr != nil {
			return nil, granularityErr
		}
		rows, queryErr := s.runtimeRepository.QueryOutboxStatusSeries(ctx, &interfaces.RuntimeOutboxQuery{
			Status:      status,
			StartAt:     start,
			EndAt:       end,
			Granularity: granularity,
			Limit:       normalizeRuntimeLimit(req.Limit),
		})
		if queryErr != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
		}
		return buildRuntimeSeriesResp(metric, rows, nil), nil
	default:
		return nil, bizerrors.NewWithMsg(
			bizerrors.CodeInvalidParams,
			"metric 仅支持 task_execution_total/task_duration_seconds/outbox_events_total/outbox_publish_duration_seconds/event_consume_total/event_consume_duration_seconds",
		)
	}
}

func parseTraceTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func parseRequiredRuntimeRange(startAt, endAt string) (time.Time, time.Time, error) {
	start, err := parseTraceTime(startAt)
	if err != nil {
		return time.Time{}, time.Time{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "start_at 需为 RFC3339 时间")
	}
	end, err := parseTraceTime(endAt)
	if err != nil {
		return time.Time{}, time.Time{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 需为 RFC3339 时间")
	}
	if start.IsZero() || end.IsZero() {
		return time.Time{}, time.Time{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "start_at/end_at 不能为空")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "end_at 必须大于 start_at")
	}
	return start.UTC(), end.UTC(), nil
}

func normalizeTracePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func normalizeRuntimeGranularity(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "", "5m":
		return "5m", nil
	case "1m", "1h", "1d":
		return strings.TrimSpace(raw), nil
	default:
		return "", bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "granularity 仅支持 1m/5m/1h/1d")
	}
}

func normalizeRuntimeLimit(limit int) int {
	if limit <= 0 {
		return 500
	}
	if limit > 2000 {
		return 2000
	}
	return limit
}

func normalizeTaskRuntimeStatus(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", "success", "error", "skipped":
		return raw, nil
	default:
		return "", bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "status 仅支持 success/error/skipped")
	}
}

func normalizeEventRuntimeStatus(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", "success", "error":
		return raw, nil
	default:
		return "", bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "status 仅支持 success/error")
	}
}

func normalizeOutboxRuntimeStatus(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", entity.OutboxEventStatusPending, entity.OutboxEventStatusPublished, entity.OutboxEventStatusFailed:
		return raw, nil
	default:
		return "", bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "status 仅支持 pending/published/failed")
	}
}

func toTraceDetailQueryResp(
	spans []*obstrace.Span,
	total int64,
	includePayload bool,
	includeErrorDetail bool,
) *resp.ObservabilityTraceQueryResp {
	list := make([]*resp.ObservabilityTraceSpanResp, 0, len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		item := &resp.ObservabilityTraceSpanResp{
			SpanID:       span.SpanID,
			ParentSpanID: span.ParentSpanID,
			TraceID:      span.TraceID,
			RequestID:    span.RequestID,
			Service:      span.Service,
			Stage:        span.Stage,
			Name:         span.Name,
			Kind:         span.Kind,
			Status:       span.Status,
			StartAt:      span.StartAt.UTC().Format(time.RFC3339),
			EndAt:        span.EndAt.UTC().Format(time.RFC3339),
			DurationMs:   span.DurationMs,
			ErrorCode:    span.ErrorCode,
			Message:      span.Message,
			Tags:         span.Tags,
		}
		if includePayload {
			item.RequestSnippet = span.RequestSnippet
			item.ResponseSnippet = span.ResponseSnippet
		}
		if includeErrorDetail {
			item.ErrorStack = span.ErrorStack
			item.ErrorDetailJSON = span.ErrorDetailJSON
		}
		list = append(list, item)
	}
	return &resp.ObservabilityTraceQueryResp{
		List:  list,
		Total: total,
	}
}

func toTraceSummaryQueryResp(
	rows []*interfaces.ObservabilityTraceRootSummary,
	total int64,
) *resp.ObservabilityTraceSummaryQueryResp {
	list := make([]*resp.ObservabilityTraceSummaryResp, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		method, routeTemplate := parseSummaryTags(row.TagsJSON)
		list = append(list, &resp.ObservabilityTraceSummaryResp{
			TraceID:        row.TraceID,
			RequestID:      row.RequestID,
			Service:        row.Service,
			Stage:          row.Stage,
			Name:           row.Name,
			Kind:           row.Kind,
			Status:         row.Status,
			ErrorCode:      row.ErrorCode,
			Message:        row.Message,
			StartAt:        row.StartAt.UTC().Format(time.RFC3339),
			EndAt:          row.EndAt.UTC().Format(time.RFC3339),
			DurationMs:     row.DurationMs,
			SpanTotal:      row.SpanTotal,
			ErrorSpanTotal: row.ErrorSpanTotal,
			Method:         method,
			RouteTemplate:  routeTemplate,
		})
	}
	return &resp.ObservabilityTraceSummaryQueryResp{
		List:  list,
		Total: total,
	}
}

// parseSummaryTags 从 JSON 格式的标签中解析出 method 和 route_template，
// 适用于 TraceSummary 中的 TagsJSON 字段。
func (s *Service) queryRuntimeEventCount(
	ctx context.Context,
	metric string,
	req *request.ObservabilityRuntimeMetricQueryReq,
	isPublish bool,
) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
	start, end, err := parseRequiredRuntimeRange(req.StartAt, req.EndAt)
	if err != nil {
		return nil, err
	}
	granularity, err := normalizeRuntimeGranularity(req.Granularity)
	if err != nil {
		return nil, err
	}
	status, err := normalizeEventRuntimeStatus(req.Status)
	if err != nil {
		return nil, err
	}
	query := &interfaces.RuntimeEventQuery{
		Topic:       strings.TrimSpace(req.Topic),
		Status:      status,
		StartAt:     start,
		EndAt:       end,
		Granularity: granularity,
		Limit:       normalizeRuntimeLimit(req.Limit),
	}

	var (
		rows     []*interfaces.RuntimeSeriesPoint
		queryErr error
	)
	if isPublish {
		rows, queryErr = s.runtimeRepository.QueryPublishSeries(ctx, query)
	} else {
		rows, queryErr = s.runtimeRepository.QueryConsumeSeries(ctx, query)
	}
	if queryErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
	}
	return buildRuntimeSeriesResp(metric, rows, nil), nil
}

// queryRuntimeEventDuration 查询事件耗时系列数据，包含平均耗时、最大耗时和百分位数等统计信息。
func (s *Service) queryRuntimeEventDuration(
	ctx context.Context,
	metric string,
	req *request.ObservabilityRuntimeMetricQueryReq,
	isPublish bool, // true 表示查询 Outbox 消息发布耗时，false 表示查询事件消费耗时
) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
	start, end, err := parseRequiredRuntimeRange(req.StartAt, req.EndAt)
	if err != nil {
		return nil, err
	}
	if end.Sub(start) > 7*24*time.Hour {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "duration 查询时间范围不能超过 7 天")
	}
	granularity, err := normalizeRuntimeGranularity(req.Granularity)
	if err != nil {
		return nil, err
	}
	status, err := normalizeEventRuntimeStatus(req.Status)
	if err != nil {
		return nil, err
	}
	query := &interfaces.RuntimeEventQuery{
		Topic:       strings.TrimSpace(req.Topic),
		Status:      status,
		StartAt:     start,
		EndAt:       end,
		Granularity: granularity,
		Limit:       normalizeRuntimeLimit(req.Limit),
	}

	var (
		rows      []*interfaces.RuntimeSeriesPoint
		durations []int64
		queryErr  error
		listErr   error
	)
	if isPublish {
		rows, queryErr = s.runtimeRepository.QueryPublishSeries(ctx, query)
		if queryErr == nil {
			durations, listErr = s.runtimeRepository.ListPublishDurations(ctx, query)
		}
	} else {
		rows, queryErr = s.runtimeRepository.QueryConsumeSeries(ctx, query)
		if queryErr == nil {
			durations, listErr = s.runtimeRepository.ListConsumeDurations(ctx, query)
		}
	}
	if queryErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
	}
	if listErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, listErr)
	}
	return buildRuntimeSeriesResp(metric, rows, durations), nil
}

// buildRuntimeSeriesResp 构建运行时指标系列查询响应，计算平均耗时和百分位数等统计数据。
func buildRuntimeSeriesResp(
	metric string,
	rows []*interfaces.RuntimeSeriesPoint,
	durations []int64,
) *resp.ObservabilityRuntimeMetricQueryResp {
	list := make([]*resp.ObservabilityRuntimeMetricPointResp, 0, len(rows))
	summary := &resp.ObservabilityRuntimeMetricSummaryResp{}
	for _, row := range rows {
		if row == nil {
			continue
		}
		avgDuration := int64(0)
		if row.Count > 0 && row.TotalDurationMs > 0 {
			avgDuration = row.TotalDurationMs / row.Count
		}
		list = append(list, &resp.ObservabilityRuntimeMetricPointResp{
			BucketStart:   row.BucketStart.UTC().Format(time.RFC3339),
			Status:        strings.TrimSpace(row.Status),
			Name:          strings.TrimSpace(row.Name),
			Topic:         strings.TrimSpace(row.Topic),
			Count:         row.Count,
			AvgDurationMs: avgDuration,
			MaxDurationMs: row.MaxDurationMs,
		})

		summary.Total += row.Count
		switch strings.TrimSpace(row.Status) {
		case "success":
			summary.SuccessTotal += row.Count
		case "error":
			summary.ErrorTotal += row.Count
		case "skipped":
			summary.SkippedTotal += row.Count
		case entity.OutboxEventStatusPending:
			summary.PendingTotal += row.Count
		case entity.OutboxEventStatusPublished:
			summary.PublishedTotal += row.Count
		case entity.OutboxEventStatusFailed:
			summary.FailedTotal += row.Count
		}
		if row.MaxDurationMs > summary.MaxDurationMs {
			summary.MaxDurationMs = row.MaxDurationMs
		}
	}

	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		summary.P50DurationMs = nearestRank(durations, 0.50)
		summary.P95DurationMs = nearestRank(durations, 0.95)
		summary.P99DurationMs = nearestRank(durations, 0.99)
		if durations[len(durations)-1] > summary.MaxDurationMs {
			summary.MaxDurationMs = durations[len(durations)-1]
		}
	}

	return &resp.ObservabilityRuntimeMetricQueryResp{
		Metric:  metric,
		Mode:    "series",
		List:    list,
		Summary: summary,
	}
}

// nearestRank 实现 Nearest Rank 百分位数算法，要求输入的 values 已经是升序排列的。
// 用于计算 P50/P95/P99 等百分位数，返回对应位置的值。
func nearestRank(values []int64, percentile float64) int64 {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(percentile*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func parseSummaryTags(tagsJSON string) (method string, routeTemplate string) {
	tagsJSON = strings.TrimSpace(tagsJSON)
	if tagsJSON == "" {
		return "", ""
	}
	tags := make(map[string]string)
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return "", ""
	}
	method = strings.TrimSpace(tags["method"])
	routeTemplate = strings.TrimSpace(tags["route_template"])
	if routeTemplate == "" {
		routeTemplate = strings.TrimSpace(tags["route"])
	}
	return method, routeTemplate
}

var _ contract.ObservabilityServiceContract = (*Service)(nil)
