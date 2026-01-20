package config

// Task 定时任务配置
type Task struct {
	OutboxCleanupRetentionDays            int  `json:"outbox_cleanup_retention_days" yaml:"outbox_cleanup_retention_days"`
	LuoguQuestionBankWarmupEnabled        bool `json:"luogu_question_bank_warmup_enabled" yaml:"luogu_question_bank_warmup_enabled"`
	LuoguQuestionBankWarmupBatchSize      int  `json:"luogu_question_bank_warmup_batch_size" yaml:"luogu_question_bank_warmup_batch_size"`
	LuoguQuestionBankWarmupLockTTLSeconds int  `json:"luogu_question_bank_warmup_lock_ttl_seconds" yaml:"luogu_question_bank_warmup_lock_ttl_seconds"`
	LuoguSyncUserIntervalSeconds          int  `json:"luogu_sync_user_interval_seconds" yaml:"luogu_sync_user_interval_seconds"`
	RankingSyncIntervalSeconds            int  `json:"ranking_sync_interval_seconds" yaml:"ranking_sync_interval_seconds"`
}
