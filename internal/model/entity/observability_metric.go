package entity

import "time"

// ObservabilityMetric HTTP 指标聚合数据（1m/5m/1d/1w）
type ObservabilityMetric struct {
	MODEL
	Granularity   string    `json:"granularity" gorm:"type:varchar(8);not null;uniqueIndex:uk_observability_metric,priority:1;comment:'粒度 1m/5m/1d/1w'"`
	BucketStart   time.Time `json:"bucket_start" gorm:"type:datetime;not null;uniqueIndex:uk_observability_metric,priority:2;index:idx_observability_metric_bucket,priority:1;comment:'桶起始时间'"`
	Service       string    `json:"service" gorm:"type:varchar(64);not null;uniqueIndex:uk_observability_metric,priority:3;index:idx_observability_metric_bucket,priority:2;comment:'服务名'"`
	RouteTemplate string    `json:"route_template" gorm:"type:varchar(255);not null;uniqueIndex:uk_observability_metric,priority:4;comment:'路由模板'"`
	Method        string    `json:"method" gorm:"type:varchar(16);not null;uniqueIndex:uk_observability_metric,priority:5;comment:'HTTP方法'"`
	StatusClass   int       `json:"status_class" gorm:"type:int;not null;uniqueIndex:uk_observability_metric,priority:6;comment:'状态码类型(2/4/5)'"`
	ErrorCode     string    `json:"error_code" gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_observability_metric,priority:7;comment:'业务错误码(可空)'"`

	RequestCount   int64 `json:"request_count" gorm:"type:bigint;not null;default:0;comment:'请求数'"`
	ErrorCount     int64 `json:"error_count" gorm:"type:bigint;not null;default:0;comment:'错误数'"`
	TotalLatencyMs int64 `json:"total_latency_ms" gorm:"type:bigint;not null;default:0;comment:'总耗时毫秒'"`
	MaxLatencyMs   int64 `json:"max_latency_ms" gorm:"type:bigint;not null;default:0;comment:'最大耗时毫秒'"`
}

func (ObservabilityMetric) TableName() string {
	return "observability_metrics"
}
