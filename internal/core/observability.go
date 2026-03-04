package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	obsmetrics "personal_assistant/pkg/observability/metrics"
	obstrace "personal_assistant/pkg/observability/trace"
)

var observabilityInitOnce sync.Once

// InitObservability 初始化观测基础设施（幂等）
func InitObservability(
	ctx context.Context,
	metricRepo interfaces.ObservabilityMetricRepository,
	traceRepo interfaces.ObservabilityTraceRepository,
) error {
	var initErr error
	observabilityInitOnce.Do(func() {
		cfg := global.Config.Observability
		if !cfg.Enabled {
			return
		}
		if metricRepo == nil || traceRepo == nil {
			initErr = fmt.Errorf("observability repo is nil")
			return
		}

		metricsBackend := obsmetrics.NewBackend(metricRepo, global.Log, obsmetrics.Options{
			Enabled:           cfg.Enabled,
			ServiceName:       cfg.ServiceName,
			FlushInterval:     time.Duration(cfg.Metrics.FlushIntervalMs) * time.Millisecond,
			DBBatchSize:       cfg.Metrics.DBBatchSize,
			FineRetentionDays: cfg.Metrics.FineRetentionDays,
			DayRetentionDays:  cfg.Metrics.DayRetentionDays,
			WeekRetentionDays: cfg.Metrics.WeekRetentionDays,
		})
		metricsBackend.Start(ctx)

		traceBackend := obstrace.NewBackend(
			global.Redis,
			&observabilityTraceStoreAdapter{repo: traceRepo},
			global.Log,
			obstrace.Options{
				Enabled: cfg.Enabled && cfg.Traces.Enabled,

				ServiceName: cfg.ServiceName,

				StreamKey:       cfg.Traces.StreamKey,
				StreamGroup:     cfg.Traces.StreamGroup,
				StreamConsumer:  cfg.Traces.StreamConsumer,
				StreamReadCount: int64(cfg.Traces.StreamReadCount),
				StreamBlock:     time.Duration(cfg.Traces.StreamBlockMs) * time.Millisecond,
				PendingIdle:     time.Duration(cfg.Traces.PendingIdleMs) * time.Millisecond,

				DBBatchSize:     cfg.Traces.DBBatchSize,
				DBFlushInterval: time.Duration(cfg.Traces.DBFlushIntervalMs) * time.Millisecond,

				NormalQueueSize:   cfg.Traces.NormalQueueSize,
				CriticalQueueSize: cfg.Traces.CriticalQueueSize,
				EnqueueTimeout:    time.Duration(cfg.Traces.EnqueueTimeoutMs) * time.Millisecond,

				SuccessSampleRate:     cfg.Traces.SuccessSampleRate,
				DropSuccessOnOverload: cfg.Traces.DropSuccessOnOverload,
				CaptureErrorPayload:   cfg.Traces.CaptureErrorPayload,
				CaptureErrorStack:     cfg.Traces.CaptureErrorStack,
				CaptureErrorDetail:    cfg.Traces.CaptureErrorDetail,
				MaxPayloadBytes:       cfg.Traces.MaxPayloadBytes,
				MaxStackBytes:         cfg.Traces.MaxStackBytes,
				MaxDetailBytes:        cfg.Traces.MaxDetailBytes,
				RedactKeys:            cfg.Traces.RedactKeys,

				SuccessRetentionDays: cfg.Traces.SuccessRetentionDays,
				ErrorRetentionDays:   cfg.Traces.ErrorRetentionDays,
			},
		)
		if err := traceBackend.Start(ctx); err != nil {
			initErr = err
			return
		}
		if err := traceBackend.StartConsumer(ctx); err != nil {
			initErr = err
			return
		}
		registerGORMTraceCallbacks(global.DB, cfg.ServiceName)

		global.ObservabilityMetrics = metricsBackend
		global.ObservabilityTraces = traceBackend
	})

	return initErr
}

type observabilityTraceStoreAdapter struct {
	repo interfaces.ObservabilityTraceRepository
}

func (a *observabilityTraceStoreAdapter) BatchCreateIgnoreDup(
	ctx context.Context,
	rows []*entity.ObservabilityTraceSpan,
) error {
	return a.repo.BatchCreateIgnoreDup(ctx, rows)
}

func (a *observabilityTraceStoreAdapter) ListByRequestID(
	ctx context.Context,
	requestID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	return a.repo.ListByRequestID(ctx, requestID, limit, offset, includePayload, includeErrorDetail)
}

func (a *observabilityTraceStoreAdapter) ListByTraceID(
	ctx context.Context,
	traceID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	return a.repo.ListByTraceID(ctx, traceID, limit, offset, includePayload, includeErrorDetail)
}

func (a *observabilityTraceStoreAdapter) Query(
	ctx context.Context,
	q *obstrace.Query,
) ([]*entity.ObservabilityTraceSpan, int64, error) {
	if q == nil {
		q = &obstrace.Query{}
	}
	return a.repo.Query(ctx, &interfaces.ObservabilityTraceQuery{
		TraceID:            strings.TrimSpace(q.TraceID),
		RequestID:          strings.TrimSpace(q.RequestID),
		Service:            strings.TrimSpace(q.Service),
		Stage:              strings.TrimSpace(q.Stage),
		Status:             strings.TrimSpace(q.Status),
		StartAt:            q.StartAt,
		EndAt:              q.EndAt,
		Limit:              q.Limit,
		Offset:             q.Offset,
		IncludePayload:     q.IncludePayload,
		IncludeErrorDetail: q.IncludeErrorDetail,
	})
}

func (a *observabilityTraceStoreAdapter) DeleteBeforeByStatus(
	ctx context.Context,
	status string,
	before time.Time,
) error {
	return a.repo.DeleteBeforeByStatus(ctx, status, before)
}
