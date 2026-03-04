package request

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

// ObservabilityTraceQueryReq 追踪 Span 查询请求
type ObservabilityTraceQueryReq struct {
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
	Service   string `json:"service"`
	Stage     string `json:"stage"`
	Status    string `json:"status"`
	StartAt   string `json:"start_at"`
	EndAt     string `json:"end_at"`

	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	IncludePayload     bool `json:"include_payload"`
	IncludeErrorDetail bool `json:"include_error_detail"`
}
