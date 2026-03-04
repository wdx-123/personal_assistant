package config

// Observability 观测基础设施配置
type Observability struct {
	Enabled      bool                      `json:"enabled" yaml:"enabled"`
	ServiceName  string                    `json:"service_name" yaml:"service_name"`
	ServiceTrace ObservabilityServiceTrace `json:"service_trace" yaml:"service_trace"`
	Propagation  ObservabilityPropagation  `json:"propagation" yaml:"propagation"`
	Metrics      ObservabilityMetrics      `json:"metrics" yaml:"metrics"`
	Traces       ObservabilityTraces       `json:"traces" yaml:"traces"`
}

// ObservabilityServiceTrace Service 层追踪配置
type ObservabilityServiceTrace struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Modules []string `json:"modules" yaml:"modules"`
}

// ObservabilityPropagation 观测链路传播配置
type ObservabilityPropagation struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	RequestIDHeader string `json:"request_id_header" yaml:"request_id_header"`
	ParseW3C        bool   `json:"parse_w3c" yaml:"parse_w3c"`
	InjectW3C       bool   `json:"inject_w3c" yaml:"inject_w3c"`
}

// ObservabilityMetrics 指标聚合配置
type ObservabilityMetrics struct {
	FlushIntervalMs   int    `json:"flush_interval_ms" yaml:"flush_interval_ms"`
	DBBatchSize       int    `json:"db_batch_size" yaml:"db_batch_size"`
	FineRetentionDays int    `json:"fine_retention_days" yaml:"fine_retention_days"`
	DayRetentionDays  int    `json:"day_retention_days" yaml:"day_retention_days"`
	WeekRetentionDays int    `json:"week_retention_days" yaml:"week_retention_days"`
	RollupCron        string `json:"rollup_cron" yaml:"rollup_cron"`
}

// ObservabilityTraces 全链路追踪配置
type ObservabilityTraces struct {
	Enabled bool `json:"enabled" yaml:"enabled"`

	StreamKey       string `json:"stream_key" yaml:"stream_key"`
	StreamGroup     string `json:"stream_group" yaml:"stream_group"`
	StreamConsumer  string `json:"stream_consumer" yaml:"stream_consumer"`
	StreamReadCount int    `json:"stream_read_count" yaml:"stream_read_count"`
	StreamBlockMs   int    `json:"stream_block_ms" yaml:"stream_block_ms"`
	PendingIdleMs   int    `json:"pending_idle_ms" yaml:"pending_idle_ms"`

	DBBatchSize       int `json:"db_batch_size" yaml:"db_batch_size"`
	DBFlushIntervalMs int `json:"db_flush_interval_ms" yaml:"db_flush_interval_ms"`

	NormalQueueSize   int `json:"normal_queue_size" yaml:"normal_queue_size"`
	CriticalQueueSize int `json:"critical_queue_size" yaml:"critical_queue_size"`
	EnqueueTimeoutMs  int `json:"enqueue_timeout_ms" yaml:"enqueue_timeout_ms"`

	SuccessSampleRate     float64  `json:"success_sample_rate" yaml:"success_sample_rate"`
	DropSuccessOnOverload bool     `json:"drop_success_on_overload" yaml:"drop_success_on_overload"`
	CaptureErrorPayload   bool     `json:"capture_error_payload" yaml:"capture_error_payload"`
	MaxPayloadBytes       int      `json:"max_payload_bytes" yaml:"max_payload_bytes"`
	CaptureErrorStack     bool     `json:"capture_error_stack" yaml:"capture_error_stack"`
	CaptureErrorDetail    bool     `json:"capture_error_detail" yaml:"capture_error_detail"`
	MaxStackBytes         int      `json:"max_stack_bytes" yaml:"max_stack_bytes"`
	MaxDetailBytes        int      `json:"max_detail_bytes" yaml:"max_detail_bytes"`
	RedactKeys            []string `json:"redact_keys" yaml:"redact_keys"`

	SuccessRetentionDays int    `json:"success_retention_days" yaml:"success_retention_days"`
	ErrorRetentionDays   int    `json:"error_retention_days" yaml:"error_retention_days"`
	CleanupCron          string `json:"cleanup_cron" yaml:"cleanup_cron"`
}
