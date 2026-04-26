package config

// AI 描述 AI runtime 与 Eino 相关配置。
type AI struct {
	Provider            string   `json:"provider" yaml:"provider"`
	APIKey              string   `json:"api_key" yaml:"api_key"`
	BaseURL             string   `json:"base_url" yaml:"base_url"`
	Model               string   `json:"model" yaml:"model"`
	ByAzure             bool     `json:"by_azure" yaml:"by_azure"`
	APIVersion          string   `json:"api_version" yaml:"api_version"`
	SystemPrompt        string   `json:"system_prompt" yaml:"system_prompt"`
	Temperature         float64  `json:"temperature" yaml:"temperature"`
	MaxCompletionTokens int      `json:"max_completion_tokens" yaml:"max_completion_tokens"`
	Memory              AIMemory `json:"memory" yaml:"memory"`
}

// AIMemory 描述记忆模块的冻结配置。
type AIMemory struct {
	// Enabled 控制记忆模块总开关。
	// Phase 1 只影响配置装配和后续能力接线，不会直接改变当前 AI 对话链路。
	Enabled bool `json:"enabled" yaml:"enabled"`
	// RecallTopK 控制单次召回时每类候选最多取多少条记忆。
	// 后续混合召回会基于这个值做二次裁剪，而不是无上限地把记忆塞进 prompt。
	RecallTopK int `json:"recall_top_k" yaml:"recall_top_k"`
	// RecallMaxChars 控制记忆召回结果在拼装 prompt 前允许占用的最大字符数。
	// 这个值用于防止召回文本过长，导致压缩摘要和最近消息被挤出上下文。
	RecallMaxChars int `json:"recall_max_chars" yaml:"recall_max_chars"`
	// RecallMinScore 控制 RAG 向量召回候选的最低相似度分数。
	RecallMinScore float64 `json:"recall_min_score" yaml:"recall_min_score"`
	// RAGMaxChars 控制长期文档片段注入 memory message 的最大字符数。
	RAGMaxChars int `json:"rag_max_chars" yaml:"rag_max_chars"`
	// RecentRawTurns 控制上下文恢复时保留多少轮最近原始消息。
	// 后续会采用 “summary + recent turns” 的组合，这个值决定 recent turns 的窗口大小。
	RecentRawTurns int `json:"recent_raw_turns" yaml:"recent_raw_turns"`
	// CompressThresholdTokens 表示会话历史接近多少 token 时开始触发摘要压缩。
	// 它不是模型最大上下文，而是系统层面的提前压缩阈值。
	CompressThresholdTokens int `json:"compress_threshold_tokens" yaml:"compress_threshold_tokens"`
	// SummaryRefreshEveryTurns 控制会话摘要每累计多少轮对话后刷新一次。
	// 后续写回链路会据此决定是同步刷新 summary，还是延迟到下一次压缩窗口再做。
	SummaryRefreshEveryTurns int `json:"summary_refresh_every_turns" yaml:"summary_refresh_every_turns"`
	// WritebackAsync 控制记忆写回是否优先走异步路径。
	// Phase 1 先冻结配置口径，后续 writeback hook 接入时会据此决定是否走 outbox。
	WritebackAsync bool `json:"writeback_async" yaml:"writeback_async"`
	// EnableEntityMemory 控制结构化事实记忆是否启用。
	// 这类记忆通常对应用户偏好、目标、画像等可覆盖事实，最终落到 AIMemoryFact。
	EnableEntityMemory bool `json:"enable_entity_memory" yaml:"enable_entity_memory"`
	// EnableLongTermMemory 控制长期文本记忆是否启用。
	// 这类记忆最终落到 AIMemoryDocument，并在后续阶段进入 embedding / vector index 流程。
	EnableLongTermMemory bool `json:"enable_long_term_memory" yaml:"enable_long_term_memory"`
	// EnableOrgMemory 控制组织级共享记忆是否参与写入和召回。
	// 关闭后，系统仍可保留 self 级记忆，但不会沉淀 org 侧 FAQ、画像或组织经验。
	EnableOrgMemory bool `json:"enable_org_memory" yaml:"enable_org_memory"`
	// EnableOpsMemory 控制平台运维类记忆是否启用。
	// 这类记忆通常对应 incident、runbook 和排障经验，后续只允许超管范围读写。
	EnableOpsMemory bool `json:"enable_ops_memory" yaml:"enable_ops_memory"`
	// MinImportance 是记忆准入或保留时的最低重要度阈值。
	// 后续治理阶段会用它过滤低价值候选，避免把闲聊、噪音和瞬时状态写进长期记忆。
	MinImportance float64 `json:"min_importance" yaml:"min_importance"`
	// EmbedModel 指定记忆文档使用的 embedding 模型名称。
	// 默认使用阿里云百炼 qwen3-vl-embedding。
	EmbedModel string `json:"embed_model" yaml:"embed_model"`
	// EmbedEndpoint 指定记忆文档 embedding 的 HTTP 接口地址。
	EmbedEndpoint string `json:"embed_endpoint" yaml:"embed_endpoint"`
	// EmbedDimension 指定 embedding 输出维度，必须和 Qdrant memory collection 维度一致。
	EmbedDimension int `json:"embed_dimension" yaml:"embed_dimension"`
	// ChunkMaxChars 控制单个记忆 chunk 的最大字符数。
	ChunkMaxChars int `json:"chunk_max_chars" yaml:"chunk_max_chars"`
	// ChunkOverlapChars 控制相邻 chunk 的尾部重叠字符数。
	ChunkOverlapChars int `json:"chunk_overlap_chars" yaml:"chunk_overlap_chars"`
	// IndexBatchSize 控制补偿扫描时单批最多处理多少 documents。
	IndexBatchSize int `json:"index_batch_size" yaml:"index_batch_size"`
	// IndexTimeoutSeconds 控制单次异步索引任务超时时间。
	IndexTimeoutSeconds int `json:"index_timeout_seconds" yaml:"index_timeout_seconds"`
}
