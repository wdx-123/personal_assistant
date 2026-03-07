package query

import (
	"context"
	"testing"
	"time"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

type stubTraceRepository struct {
	lastRootQuery *interfaces.ObservabilityTraceRootSummaryQuery
	rootRows      []*interfaces.ObservabilityTraceRootSummary
	rootTotal     int64
}

func (s *stubTraceRepository) BatchCreateIgnoreDup(ctx context.Context, rows []*entity.ObservabilityTraceSpan) error {
	return nil
}

func (s *stubTraceRepository) ListByRequestID(
	ctx context.Context,
	requestID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceRepository) ListByTraceID(
	ctx context.Context,
	traceID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceRepository) Query(
	ctx context.Context,
	q *interfaces.ObservabilityTraceQuery,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	return nil, 0, nil
}

func (s *stubTraceRepository) QueryRootSummaries(
	ctx context.Context,
	q *interfaces.ObservabilityTraceRootSummaryQuery,
) ([]*interfaces.ObservabilityTraceRootSummary, int64, error) {
	if q != nil {
		copyQuery := *q
		s.lastRootQuery = &copyQuery
	}
	return s.rootRows, s.rootTotal, nil
}

func (s *stubTraceRepository) DeleteBeforeByStatus(ctx context.Context, status string, before time.Time) error {
	return nil
}

type stubRuntimeRepository struct {
	lastTaskQuery    *interfaces.RuntimeTaskQuery
	lastPublishQuery *interfaces.RuntimeEventQuery
	lastConsumeQuery *interfaces.RuntimeEventQuery
	lastOutboxQuery  *interfaces.RuntimeOutboxQuery

	taskRows         []*interfaces.RuntimeSeriesPoint
	taskDurations    []int64
	publishRows      []*interfaces.RuntimeSeriesPoint
	publishDurations []int64
	consumeRows      []*interfaces.RuntimeSeriesPoint
	consumeDurations []int64
	outboxRows       []*interfaces.RuntimeSeriesPoint
	outboxSnapshot   *interfaces.RuntimeOutboxSnapshot
}

func (s *stubRuntimeRepository) QueryTaskExecutionSeries(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	if q != nil {
		copyQuery := *q
		s.lastTaskQuery = &copyQuery
	}
	return s.taskRows, nil
}

func (s *stubRuntimeRepository) QueryTaskDurationSeries(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	if q != nil {
		copyQuery := *q
		s.lastTaskQuery = &copyQuery
	}
	return s.taskRows, nil
}

func (s *stubRuntimeRepository) ListTaskDurations(
	ctx context.Context,
	q *interfaces.RuntimeTaskQuery,
) ([]int64, error) {
	if q != nil {
		copyQuery := *q
		s.lastTaskQuery = &copyQuery
	}
	return s.taskDurations, nil
}

func (s *stubRuntimeRepository) QueryPublishSeries(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	if q != nil {
		copyQuery := *q
		s.lastPublishQuery = &copyQuery
	}
	return s.publishRows, nil
}

func (s *stubRuntimeRepository) ListPublishDurations(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]int64, error) {
	if q != nil {
		copyQuery := *q
		s.lastPublishQuery = &copyQuery
	}
	return s.publishDurations, nil
}

func (s *stubRuntimeRepository) QueryConsumeSeries(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	if q != nil {
		copyQuery := *q
		s.lastConsumeQuery = &copyQuery
	}
	return s.consumeRows, nil
}

func (s *stubRuntimeRepository) ListConsumeDurations(
	ctx context.Context,
	q *interfaces.RuntimeEventQuery,
) ([]int64, error) {
	if q != nil {
		copyQuery := *q
		s.lastConsumeQuery = &copyQuery
	}
	return s.consumeDurations, nil
}

func (s *stubRuntimeRepository) QueryOutboxStatusSeries(
	ctx context.Context,
	q *interfaces.RuntimeOutboxQuery,
) ([]*interfaces.RuntimeSeriesPoint, error) {
	if q != nil {
		copyQuery := *q
		s.lastOutboxQuery = &copyQuery
	}
	return s.outboxRows, nil
}

func (s *stubRuntimeRepository) GetOutboxStatusSnapshot(
	ctx context.Context,
) (*interfaces.RuntimeOutboxSnapshot, error) {
	if s.outboxSnapshot == nil {
		return &interfaces.RuntimeOutboxSnapshot{}, nil
	}
	copySnapshot := *s.outboxSnapshot
	return &copySnapshot, nil
}

func TestQueryTrace_DefaultRootStageIsHTTP(t *testing.T) {
	repo := &stubTraceRepository{
		rootRows: []*interfaces.ObservabilityTraceRootSummary{
			{
				TraceID:        "trace-1",
				RequestID:      "request-1",
				Service:        "personal_assistant",
				Stage:          request.TraceRootStageHTTP,
				Name:           "GET /system/users",
				Kind:           "server",
				Status:         "ok",
				StartAt:        time.Unix(1700000000, 0).UTC(),
				EndAt:          time.Unix(1700000001, 0).UTC(),
				DurationMs:     1000,
				SpanTotal:      4,
				ErrorSpanTotal: 0,
				TagsJSON:       `{"method":"GET","route_template":"/system/users"}`,
			},
		},
		rootTotal: 1,
	}
	svc := NewQueryService(nil, nil, repo, nil)

	resp, err := svc.QueryTrace(context.Background(), &request.ObservabilityTraceQueryReq{})
	if err != nil {
		t.Fatalf("QueryTrace returned error: %v", err)
	}
	if repo.lastRootQuery == nil {
		t.Fatalf("expected root query to be recorded")
	}
	if repo.lastRootQuery.RootStage != request.TraceRootStageHTTP {
		t.Fatalf("expected default root stage %q, got %q", request.TraceRootStageHTTP, repo.lastRootQuery.RootStage)
	}
	if resp.Total != 1 || len(resp.List) != 1 {
		t.Fatalf("unexpected response size: total=%d len=%d", resp.Total, len(resp.List))
	}
	if resp.List[0].Stage != request.TraceRootStageHTTP {
		t.Fatalf("expected stage %q, got %q", request.TraceRootStageHTTP, resp.List[0].Stage)
	}
	if resp.List[0].Kind != "server" {
		t.Fatalf("expected kind server, got %q", resp.List[0].Kind)
	}
	if resp.List[0].Method != "GET" || resp.List[0].RouteTemplate != "/system/users" {
		t.Fatalf("unexpected method/route: %q %q", resp.List[0].Method, resp.List[0].RouteTemplate)
	}
}

func TestQueryTrace_UsesExplicitRootStage(t *testing.T) {
	repo := &stubTraceRepository{}
	svc := NewQueryService(nil, nil, repo, nil)

	_, err := svc.QueryTrace(context.Background(), &request.ObservabilityTraceQueryReq{
		RootStage: request.TraceRootStageTask,
	})
	if err != nil {
		t.Fatalf("QueryTrace returned error: %v", err)
	}
	if repo.lastRootQuery == nil {
		t.Fatalf("expected root query to be recorded")
	}
	if repo.lastRootQuery.RootStage != request.TraceRootStageTask {
		t.Fatalf("expected root stage %q, got %q", request.TraceRootStageTask, repo.lastRootQuery.RootStage)
	}
}

func TestQueryTrace_RejectsInvalidRootStage(t *testing.T) {
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, nil)

	_, err := svc.QueryTrace(context.Background(), &request.ObservabilityTraceQueryReq{
		RootStage: "consumer",
	})
	if err == nil {
		t.Fatalf("expected invalid root_stage to return error")
	}
}

func TestQueryRuntimeMetrics_TaskExecutionSeries(t *testing.T) {
	runtimeRepo := &stubRuntimeRepository{
		taskRows: []*interfaces.RuntimeSeriesPoint{
			{
				BucketStart: time.Unix(1700000000, 0).UTC(),
				Status:      "success",
				Name:        "RankingSyncTask",
				Count:       2,
			},
			{
				BucketStart: time.Unix(1700000000, 0).UTC(),
				Status:      "skipped",
				Name:        "RankingSyncTask",
				Count:       1,
			},
		},
	}
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, runtimeRepo)

	resp, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric:   "task_execution_total",
		StartAt:  "2024-01-01T00:00:00Z",
		EndAt:    "2024-01-01T01:00:00Z",
		TaskName: "RankingSyncTask",
	})
	if err != nil {
		t.Fatalf("QueryRuntimeMetrics returned error: %v", err)
	}
	if runtimeRepo.lastTaskQuery == nil {
		t.Fatalf("expected task query to be recorded")
	}
	if runtimeRepo.lastTaskQuery.Granularity != "5m" {
		t.Fatalf("expected default granularity 5m, got %q", runtimeRepo.lastTaskQuery.Granularity)
	}
	if resp.Mode != "series" || resp.Summary.Total != 3 {
		t.Fatalf("unexpected mode/total: mode=%s total=%d", resp.Mode, resp.Summary.Total)
	}
	if resp.Summary.SuccessTotal != 2 || resp.Summary.SkippedTotal != 1 {
		t.Fatalf("unexpected execution summary: %+v", resp.Summary)
	}
}

func TestQueryRuntimeMetrics_TaskDurationPercentiles(t *testing.T) {
	runtimeRepo := &stubRuntimeRepository{
		taskRows: []*interfaces.RuntimeSeriesPoint{
			{
				BucketStart:     time.Unix(1700000000, 0).UTC(),
				Status:          "success",
				Name:            "RankingSyncTask",
				Count:           3,
				TotalDurationMs: 600,
				MaxDurationMs:   300,
			},
		},
		taskDurations: []int64{100, 200, 300},
	}
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, runtimeRepo)

	resp, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric:   "task_duration_seconds",
		StartAt:  "2024-01-01T00:00:00Z",
		EndAt:    "2024-01-01T01:00:00Z",
		TaskName: "RankingSyncTask",
	})
	if err != nil {
		t.Fatalf("QueryRuntimeMetrics returned error: %v", err)
	}
	if len(resp.List) != 1 || resp.List[0].AvgDurationMs != 200 {
		t.Fatalf("unexpected duration list: %+v", resp.List)
	}
	if resp.Summary.P50DurationMs != 200 || resp.Summary.P95DurationMs != 300 || resp.Summary.P99DurationMs != 300 {
		t.Fatalf("unexpected percentile summary: %+v", resp.Summary)
	}
}

func TestQueryRuntimeMetrics_OutboxPendingSnapshot(t *testing.T) {
	snapshotAt := time.Unix(1700000200, 0).UTC()
	runtimeRepo := &stubRuntimeRepository{
		outboxSnapshot: &interfaces.RuntimeOutboxSnapshot{
			Pending:    5,
			Published:  8,
			Failed:     2,
			SnapshotAt: snapshotAt,
		},
	}
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, runtimeRepo)

	resp, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric: "outbox_events_total",
		Status: "pending",
	})
	if err != nil {
		t.Fatalf("QueryRuntimeMetrics returned error: %v", err)
	}
	if resp.Mode != "snapshot" {
		t.Fatalf("expected snapshot mode, got %q", resp.Mode)
	}
	if resp.Summary.PendingTotal != 5 || resp.Summary.PublishedTotal != 8 || resp.Summary.FailedTotal != 2 {
		t.Fatalf("unexpected snapshot summary: %+v", resp.Summary)
	}
	if resp.Summary.SnapshotAt != snapshotAt.Format(time.RFC3339) {
		t.Fatalf("unexpected snapshot time: %s", resp.Summary.SnapshotAt)
	}
}

func TestQueryRuntimeMetrics_OutboxPublishedSeries(t *testing.T) {
	runtimeRepo := &stubRuntimeRepository{
		outboxRows: []*interfaces.RuntimeSeriesPoint{
			{
				BucketStart: time.Unix(1700000000, 0).UTC(),
				Status:      entity.OutboxEventStatusPublished,
				Count:       4,
			},
		},
	}
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, runtimeRepo)

	resp, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric:  "outbox_events_total",
		Status:  entity.OutboxEventStatusPublished,
		StartAt: "2024-01-01T00:00:00Z",
		EndAt:   "2024-01-01T01:00:00Z",
	})
	if err != nil {
		t.Fatalf("QueryRuntimeMetrics returned error: %v", err)
	}
	if runtimeRepo.lastOutboxQuery == nil || runtimeRepo.lastOutboxQuery.Status != entity.OutboxEventStatusPublished {
		t.Fatalf("expected outbox series query to be recorded")
	}
	if resp.Mode != "series" || resp.Summary.PublishedTotal != 4 {
		t.Fatalf("unexpected outbox series response: %+v", resp.Summary)
	}
}

func TestQueryRuntimeMetrics_EventConsumeCount(t *testing.T) {
	runtimeRepo := &stubRuntimeRepository{
		consumeRows: []*interfaces.RuntimeSeriesPoint{
			{
				BucketStart: time.Unix(1700000000, 0).UTC(),
				Status:      "success",
				Topic:       "luogu.bind",
				Count:       6,
			},
		},
	}
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, runtimeRepo)

	resp, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric:  "event_consume_total",
		StartAt: "2024-01-01T00:00:00Z",
		EndAt:   "2024-01-01T01:00:00Z",
		Topic:   "luogu.bind",
		Status:  "success",
	})
	if err != nil {
		t.Fatalf("QueryRuntimeMetrics returned error: %v", err)
	}
	if runtimeRepo.lastConsumeQuery == nil || runtimeRepo.lastConsumeQuery.Topic != "luogu.bind" {
		t.Fatalf("expected consume query to be recorded")
	}
	if resp.Summary.SuccessTotal != 6 {
		t.Fatalf("unexpected consume summary: %+v", resp.Summary)
	}
}

func TestQueryRuntimeMetrics_RejectsInvalidGranularity(t *testing.T) {
	svc := NewQueryService(nil, nil, &stubTraceRepository{}, &stubRuntimeRepository{})

	_, err := svc.QueryRuntimeMetrics(context.Background(), &request.ObservabilityRuntimeMetricQueryReq{
		Metric:      "task_execution_total",
		StartAt:     "2024-01-01T00:00:00Z",
		EndAt:       "2024-01-01T01:00:00Z",
		Granularity: "10m",
	})
	if err == nil {
		t.Fatalf("expected invalid granularity to return error")
	}
}
