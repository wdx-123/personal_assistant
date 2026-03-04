package trace

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

const (
	// SpanStatusOK 表示操作成功
	SpanStatusOK = "ok"
	// SpanStatusError 表示操作失败
	SpanStatusError = "error"
)

// Span 链路追踪中的一个执行片段
// 对应分布式追踪中的 Span 概念，记录一次具体操作（如 HTTP 请求、DB 查询、函数调用）
type Span struct {
	// SpanID 本次操作的唯一标识（16 hex），由 W3C 标准生成
	SpanID string `json:"span_id"`
	// ParentSpanID 父操作的标识（16 hex），用于构建树状调用链
	ParentSpanID string `json:"parent_span_id"`
	// TraceID 整条链路的唯一标识（32 hex），贯穿整个请求生命周期
	TraceID string `json:"trace_id"`
	// RequestID 业务层面的请求 ID（通常为 UUID），便于日志检索
	RequestID string `json:"request_id"`

	// Service 服务名称（如 "order-service"），用于微服务架构下的服务区分
	Service string `json:"service"`
	// Stage 阶段标识（如 "controller", "service", "dao", "cache"），用于区分层级
	Stage string `json:"stage"`
	// Name 操作名称（如 "GET /api/v1/users", "db.query"），需具备低基数特征
	Name string `json:"name"`
	// Kind Span 类型（如 "server", "client", "internal", "producer", "consumer"）
	Kind string `json:"kind"`
	// Status 状态（"ok" 或 "error"），用于快速过滤失败链路
	Status string `json:"status"`

	// StartAt 操作开始时间（UTC）
	StartAt time.Time `json:"start_at"`
	// EndAt 操作结束时间（UTC）
	EndAt time.Time `json:"end_at"`
	// DurationMs 耗时（毫秒），用于性能分析
	DurationMs int64 `json:"duration_ms"`

	// ErrorCode 业务错误码（如 "404", "10001"），仅在 Status="error" 时有意义
	ErrorCode string `json:"error_code"`
	// Message 错误消息或备注信息
	Message string `json:"message"`
	// Tags 自定义标签（键值对），用于携带额外的上下文信息（如 "user_id", "db.table"）
	Tags map[string]string `json:"tags"`

	// RequestSnippet 请求体片段（截断），用于复现问题，仅在错误或调试模式下记录
	RequestSnippet string `json:"request_snippet"`
	// ResponseSnippet 响应体片段（截断），用于复现问题，仅在错误或调试模式下记录
	ResponseSnippet string `json:"response_snippet"`
	// ErrorStack 错误堆栈信息，用于定位 panic 或深层错误
	ErrorStack string `json:"error_stack"`
	// ErrorDetailJSON 结构化的错误详情（JSON 字符串），用于前端展示或深入分析
	ErrorDetailJSON string `json:"error_detail_json"`
}

// Query 追踪查询条件
// 用于在 API 层接收前端的查询参数，并传递给存储层
type Query struct {
	// TraceID 精确匹配
	TraceID string
	// RequestID 精确匹配
	RequestID string
	// Service 精确匹配
	Service string
	// Stage 精确匹配
	Stage string
	// Status 精确匹配 ("ok" / "error")
	Status string

	// StartAt 时间范围开始（包含）
	StartAt time.Time
	// EndAt 时间范围结束（不包含）
	EndAt time.Time

	// Limit 分页大小
	Limit int
	// Offset 分页偏移
	Offset int

	// IncludePayload 是否包含 RequestSnippet/ResponseSnippet（大字段，默认不查以提升性能）
	IncludePayload bool
	// IncludeErrorDetail 是否包含 ErrorStack/ErrorDetailJSON（大字段，默认不查）
	IncludeErrorDetail bool
}

// Options 全链路追踪后端配置
// 通常由配置文件映射而来，控制采集、采样、存储等行为
type Options struct {
	// Enabled 是否启用追踪功能
	Enabled bool

	// ServiceName 当前服务名称，用于填充 Span.Service
	ServiceName string

	// StreamKey Redis Stream 的 Key
	StreamKey string
	// StreamGroup 消费者组名称
	StreamGroup string
	// StreamConsumer 消费者名称（建议包含 hostname/pid 以区分实例）
	StreamConsumer string
	// StreamReadCount 每次从 Stream 读取的消息数量
	StreamReadCount int64
	// StreamBlock 消费者阻塞读取的超时时间
	StreamBlock time.Duration
	// PendingIdle 消息处于 Pending 状态多久后被视为超时（可被 Claim）
	PendingIdle time.Duration

	// DBBatchSize 批量写入数据库的大小
	DBBatchSize int
	// DBFlushInterval 批量写入数据库的时间间隔
	DBFlushInterval time.Duration

	// NormalQueueSize 正常（成功）Span 的内存缓冲队列大小
	NormalQueueSize int
	// CriticalQueueSize 异常（失败）Span 的内存缓冲队列大小（通常更大，防丢）
	CriticalQueueSize int
	// EnqueueTimeout 写入内存队列的超时时间（队列满时）
	EnqueueTimeout time.Duration

	// SuccessSampleRate 成功请求的采样率 (0.0 - 1.0)
	SuccessSampleRate float64
	// DropSuccessOnOverload 当系统过载时，是否优先丢弃成功请求的 Span
	DropSuccessOnOverload bool
	// CaptureErrorPayload 是否在 SpanStatusError 时采集请求/响应体
	CaptureErrorPayload bool
	// CaptureErrorStack 是否在 SpanStatusError 时采集堆栈
	CaptureErrorStack bool
	// CaptureErrorDetail 是否在 SpanStatusError 时采集结构化错误详情
	CaptureErrorDetail bool
	// MaxPayloadBytes 请求/响应体截断长度
	MaxPayloadBytes int
	// MaxStackBytes 堆栈截断长度
	MaxStackBytes int
	// MaxDetailBytes 错误详情截断长度
	MaxDetailBytes int
	// RedactKeys 需要脱敏的字段 Key（如 "password", "token"）
	RedactKeys []string

	// SuccessRetentionDays 成功 Span 的保留天数
	SuccessRetentionDays int
	// ErrorRetentionDays 失败 Span 的保留天数（通常比成功长）
	ErrorRetentionDays int
}

// TraceBackend 全链路追踪后端接口
// 定义了 Span 的采集、查询、清理以及后台任务的生命周期管理
type TraceBackend interface {
	// RecordSpan 记录一个 Span。实现应是异步非阻塞的。
	RecordSpan(ctx context.Context, span *Span) error
	// ListByRequestID 按 RequestID 查询 Span 列表
	ListByRequestID(ctx context.Context, requestID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*Span, int64, error)
	// ListByTraceID 按 TraceID 查询 Span 列表
	ListByTraceID(ctx context.Context, traceID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*Span, int64, error)
	// Query 按综合条件查询 Span
	Query(ctx context.Context, q *Query) ([]*Span, int64, error)
	// CleanupBeforeByStatus 按状态清理指定时间之前的数据
	CleanupBeforeByStatus(ctx context.Context, status string, before time.Time) error
	// Start 启动后台写入协程（Producer）
	Start(ctx context.Context) error
	// StartConsumer 启动后台消费协程（Consumer，从 Redis 到 DB）
	StartConsumer(ctx context.Context) error
}

// Store Span 持久层接口
// 负责与数据库交互，实现 Span 的存储与查询
type Store interface {
	// BatchCreateIgnoreDup 批量插入 Span，遇到重复 SpanID 则忽略（幂等）
	BatchCreateIgnoreDup(ctx context.Context, rows []*entity.ObservabilityTraceSpan) error
	// ListByRequestID 按 RequestID 查询 DB
	ListByRequestID(ctx context.Context, requestID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*entity.ObservabilityTraceSpan, int64, error)
	// ListByTraceID 按 TraceID 查询 DB
	ListByTraceID(ctx context.Context, traceID string, limit, offset int, includePayload bool, includeErrorDetail bool) ([]*entity.ObservabilityTraceSpan, int64, error)
	// Query 按条件查询 DB
	Query(ctx context.Context, q *Query) ([]*entity.ObservabilityTraceSpan, int64, error)
	// DeleteBeforeByStatus 物理删除过期数据
	DeleteBeforeByStatus(ctx context.Context, status string, before time.Time) error
}
