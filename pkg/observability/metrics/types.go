package metrics

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

// HTTPRecord 表示一次 HTTP 请求的“采样记录”（原始点）。
// 该结构通常在请求结束时由 middleware/拦截器构造并上报到 MetricsBackend（RecordHTTP）。
//
// 设计目标：
//   - 字段尽量少且稳定，便于高吞吐写入与低成本聚合。
//   - 以“可聚合维度 + 可度量数值”为主（route/method/status/error_code + latency）。
//
// 字段语义：
//   - Timestamp：事件发生时间（建议使用 UTC），用于落入聚合桶。
//   - Service：服务名（用于多服务聚合/筛选）；为空时通常由后端兜底为 Options.ServiceName。
//   - RouteTemplate：路由模板（如 "/v1/users/:id"），避免使用具体 path 导致高基数。
//   - Method：HTTP 方法（建议统一大写）。
//   - StatusCode/StatusClass：HTTP 状态码及其类别（2/4/5）；StatusClass 通常可由 StatusCode/100 推导。
//   - ErrorCode：业务错误码（与 HTTP status 不同），用于区分同类状态下的业务错误原因；为空表示无业务错误。
//   - LatencyMs：请求耗时（毫秒）；建议在入口统一采用同一计时口径（如 handler 总耗时或网关耗时）。
//
// 高基数风险提示：
//   - RouteTemplate、ErrorCode 如果来源不稳定/不可枚举，会造成指标维度爆炸（存储与查询成本剧增）。
//   - 生产建议对 ErrorCode 做白名单/采样/截断策略，或仅在 status_class>=4 时上报。
type HTTPRecord struct {
	// Timestamp 为请求采样的发生时间点（推荐使用 time.Now().UTC()）。
	Timestamp time.Time

	// Service 为服务名（例如 "user-api"），用于聚合/过滤。
	Service string

	// RouteTemplate 为路由模板（而非真实 path），用于控制基数。
	RouteTemplate string

	// Method 为 HTTP 方法（GET/POST/...），建议大写。
	Method string

	// StatusCode 为 HTTP 状态码。
	StatusCode int

	// StatusClass 为状态码类别（2/4/5），便于聚合统计（如 5xx 错误率）。
	StatusClass int

	// ErrorCode 为业务错误码（可选）。空字符串表示无业务错误或不采集。
	ErrorCode string

	// LatencyMs 为请求耗时（毫秒）。
	LatencyMs int64
}

// MetricPoint 表示指标查询返回的“聚合点”（聚合后的统计结果）。
// 它通常来源于存储层的聚合表/指标表（例如按分钟/小时/天/周 rollup 后的数据）。
//
// 关键概念：
//   - Granularity：聚合粒度（例如 "minute"/"hour"/"day"/"week" 或你们内部约定的枚举）。
//   - BucketStart：该聚合桶的起始时间点（对齐粒度边界），用于绘图与时间序列分析。
//   - 维度字段（Service/RouteTemplate/Method/StatusClass/ErrorCode）用于切片与分组。
//   - 指标字段（RequestCount/ErrorCount/TotalLatencyMs/MaxLatencyMs）用于计算 QPS、错误率、平均/最大耗时等。
//
// 指标口径建议：
//   - RequestCount：该桶内符合维度条件的总请求数。
//   - ErrorCount：该桶内错误请求数（一般 status_class>=4 或 error_code 非空，具体口径由实现定义并保持稳定）。
//   - TotalLatencyMs：该桶内延迟总和（用于计算平均延迟 = TotalLatencyMs/RequestCount）。
//   - MaxLatencyMs：该桶内最大延迟（用于观察尾延迟/突刺）。
type MetricPoint struct {
	// Granularity 为聚合粒度标识（应与查询参数一致）。
	Granularity string

	// BucketStart 为聚合桶开始时间（通常为对齐后的时间，如分钟桶 10:05:00）。
	BucketStart time.Time

	// Service/RouteTemplate/Method/StatusClass/ErrorCode 为维度字段。
	Service       string
	RouteTemplate string
	Method        string
	StatusClass   int
	ErrorCode     string

	// RequestCount 为该桶请求总数。
	RequestCount int64

	// ErrorCount 为该桶错误请求数（口径由实现定义）。
	ErrorCount int64

	// TotalLatencyMs 为该桶延迟总和（毫秒）。
	TotalLatencyMs int64

	// MaxLatencyMs 为该桶最大延迟（毫秒）。
	MaxLatencyMs int64
}

// MetricsBackend 定义指标采集与查询的后端接口。
// 典型实现会：
//   - 在 RecordHTTP 中将 HTTPRecord 写入缓冲（内存/Redis/队列）或直接写存储（DB/Prometheus/OTel）。
//   - 在 QueryMetrics 中按时间范围与维度条件返回聚合点。
//   - 在 RollupAndCleanup 中将细粒度数据聚合到粗粒度，并按保留策略清理历史数据。
//
// 接口设计说明：
//   - 采集（RecordHTTP）与查询（QueryMetrics）分离，便于替换存储实现（MySQL、ClickHouse、Prometheus、OTel）。
//   - RollupAndCleanup 通常由后台任务/定时器触发，避免在请求链路上做重计算。
type MetricsBackend interface {
	// RecordHTTP 记录一次 HTTP 请求采样。
	// 实现应尽量“轻量/低延迟”，避免影响业务主流程；必要时可降级（例如失败仅打日志）。
	RecordHTTP(ctx context.Context, record *HTTPRecord) error

	// QueryMetrics 查询指定时间范围内的指标聚合结果。
	//
	// 参数约定：
	//   - granularity：聚合粒度（例如 minute/hour/day/week）。
	//   - start/end：时间范围（建议使用 UTC；通常为 [start, end) 或 [start, end]，实现需固定并在文档说明）。
	//   - service/routeTemplate/method/statusClass/errorCode：过滤维度；空值/0 通常表示“不筛选该维度”（具体约定由实现定义）。
	//   - errorCode 使用 *string：nil 表示“不按 error_code 过滤”，非 nil 表示过滤指定 error_code（可支持空字符串表示仅查询 error_code 为空）。
	//   - limit：限制返回点的数量，防止超大查询造成存储压力与响应膨胀。
	QueryMetrics(
		ctx context.Context,
		granularity string,
		start time.Time,
		end time.Time,
		service string,
		routeTemplate string,
		method string,
		statusClass int,
		errorCode *string,
		limit int,
	) ([]*MetricPoint, error)

	// RollupAndCleanup 执行聚合与清理任务。
	// 常见工作：
	//   - 将细粒度（例如 minute）数据聚合到 day/week 等更粗粒度，以支持长时间范围查询。
	//   - 按不同粒度保留策略清理历史数据（FineRetentionDays/DayRetentionDays/WeekRetentionDays）。
	//
	// now 参数用于对齐桶边界与计算清理阈值，便于测试与可重复执行。
	RollupAndCleanup(ctx context.Context, now time.Time) error
}

// Store 定义指标存储接口（适配 MySQL/Prometheus/OTel 等）。
// 该接口偏“数据层原语”，供 MetricsBackend 实现调用；不同存储可采用不同策略：
//   - MySQL/ClickHouse：表结构 + upsert/aggregate + delete。
//   - Prometheus/OTel：可能不需要 Aggregate/DeleteBefore（由系统自身保留策略管理），实现可做适配或部分 no-op。
//
// 方法说明：
//   - IncrementBatch：增量写入（计数/累加）型指标，适用于高频写入与按桶累加。
//   - UpsertAbsoluteBatch：绝对值覆盖写入（例如 rollup 后写入聚合表），需要幂等。
//   - Query：按条件查询指标行（通常是已聚合表数据）。
//   - Aggregate：将 fromGranularity 的数据聚合到 toGranularity（rollup 核心能力）。
//   - DeleteBeforeByGranularity：按粒度删除历史数据，控制存储成本。
type Store interface {
	// IncrementBatch 批量增量更新指标（例如 request_count +1, total_latency +x）。
	// 适用于“写多读少”的场景；实现通常需要原子性（如 SQL upsert + 累加）。
	IncrementBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error

	// UpsertAbsoluteBatch 批量 upsert 指标的“绝对值”结果（覆盖写入）。
	// 常用于 rollup 输出（聚合后结果可重复计算，写入需幂等）。
	UpsertAbsoluteBatch(ctx context.Context, rows []*entity.ObservabilityMetric) error

	// Query 查询指定粒度与过滤条件的指标行。
	// 返回 entity.ObservabilityMetric 便于后续转换为对外的 MetricPoint。
	Query(
		ctx context.Context,
		granularity string,
		start time.Time,
		end time.Time,
		service string,
		routeTemplate string,
		method string,
		statusClass int,
		errorCode *string,
		limit int,
	) ([]*entity.ObservabilityMetric, error)

	// Aggregate 在存储层执行聚合，将较细粒度数据汇总到较粗粒度。
	// 例如：minute -> day、day -> week。聚合范围通常是 [start, end)。
	Aggregate(
		ctx context.Context,
		fromGranularity string,
		toGranularity string,
		start time.Time,
		end time.Time,
	) ([]*entity.ObservabilityMetric, error)

	// DeleteBeforeByGranularity 删除指定粒度下某时间点之前的数据，用于数据保留（retention）。
	DeleteBeforeByGranularity(ctx context.Context, granularity string, before time.Time) error
}

// Options 表示指标后端配置。
// 设计目标：
//   - 通过 FlushInterval + DBBatchSize 控制写入节奏（吞吐 vs 实时性）。
//   - 通过分粒度保留天数控制成本（细粒度保留短、粗粒度保留长）。
//
// 参数建议：
//   - Enabled：用于一键开关，关闭时采集与后台任务应 no-op。
//   - ServiceName：采集记录缺省服务名兜底，避免空字段导致聚合维度缺失。
//   - FlushInterval：缓冲 flush 周期；越小越实时，越大吞吐越好但延迟更高。
//   - DBBatchSize：批量写入大小；越大越高吞吐，但失败重试成本更高。
//   - FineRetentionDays：细粒度（例如 minute/hour）数据保留天数，通常较短（如 3~14 天）。
//   - DayRetentionDays：天粒度数据保留天数，通常更长（如 30~180 天）。
//   - WeekRetentionDays：周粒度数据保留天数/周数，通常最长（如 52 周）。
type Options struct {
	Enabled     bool
	ServiceName string

	// FlushInterval 控制采集缓冲刷写到存储的周期。
	FlushInterval time.Duration

	// DBBatchSize 控制批量写入大小。
	DBBatchSize int

	// FineRetentionDays 细粒度数据保留天数（例如 minute/hour）。
	FineRetentionDays int

	// DayRetentionDays 天粒度数据保留天数。
	DayRetentionDays int

	// WeekRetentionDays 周粒度数据保留天数（或等效周数，视实现约定）。
	WeekRetentionDays int
}
