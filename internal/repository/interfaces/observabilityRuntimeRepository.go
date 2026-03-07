package interfaces

import (
	"context"
	"time"
)

// RuntimeSeriesPoint 运行时指标时序数据点
// 用于图表展示，每个点代表一个时间桶内的聚合数据
type RuntimeSeriesPoint struct {
	BucketStart     time.Time `json:"bucket_start"`      // 时间桶开始时间
	Status          string    `json:"status"`            // 执行状态 (success/error/skipped/pending/published/failed)
	Name            string    `json:"name"`              // 任务名称
	Topic           string    `json:"topic"`             // 消息主题
	Count           int64     `json:"count"`             // 执行次数
	TotalDurationMs int64     `json:"total_duration_ms"` // 总耗时(ms)
	MaxDurationMs   int64     `json:"max_duration_ms"`   // 最大耗时(ms)
}

// RuntimeTaskQuery 后台任务查询条件
type RuntimeTaskQuery struct {
	TaskName    string    // 任务名称(精确匹配)
	Status      string    // 任务状态(success/error/skipped)
	StartAt     time.Time // 开始时间(包含)
	EndAt       time.Time // 结束时间(不包含)
	Granularity string    // 时间粒度(1m/5m/1h/1d)
	Limit       int       // 返回点数限制
}

// RuntimeEventQuery 消息事件查询条件
type RuntimeEventQuery struct {
	Topic       string    // 消息主题
	Status      string    // 处理状态(success/error)
	StartAt     time.Time // 开始时间(包含)
	EndAt       time.Time // 结束时间(不包含)
	Granularity string    // 时间粒度(1m/5m/1h/1d)
	Limit       int       // 返回点数限制
}

// RuntimeOutboxQuery 发件箱查询条件
type RuntimeOutboxQuery struct {
	Status      string    // 发送状态(published/failed)
	StartAt     time.Time // 开始时间(包含)
	EndAt       time.Time // 结束时间(不包含)
	Granularity string    // 时间粒度(1m/5m/1h/1d)
	Limit       int       // 返回点数限制
}

// RuntimeOutboxSnapshot 发件箱实时快照
type RuntimeOutboxSnapshot struct {
	Pending    int64     `json:"pending"`     // 待发送总数
	Published  int64     `json:"published"`   // 已发送总数
	Failed     int64     `json:"failed"`      // 发送失败总数
	SnapshotAt time.Time `json:"snapshot_at"` // 快照时间
}

// ObservabilityRuntimeRepository 运行时观测指标仓储接口
// 负责查询基于 Trace 数据聚合生成的运行时指标（后台任务、消息队列等）
type ObservabilityRuntimeRepository interface {
	// QueryTaskExecutionSeries 查询任务执行次数时序数据
	// 用于展示任务执行频率、成功率趋势图
	QueryTaskExecutionSeries(ctx context.Context, q *RuntimeTaskQuery) ([]*RuntimeSeriesPoint, error)

	// QueryTaskDurationSeries 查询任务耗时时序数据
	// 用于展示任务平均耗时、最大耗时趋势图
	QueryTaskDurationSeries(ctx context.Context, q *RuntimeTaskQuery) ([]*RuntimeSeriesPoint, error)

	// ListTaskDurations 列出任务所有执行耗时
	// 用于计算 P50/P95/P99 分位数值（需配合 QueryTaskDurationSeries 使用）
	ListTaskDurations(ctx context.Context, q *RuntimeTaskQuery) ([]int64, error)

	// QueryPublishSeries 查询消息发布次数时序数据
	// 用于展示消息生产速率趋势图
	QueryPublishSeries(ctx context.Context, q *RuntimeEventQuery) ([]*RuntimeSeriesPoint, error)

	// ListPublishDurations 列出消息发布所有耗时
	// 用于计算消息发布耗时分位数
	ListPublishDurations(ctx context.Context, q *RuntimeEventQuery) ([]int64, error)

	// QueryConsumeSeries 查询消息消费次数时序数据
	// 用于展示消息消费速率趋势图
	QueryConsumeSeries(ctx context.Context, q *RuntimeEventQuery) ([]*RuntimeSeriesPoint, error)

	// ListConsumeDurations 列出消息消费所有耗时
	// 用于计算消息消费耗时分位数
	ListConsumeDurations(ctx context.Context, q *RuntimeEventQuery) ([]int64, error)

	// QueryOutboxStatusSeries 查询发件箱状态时序数据
	// 用于展示消息发送历史趋势（已发送/失败）
	QueryOutboxStatusSeries(ctx context.Context, q *RuntimeOutboxQuery) ([]*RuntimeSeriesPoint, error)

	// GetOutboxStatusSnapshot 获取发件箱实时快照
	// 用于展示当前积压、累计发送和累计失败的实时统计大屏
	GetOutboxStatusSnapshot(ctx context.Context) (*RuntimeOutboxSnapshot, error)
}
