package response

// ObservabilityMetricPointResp 指标点返回结构
type ObservabilityMetricPointResp struct {
	Granularity    string `json:"granularity"`
	BucketStart    string `json:"bucket_start"`
	Service        string `json:"service"`
	RouteTemplate  string `json:"route_template"`
	Method         string `json:"method"`
	StatusClass    int    `json:"status_class"`
	ErrorCode      string `json:"error_code"`
	RequestCount   int64  `json:"request_count"`
	ErrorCount     int64  `json:"error_count"`
	TotalLatencyMs int64  `json:"total_latency_ms"`
	MaxLatencyMs   int64  `json:"max_latency_ms"`
}

// ObservabilityMetricsQueryResp 指标查询响应
type ObservabilityMetricsQueryResp struct {
	Granularity string                          `json:"granularity"`
	List        []*ObservabilityMetricPointResp `json:"list"`
}

// ObservabilityTraceSummaryResp 追踪 root 摘要响应项
type ObservabilityTraceSummaryResp struct {
	TraceID        string `json:"trace_id"`
	RequestID      string `json:"request_id"`
	Service        string `json:"service"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	ErrorCode      string `json:"error_code,omitempty"`
	Message        string `json:"message,omitempty"`
	StartAt        string `json:"start_at"`
	EndAt          string `json:"end_at"`
	DurationMs     int64  `json:"duration_ms"`
	SpanTotal      int64  `json:"span_total"`
	ErrorSpanTotal int64  `json:"error_span_total"`
	Method         string `json:"method,omitempty"`
	RouteTemplate  string `json:"route_template,omitempty"`
}

// ObservabilityTraceSummaryQueryResp 追踪摘要查询响应
type ObservabilityTraceSummaryQueryResp struct {
	List  []*ObservabilityTraceSummaryResp `json:"list"`
	Total int64                            `json:"total"`
}

// ObservabilityTraceSpanResp 全链路 Span 响应
type ObservabilityTraceSpanResp struct {
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id"`
	TraceID      string `json:"trace_id"`
	RequestID    string `json:"request_id"`

	Service string `json:"service"`
	Stage   string `json:"stage"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`

	StartAt    string `json:"start_at"`
	EndAt      string `json:"end_at"`
	DurationMs int64  `json:"duration_ms"`

	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`

	Tags map[string]string `json:"tags,omitempty"`

	RequestSnippet  string `json:"request_snippet,omitempty"`
	ResponseSnippet string `json:"response_snippet,omitempty"`
	ErrorStack      string `json:"error_stack,omitempty"`
	ErrorDetailJSON string `json:"error_detail_json,omitempty"`
}

// ObservabilityTraceQueryResp 追踪详情查询响应
type ObservabilityTraceQueryResp struct {
	List  []*ObservabilityTraceSpanResp `json:"list"`
	Total int64                         `json:"total"`
}
