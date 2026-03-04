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

func (t *tracedObservabilityService) QueryTraceByRequestID(
	ctx context.Context,
	requestID string,
	limit int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	return runTraced(ctx, "observability", "QueryTraceByRequestID", func(inner context.Context) (*resp.ObservabilityTraceQueryResp, error) {
		return t.next.QueryTraceByRequestID(inner, requestID, limit, includePayload, includeErrorDetail)
	})
}

func (t *tracedObservabilityService) QueryTraceByTraceID(
	ctx context.Context,
	traceID string,
	limit int,
	offset int,
	includePayload bool,
	includeErrorDetail bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	return runTraced(ctx, "observability", "QueryTraceByTraceID", func(inner context.Context) (*resp.ObservabilityTraceQueryResp, error) {
		return t.next.QueryTraceByTraceID(inner, traceID, limit, offset, includePayload, includeErrorDetail)
	})
}

func (t *tracedObservabilityService) QueryTrace(
	ctx context.Context,
	req *request.ObservabilityTraceQueryReq,
) (*resp.ObservabilityTraceQueryResp, error) {
	return runTraced(ctx, "observability", "QueryTrace", func(inner context.Context) (*resp.ObservabilityTraceQueryResp, error) {
		return t.next.QueryTrace(inner, req)
	})
}

var _ contract.ObservabilityServiceContract = (*tracedObservabilityService)(nil)
