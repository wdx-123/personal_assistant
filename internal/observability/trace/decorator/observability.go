package decorator

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/service/contract"
)

type tracedObservabilityService struct {
	next contract.ObservabilityServiceContract
}

func WrapObservabilityService(next contract.ObservabilityServiceContract) contract.ObservabilityServiceContract {
	if next == nil {
		return nil
	}
	return &tracedObservabilityService{next: next}
}

func (t *tracedObservabilityService) QueryMetrics(
	ctx context.Context,
	req *request.ObservabilityMetricsQueryReq,
) (*resp.ObservabilityMetricsQueryResp, error) {
	return runTraced(ctx, "observability", "QueryMetrics", func(inner context.Context) (*resp.ObservabilityMetricsQueryResp, error) {
		return t.next.QueryMetrics(inner, req)
	})
}

func (t *tracedObservabilityService) QueryRuntimeMetrics(
	ctx context.Context,
	req *request.ObservabilityRuntimeMetricQueryReq,
) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
	return runTraced(ctx, "observability", "QueryRuntimeMetrics", func(inner context.Context) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
		return t.next.QueryRuntimeMetrics(inner, req)
	})
}

func (t *tracedObservabilityService) QueryTraceDetail(
	ctx context.Context,
	id string,
	idType string,
	limit int,
	offset int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	return runTraced(ctx, "observability", "QueryTraceDetail", func(inner context.Context) (*resp.ObservabilityTraceQueryResp, error) {
		return t.next.QueryTraceDetail(inner, id, idType, limit, offset, includePayload, includeErrorDetail)
	})
}

func (t *tracedObservabilityService) QueryTrace(
	ctx context.Context,
	req *request.ObservabilityTraceQueryReq,
) (*resp.ObservabilityTraceSummaryQueryResp, error) {
	return runTraced(ctx, "observability", "QueryTrace", func(inner context.Context) (*resp.ObservabilityTraceSummaryQueryResp, error) {
		return t.next.QueryTrace(inner, req)
	})
}

var _ contract.ObservabilityServiceContract = (*tracedObservabilityService)(nil)
