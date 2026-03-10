package config

// Task 定时任务配置
type Task struct {
	OutboxCleanupRetentionDays int `json:"outbox_cleanup_retention_days" yaml:"outbox_cleanup_retention_days"`
	// OutboxFailedCleanupRetentionDays 失败消息保留更久，便于排查和补救
	OutboxFailedCleanupRetentionDays int `json:"outbox_failed_cleanup_retention_days" yaml:"outbox_failed_cleanup_retention_days"`

	// DistributedLockEnabled 是否启用分布式锁来协调定时任务，防止多实例重复执行
	DistributedLockEnabled bool `json:"distributed_lock_enabled" yaml:"distributed_lock_enabled"`
	// DistributedLockTTLSeconds 分布式锁 TTL，单位秒，需略大于单次任务执行预估时间，防止死锁
	DistributedLockTTLSeconds int `json:"distributed_lock_ttl_seconds" yaml:"distributed_lock_ttl_seconds"`

	LuoguQuestionBankWarmupEnabled           bool   `json:"luogu_question_bank_warmup_enabled" yaml:"luogu_question_bank_warmup_enabled"`
	LuoguQuestionBankWarmupBatchSize         int    `json:"luogu_question_bank_warmup_batch_size" yaml:"luogu_question_bank_warmup_batch_size"`
	LuoguQuestionBankWarmupLockTTLSeconds    int    `json:"luogu_question_bank_warmup_lock_ttl_seconds" yaml:"luogu_question_bank_warmup_lock_ttl_seconds"`
	LeetcodeQuestionBankWarmupEnabled        bool   `json:"leetcode_question_bank_warmup_enabled" yaml:"leetcode_question_bank_warmup_enabled"`
	LeetcodeQuestionBankWarmupBatchSize      int    `json:"leetcode_question_bank_warmup_batch_size" yaml:"leetcode_question_bank_warmup_batch_size"`
	LeetcodeQuestionBankWarmupLockTTLSeconds int    `json:"leetcode_question_bank_warmup_lock_ttl_seconds" yaml:"leetcode_question_bank_warmup_lock_ttl_seconds"`
	LuoguSyncUserIntervalSeconds             int    `json:"luogu_sync_user_interval_seconds" yaml:"luogu_sync_user_interval_seconds"`       // 洛谷用户间隔秒数
	LeetcodeSyncUserIntervalSeconds          int    `json:"leetcode_sync_user_interval_seconds" yaml:"leetcode_sync_user_interval_seconds"` // 力扣用户间隔秒数
	LeetcodeSyncIntervalSeconds              int    `json:"leetcode_sync_interval_seconds" yaml:"leetcode_sync_interval_seconds"`           // 力扣全量同步间隔秒数
	RankingSyncIntervalSeconds               int    `json:"ranking_sync_interval_seconds" yaml:"ranking_sync_interval_seconds"`             // 排行榜同步间隔秒数
	ImageOrphanCleanupCron                   string `json:"image_orphan_cleanup_cron" yaml:"image_orphan_cleanup_cron"`                     // 孤儿图片清理 cron 表达式，默认 @daily

	// DisabledUserCleanupEnabled 是否启用禁用账号清理任务
	DisabledUserCleanupEnabled bool `json:"disabled_user_cleanup_enabled" yaml:"disabled_user_cleanup_enabled"` // 是否启用禁用账号清理

	// DisabledUserRetentionDays 禁用账号保留天数，超过该天数的禁用账号将被清理
	DisabledUserRetentionDays int `json:"disabled_user_retention_days" yaml:"disabled_user_retention_days"` // 禁用账号保留天数

	// DisabledUserCleanupBatchSize 每次清理批次大小，避免一次性处理过多账号导致性能问题
	DisabledUserCleanupCron string `json:"disabled_user_cleanup_cron" yaml:"disabled_user_cleanup_cron"` // 禁用账号清理 cron
}
