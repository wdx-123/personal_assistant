package config

// Task 定时任务配置
type Task struct {
	OutboxCleanupRetentionDays               int    `json:"outbox_cleanup_retention_days" yaml:"outbox_cleanup_retention_days"`
	LuoguQuestionBankWarmupEnabled           bool   `json:"luogu_question_bank_warmup_enabled" yaml:"luogu_question_bank_warmup_enabled"`
	LuoguQuestionBankWarmupBatchSize         int    `json:"luogu_question_bank_warmup_batch_size" yaml:"luogu_question_bank_warmup_batch_size"`
	LuoguQuestionBankWarmupLockTTLSeconds    int    `json:"luogu_question_bank_warmup_lock_ttl_seconds" yaml:"luogu_question_bank_warmup_lock_ttl_seconds"`
	LeetcodeQuestionBankWarmupEnabled        bool   `json:"leetcode_question_bank_warmup_enabled" yaml:"leetcode_question_bank_warmup_enabled"`
	LeetcodeQuestionBankWarmupBatchSize      int    `json:"leetcode_question_bank_warmup_batch_size" yaml:"leetcode_question_bank_warmup_batch_size"`
	LeetcodeQuestionBankWarmupLockTTLSeconds int    `json:"leetcode_question_bank_warmup_lock_ttl_seconds" yaml:"leetcode_question_bank_warmup_lock_ttl_seconds"`
	LuoguSyncUserIntervalSeconds             int    `json:"luogu_sync_user_interval_seconds" yaml:"luogu_sync_user_interval_seconds"`             // 洛谷用户间隔秒数
	LeetcodeSyncUserIntervalSeconds          int    `json:"leetcode_sync_user_interval_seconds" yaml:"leetcode_sync_user_interval_seconds"`       // 力扣用户间隔秒数
	LeetcodeSyncIntervalSeconds              int    `json:"leetcode_sync_interval_seconds" yaml:"leetcode_sync_interval_seconds"`                 // 力扣全量同步间隔秒数
	RankingSyncIntervalSeconds               int    `json:"ranking_sync_interval_seconds" yaml:"ranking_sync_interval_seconds"`                   // 排行榜同步间隔秒数
	ImageOrphanCleanupCron                   string `json:"image_orphan_cleanup_cron" yaml:"image_orphan_cleanup_cron"`                           // 孤儿图片清理 cron 表达式，默认 @daily
}
