package request

import "strings"

// ObservabilityMetricsQueryReq 指标查询请求
type ObservabilityMetricsQueryReq struct {
	Granularity   string  `json:"granularity" binding:"required"`
	StartAt       string  `json:"start_at" binding:"required"`
	EndAt         string  `json:"end_at" binding:"required"`
	Service       string  `json:"service"`
	RouteTemplate string  `json:"route_template"`
	Method        string  `json:"method"`
	StatusClass   int     `json:"status_class"`
	ErrorCode     *string `json:"error_code"`
	Limit         int     `json:"limit"`
}

const (
	// TraceDetailIDTypeTrace 表示按 trace_id 查询详情。
	TraceDetailIDTypeTrace = "trace"
	// TraceDetailIDTypeRequest 表示按 request_id 查询详情。
	TraceDetailIDTypeRequest = "request"

	// TraceRootStageHTTP 表示默认的 HTTP 请求 root。
	TraceRootStageHTTP = "http.request"
	// TraceRootStageTask 表示定时任务 root。
	TraceRootStageTask = "task"
	// TraceRootStageAll 表示同时查询 HTTP 与任务 root。
	TraceRootStageAll = "all"
)

// IsValidTraceDetailIDType 校验详情查询 id_type 参数。
func IsValidTraceDetailIDType(idType string) bool {
	switch strings.ToLower(strings.TrimSpace(idType)) {
	case TraceDetailIDTypeTrace, TraceDetailIDTypeRequest:
		return true
	default:
		return false
	}
}

// NormalizeTraceDetailIDType 归一化详情查询 id_type。
func NormalizeTraceDetailIDType(idType string) string {
	return strings.ToLower(strings.TrimSpace(idType))
}

// NormalizeTraceRootStage 归一化 root_stage；空值保留给上层做默认处理。
func NormalizeTraceRootStage(rootStage string) string {
	return strings.ToLower(strings.TrimSpace(rootStage))
}

// IsValidTraceRootStage 校验 root_stage 参数。
func IsValidTraceRootStage(rootStage string) bool {
	switch NormalizeTraceRootStage(rootStage) {
	case TraceRootStageHTTP, TraceRootStageTask, TraceRootStageAll:
		return true
	default:
		return false
	}
}

// ObservabilityTraceQueryReq 追踪 root 摘要查询请求
type ObservabilityTraceQueryReq struct {
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
	Service   string `json:"service"`
	Status    string `json:"status"`
	RootStage string `json:"root_stage"`
	StartAt   string `json:"start_at"`
	EndAt     string `json:"end_at"`

	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ObservabilityRuntimeMetricQueryReq struct {
	Metric      string `json:"metric"` // 指标名称
	StartAt     string `json:"start_at"`
	EndAt       string `json:"end_at"`
	Granularity string `json:"granularity"`
	TaskName    string `json:"task_name"`
	Topic       string `json:"topic"`
	Status      string `json:"status"`
	Limit       int    `json:"limit"`
}
