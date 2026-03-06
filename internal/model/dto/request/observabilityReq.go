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

// ObservabilityTraceQueryReq 追踪 root 摘要查询请求
type ObservabilityTraceQueryReq struct {
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
	Service   string `json:"service"`
	Status    string `json:"status"`
	StartAt   string `json:"start_at"`
	EndAt     string `json:"end_at"`

	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
