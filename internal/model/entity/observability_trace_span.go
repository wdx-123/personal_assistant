package entity

import "time"

// ObservabilityTraceSpan 全链路追踪 Span 明细
type ObservabilityTraceSpan struct {
	MODEL
	SpanID       string `json:"span_id" gorm:"type:varchar(64);not null;uniqueIndex;comment:'SpanID'"`
	ParentSpanID string `json:"parent_span_id" gorm:"type:varchar(64);not null;default:'';comment:'父SpanID'"`

	TraceID   string `json:"trace_id" gorm:"type:varchar(64);not null;index:idx_trace_time,priority:1;comment:'链路ID'"`
	RequestID string `json:"request_id" gorm:"type:varchar(64);not null;index:idx_request_time,priority:1;comment:'请求ID'"`

	Service string `json:"service" gorm:"type:varchar(64);not null;index:idx_service_time,priority:1;comment:'服务名'"`
	Stage   string `json:"stage" gorm:"type:varchar(64);not null;default:'';index:idx_stage_time,priority:1;comment:'阶段'"`
	Name    string `json:"name" gorm:"type:varchar(128);not null;default:'';comment:'Span名称'"`
	Kind    string `json:"kind" gorm:"type:varchar(32);not null;default:'';comment:'Span类型'"`
	Status  string `json:"status" gorm:"type:varchar(16);not null;index:idx_status_time,priority:1;comment:'状态 ok/error'"`

	StartAt    time.Time `json:"start_at" gorm:"type:datetime;not null;index:idx_trace_time,priority:2;index:idx_request_time,priority:2;index:idx_service_time,priority:2;index:idx_status_time,priority:2;index:idx_stage_time,priority:2;comment:'开始时间'"`
	EndAt      time.Time `json:"end_at" gorm:"type:datetime;not null;comment:'结束时间'"`
	DurationMs int64     `json:"duration_ms" gorm:"type:bigint;not null;default:0;comment:'耗时毫秒'"`

	ErrorCode       string `json:"error_code" gorm:"type:varchar(64);not null;default:'';comment:'错误码'"`
	Message         string `json:"message" gorm:"type:text;comment:'错误消息'"`
	TagsJSON        string `json:"tags_json" gorm:"type:json;comment:'标签JSON'"`
	RequestSnippet  string `json:"request_snippet" gorm:"type:text;comment:'请求片段'"`
	ResponseSnippet string `json:"response_snippet" gorm:"type:text;comment:'响应片段'"`
	ErrorStack      string `json:"error_stack" gorm:"type:longtext;comment:'错误堆栈'"`
	ErrorDetailJSON string `json:"error_detail_json" gorm:"type:json;comment:'错误上下文JSON'"`
}

func (ObservabilityTraceSpan) TableName() string {
	return "observability_trace_spans"
}
