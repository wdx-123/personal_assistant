package query

import (
	"context"
	"strings"
	"time"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	obsmetrics "personal_assistant/pkg/observability/metrics"
	obstrace "personal_assistant/pkg/observability/trace"
)

// Service 提供可观测性查询相关应用服务。
type Service struct {
	metricsBackend obsmetrics.MetricsBackend
	traceBackend   obstrace.TraceBackend
}

func NewQueryService(metricsBackend obsmetrics.MetricsBackend, traceBackend obstrace.TraceBackend) *Service {
	return &Service{
		metricsBackend: metricsBackend,
		traceBackend:   traceBackend,
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

func (s *Service) QueryTraceByRequestID(
	ctx context.Context,
	requestID string,
	limit int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "request_id 不能为空")
	}
	limit, offset := normalizeTracePage(limit, 0)
	if s.traceBackend == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "追踪后端未初始化")
	}
	spans, total, err := s.traceBackend.ListByRequestID(
		ctx,
		requestID,
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return toTraceQueryResp(spans, total, includePayload, includeErrorDetail), nil
}

func (s *Service) QueryTraceByTraceID(
	ctx context.Context,
	traceID string,
	limit int,
	offset int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "trace_id 不能为空")
	}
	if s.traceBackend == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "追踪后端未初始化")
	}
	limit, offset = normalizeTracePage(limit, offset)
	spans, total, err := s.traceBackend.ListByTraceID(
		ctx,
		traceID,
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return toTraceQueryResp(spans, total, includePayload, includeErrorDetail), nil
}

func (s *Service) QueryTrace(
	ctx context.Context,
	req *request.ObservabilityTraceQueryReq,
) (*resp.ObservabilityTraceQueryResp, error) {
	if req == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "请求参数不能为空")
	}
	if s.traceBackend == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "追踪后端未初始化")
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "" && status != obstrace.SpanStatusOK && status != obstrace.SpanStatusError {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "status 仅支持 ok/error")
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

	rows, total, queryErr := s.traceBackend.Query(ctx, &obstrace.Query{
		TraceID:            strings.TrimSpace(req.TraceID),
		RequestID:          strings.TrimSpace(req.RequestID),
		Service:            strings.TrimSpace(req.Service),
		Stage:              strings.TrimSpace(req.Stage),
		Status:             status,
		StartAt:            start.UTC(),
		EndAt:              end.UTC(),
		Limit:              limit,
		Offset:             offset,
		IncludePayload:     req.IncludePayload,
		IncludeErrorDetail: req.IncludeErrorDetail,
	})
	if queryErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, queryErr)
	}
	return toTraceQueryResp(rows, total, req.IncludePayload, req.IncludeErrorDetail), nil
}

func parseTraceTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, raw)
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

func toTraceQueryResp(
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

var _ contract.ObservabilityServiceContract = (*Service)(nil)
