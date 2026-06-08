package config

// SSE 项目级实时推送配置
type SSE struct {
	HeartbeatIntervalSeconds int      `json:"heartbeat_interval_seconds" yaml:"heartbeat_interval_seconds"`
	WriteTimeoutSeconds      int      `json:"write_timeout_seconds" yaml:"write_timeout_seconds"`
	QueueCapacity            int      `json:"queue_capacity" yaml:"queue_capacity"`
	MaxConnectionsPerSubject int      `json:"max_connections_per_subject" yaml:"max_connections_per_subject"`
	ReplayLimit              int      `json:"replay_limit" yaml:"replay_limit"`
	AllowedOrigins           []string `json:"allowed_origins" yaml:"allowed_origins"`
	DrainTimeoutSeconds      int      `json:"drain_timeout_seconds" yaml:"drain_timeout_seconds"`
	PubSubChannelPrefix      string   `json:"pubsub_channel_prefix" yaml:"pubsub_channel_prefix"`
	ReplayStreamPrefix       string   `json:"replay_stream_prefix" yaml:"replay_stream_prefix"`
	AIRuntimeMode            string   `json:"ai_runtime_mode" yaml:"ai_runtime_mode"`
}
