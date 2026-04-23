package config

import "github.com/spf13/viper"

// Config 应用全局配置结构体，包含所有核心模块配置
type Config struct {
	Redis         Redis         `json:"redis" yaml:"redis"`   // Redis配置
	Mysql         Mysql         `json:"mysql" yaml:"mysql"`   // MySQL数据库配置
	System        System        `json:"system" yaml:"system"` // 系统服务配置
	Security      Security      `json:"security" yaml:"security"`
	Zap           Zap           `json:"zap" yaml:"zap"`         // 日志配置
	JWT           JWT           `json:"jwt" yaml:"jwt"`         // JWT认证配置
	Upload        Upload        `json:"upload" yaml:"upload"`   // 文件上传配置
	Captcha       Captcha       `json:"captcha" yaml:"captcha"` // 验证码配置
	Email         Email         `json:"email" yaml:"email"`     // 邮件发送配置
	Gaode         Gaode         `json:"gaode" yaml:"gaode"`     // 高德地图API配置
	Website       Website       `json:"website" yaml:"website"` // 个人网站配置
	Storage       Storage       `json:"storage" yaml:"storage"` // 存储驱动配置
	Static        Static        `json:"static" yaml:"static"`   // 静态文件配置
	Crawler       Crawler       `json:"crawler" yaml:"crawler"`
	Task          Task          `json:"task" yaml:"task"`                   // 定时任务配置
	Messaging     Messaging     `json:"messaging" yaml:"messaging"`         // 消息队列配置
	SSE           SSE           `json:"sse" yaml:"sse"`                     // SSE 实时推送配置
	AI            AI            `json:"ai" yaml:"ai"`                       // AI Runtime / Eino 配置
	Qdrant        Qdrant        `json:"qdrant" yaml:"qdrant"`               // Qdrant 向量数据库配置
	RateLimit     RateLimit     `json:"rate_limit" yaml:"rate_limit"`       // 限流配置
	Observability Observability `json:"observability" yaml:"observability"` // 观测基础设施配置
}

// NewConfig 负责创建并返回当前对象所需的实例。
// 参数：
//   - 无。
//
// 返回值：
//   - *Config：当前函数返回的目标对象；失败时可能为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func NewConfig() *Config {
	// Redis配置初始化
	_redis := &Redis{
		Address:                         viper.GetString("redis.address"),
		Password:                        viper.GetString("redis.password"),
		DB:                              viper.GetInt("redis.db"),
		ActiveUserStateTTLSeconds:       viper.GetInt("redis.active_user_state_ttl_seconds"),
		ActiveUserStateTTLJitterSeconds: viper.GetInt("redis.active_user_state_ttl_jitter_seconds"),
	}
	// MySQL数据库配置初始化
	_mysql := &Mysql{
		Host:         viper.GetString("mysql.host"),
		Port:         viper.GetInt("mysql.port"),
		Config:       viper.GetString("mysql.config"),
		DBName:       viper.GetString("mysql.db_name"),
		Username:     viper.GetString("mysql.username"),
		Password:     viper.GetString("mysql.password"),
		MaxIdleConns: viper.GetInt("mysql.max_idle_conns"),
		MaxOpenConns: viper.GetInt("mysql.max_open_conns"),
		LogMode:      viper.GetString("mysql.log_mode"),
	}
	// 系统服务配置初始化
	_system := &System{
		Host:           viper.GetString("system.host"),
		Port:           viper.GetInt("system.port"),
		Env:            viper.GetString("system.env"),
		RouterPrefix:   viper.GetString("system.router_prefix"),
		AutoMigrate:    viper.GetBool("system.auto_migrate"),
		UseMultipoint:  viper.GetBool("system.use_multipoint"),
		SessionsSecret: viper.GetString("system.sessions_secret"),

		// 角色配置
		DefaultRoleCode: viper.GetString("system.default_role_code"),
		DefaultRoleName: viper.GetString("system.default_role_name"),

		// 业务逻辑配置
		BindCoolDownHours: viper.GetInt("system.bind_cool_down_hours"),
	}
	_security := &Security{
		SensitiveData: SensitiveData{
			Enabled:       viper.GetBool("security.sensitive_data.enabled"),
			CipherPrefix:  viper.GetString("security.sensitive_data.cipher_prefix"),
			AESKeyBase64:  viper.GetString("security.sensitive_data.aes_key_base64"),
			HashKeyBase64: viper.GetString("security.sensitive_data.hash_key_base64"),
		},
	}
	// 日志配置初始化
	_zap := &Zap{
		Level:          viper.GetString("zap.level"),
		Filename:       viper.GetString("zap.filename"),
		MaxSize:        viper.GetInt("zap.max_size"),
		MaxBackups:     viper.GetInt("zap.max_backups"),
		MaxAge:         viper.GetInt("zap.max_age"),
		IsConsolePrint: viper.GetBool("zap.is_console_print"),
	}
	// JWT认证配置初始化
	_jwt := &JWT{
		AccessTokenSecret:      viper.GetString("jwt.access_token_secret"),
		RefreshTokenSecret:     viper.GetString("jwt.refresh_token_secret"),
		AccessTokenExpiryTime:  viper.GetString("jwt.access_token_expiry_time"),
		RefreshTokenExpiryTime: viper.GetString("jwt.refresh_token_expiry_time"),
		Issuer:                 viper.GetString("jwt.issuer"),
	}
	// 文件上传配置初始化
	_upload := &Upload{
		Size: viper.GetInt("upload.size"),
		Path: viper.GetString("upload.path"),
	}
	// 验证码配置初始化
	_captcha := &Captcha{
		Height:   viper.GetInt("captcha.height"),
		Width:    viper.GetInt("captcha.width"),
		Length:   viper.GetInt("captcha.length"),
		MaxSkew:  viper.GetFloat64("captcha.max_skew"),
		DotCount: viper.GetInt("captcha.dot_count"),
	}
	// 邮件发送配置初始化
	_email := &Email{
		Host:     viper.GetString("email.host"),
		Port:     viper.GetInt("email.port"),
		From:     viper.GetString("email.from"),
		Nickname: viper.GetString("email.nickname"),
		Secret:   viper.GetString("email.secret"),
		IsSSL:    viper.GetBool("email.is_ssl"),
	}
	// 存储驱动配置初始化
	_storage := &Storage{
		Current: viper.GetString("storage.current"),
		Local: StorageLocal{
			BaseURL:   viper.GetString("storage.local.base_url"),
			KeyPrefix: viper.GetString("storage.local.key_prefix"),
		},
		Qiniu: StorageQiniu{
			Bucket:    viper.GetString("storage.qiniu.bucket"),
			Domain:    viper.GetString("storage.qiniu.domain"),
			KeyPrefix: viper.GetString("storage.qiniu.key_prefix"),
			AccessKey: viper.GetString("storage.qiniu.access_key"),
			SecretKey: viper.GetString("storage.qiniu.secret_key"),
		},
	}
	// 静态文件配置初始化
	_static := &Static{
		Path:                 viper.GetString("static.path"),
		Prefix:               viper.GetString("static.prefix"),
		MaxSize:              viper.GetInt("static.max_size"),
		MaxUploads:           viper.GetInt("static.max_uploads"),
		AllowedTypes:         viper.GetStringSlice("static.allowed_types"),
		MaxConcurrentUploads: viper.GetInt("static.max_concurrent_uploads"),
		UserQuotaMB:          viper.GetInt("static.user_quota_mb"),
	}
	// 高德地图API配置初始化
	_gaode := &Gaode{
		Enable: viper.GetBool("gaode.enable"),
		Key:    viper.GetString("gaode.key"),
	}
	// 个人网站配置初始化
	_website := &Website{
		Logo:                 viper.GetString("website.logo"),
		FullLogo:             viper.GetString("website.full_logo"),
		Title:                viper.GetString("website.title"),
		Slogan:               viper.GetString("website.slogan"),
		SloganEn:             viper.GetString("website.slogan_en"),
		Description:          viper.GetString("website.description"),
		Version:              viper.GetString("website.version"),
		CreatedAt:            viper.GetString("website.created_at"),
		IcpFiling:            viper.GetString("website.icp_filing"),
		PublicSecurityFiling: viper.GetString("website.public_security_filing"),
		BilibiliUrl:          viper.GetString("website.bilibili_url"),
		GiteeUrl:             viper.GetString("website.gitee_url"),
		GithubUrl:            viper.GetString("website.github_url"),
		BlogUrl:              viper.GetString("website.blog_url"),
		Name:                 viper.GetString("website.name"),
		Job:                  viper.GetString("website.job"),
		Address:              viper.GetString("website.address"),
		Email:                viper.GetString("website.email"),
		QqImage:              viper.GetString("website.qq_image"),
		WechatImage:          viper.GetString("website.wechat_image"),
	}

	_crawler := &Crawler{
		LeetCode: LeetCodeCrawler{
			BaseURL:                viper.GetString("crawler.leetcode.base_url"),
			APIPrefix:              getCrawlerAPIPrefix("crawler.leetcode.api_prefix"),
			TimeoutMs:              viper.GetInt("crawler.leetcode.timeout_ms"),
			MaxIdleConns:           viper.GetInt("crawler.leetcode.max_idle_conns"),
			MaxIdleConnsPerHost:    viper.GetInt("crawler.leetcode.max_idle_conns_per_host"),
			IdleConnTimeoutSec:     viper.GetInt("crawler.leetcode.idle_conn_timeout_sec"),
			RetryCount:             viper.GetInt("crawler.leetcode.retry_count"),
			RetryWaitMs:            viper.GetInt("crawler.leetcode.retry_wait_ms"),
			RetryMaxWaitMs:         viper.GetInt("crawler.leetcode.retry_max_wait_ms"),
			ResponseBodyLimitBytes: viper.GetInt64("crawler.leetcode.response_body_limit_bytes"),
		},
		Luogu: LuoguCrawler{
			BaseURL:                viper.GetString("crawler.luogu.base_url"),
			APIPrefix:              getCrawlerAPIPrefix("crawler.luogu.api_prefix"),
			TimeoutMs:              viper.GetInt("crawler.luogu.timeout_ms"),
			MaxIdleConns:           viper.GetInt("crawler.luogu.max_idle_conns"),
			MaxIdleConnsPerHost:    viper.GetInt("crawler.luogu.max_idle_conns_per_host"),
			IdleConnTimeoutSec:     viper.GetInt("crawler.luogu.idle_conn_timeout_sec"),
			RetryCount:             viper.GetInt("crawler.luogu.retry_count"),
			RetryWaitMs:            viper.GetInt("crawler.luogu.retry_wait_ms"),
			RetryMaxWaitMs:         viper.GetInt("crawler.luogu.retry_max_wait_ms"),
			ResponseBodyLimitBytes: viper.GetInt64("crawler.luogu.response_body_limit_bytes"),
		},
		Lanqiao: LanqiaoCrawler{
			BaseURL:                viper.GetString("crawler.lanqiao.base_url"),
			APIPrefix:              getCrawlerAPIPrefix("crawler.lanqiao.api_prefix"),
			TimeoutMs:              viper.GetInt("crawler.lanqiao.timeout_ms"),
			MaxIdleConns:           viper.GetInt("crawler.lanqiao.max_idle_conns"),
			MaxIdleConnsPerHost:    viper.GetInt("crawler.lanqiao.max_idle_conns_per_host"),
			IdleConnTimeoutSec:     viper.GetInt("crawler.lanqiao.idle_conn_timeout_sec"),
			RetryCount:             viper.GetInt("crawler.lanqiao.retry_count"),
			RetryWaitMs:            viper.GetInt("crawler.lanqiao.retry_wait_ms"),
			RetryMaxWaitMs:         viper.GetInt("crawler.lanqiao.retry_max_wait_ms"),
			ResponseBodyLimitBytes: viper.GetInt64("crawler.lanqiao.response_body_limit_bytes"),
		},
	}

	_task := &Task{
		OutboxCleanupRetentionDays:            viper.GetInt("task.outbox_cleanup_retention_days"),
		OutboxFailedCleanupRetentionDays:      viper.GetInt("task.outbox_failed_cleanup_retention_days"),
		DistributedLockEnabled:                viper.GetBool("task.distributed_lock_enabled"),
		DistributedLockTTLSeconds:             viper.GetInt("task.distributed_lock_ttl_seconds"),
		LuoguQuestionBankWarmupEnabled:        viper.GetBool("task.luogu_question_bank_warmup_enabled"),
		LuoguQuestionBankWarmupBatchSize:      viper.GetInt("task.luogu_question_bank_warmup_batch_size"),
		LuoguQuestionBankWarmupLockTTLSeconds: viper.GetInt("task.luogu_question_bank_warmup_lock_ttl_seconds"),
		LeetcodeQuestionBankWarmupEnabled:     viper.GetBool("task.leetcode_question_bank_warmup_enabled"),
		LeetcodeQuestionBankWarmupBatchSize:   viper.GetInt("task.leetcode_question_bank_warmup_batch_size"),
		LeetcodeQuestionBankWarmupLockTTLSeconds: viper.GetInt(
			"task.leetcode_question_bank_warmup_lock_ttl_seconds",
		),
		LuoguSyncUserIntervalSeconds:    viper.GetInt("task.luogu_sync_user_interval_seconds"),    // 读取洛谷用户间隔
		LeetcodeSyncUserIntervalSeconds: viper.GetInt("task.leetcode_sync_user_interval_seconds"), // 读取力扣用户间隔
		LeetcodeSyncIntervalSeconds:     viper.GetInt("task.leetcode_sync_interval_seconds"),      // 读取力扣全量间隔
		RankingSyncIntervalSeconds:      viper.GetInt("task.ranking_sync_interval_seconds"),       // 读取排行榜间隔
		OJDailyStatsRepairCron:          viper.GetString("task.oj_daily_stats_repair_cron"),
		OJDailyStatsRepairBatchSize:     viper.GetInt("task.oj_daily_stats_repair_batch_size"),
		OJDailyStatsRepairWindowDays:    viper.GetInt("task.oj_daily_stats_repair_window_days"),
		OJTaskDispatchEnabled:           viper.GetBool("task.oj_task_dispatch_enabled"),
		OJTaskDispatchIntervalSeconds:   viper.GetInt("task.oj_task_dispatch_interval_seconds"),
		OJTaskDispatchBatchSize:         viper.GetInt("task.oj_task_dispatch_batch_size"),
		OJTaskDispatchWorkerCount:       viper.GetInt("task.oj_task_dispatch_worker_count"),
		OJTaskSnapshotInsertBatchSize:   viper.GetInt("task.oj_task_snapshot_insert_batch_size"),
		OJTaskExecutionLockTTLSeconds:   viper.GetInt("task.oj_task_execution_lock_ttl_seconds"),
		ImageOrphanCleanupCron:          viper.GetString("task.image_orphan_cleanup_cron"),
		DisabledUserCleanupEnabled:      viper.GetBool("task.disabled_user_cleanup_enabled"),
		DisabledUserRetentionDays:       viper.GetInt("task.disabled_user_retention_days"),
		DisabledUserCleanupCron:         viper.GetString("task.disabled_user_cleanup_cron"),
	}

	// 限流配置初始化
	_rateLimit := &RateLimit{
		Upload: UploadRateLimit{
			GlobalLimit:     viper.GetInt("rate_limit.upload.global_limit"),
			GlobalWindowSec: viper.GetInt("rate_limit.upload.global_window_sec"),
			UserLimit:       viper.GetInt("rate_limit.upload.user_limit"),
			UserWindowSec:   viper.GetInt("rate_limit.upload.user_window_sec"),
		},
		OJBind: OJBindRateLimit{
			Limit:     viper.GetInt("rate_limit.oj_bind.limit"),
			WindowSec: viper.GetInt("rate_limit.oj_bind.window_sec"),
		},
	}

	_messaging := &Messaging{
		RedisStreamReadCount:        viper.GetInt("messaging.redis_stream_read_count"),
		RedisStreamBlockMs:          viper.GetInt("messaging.redis_stream_block_ms"),
		OutboxRelayLockEnabled:      viper.GetBool("messaging.outbox_relay_lock_enabled"),
		OutboxRelayLockTTLSeconds:   viper.GetInt("messaging.outbox_relay_lock_ttl_seconds"),
		LuoguBindTopic:              viper.GetString("messaging.luogu_bind_topic"),
		LuoguBindGroup:              viper.GetString("messaging.luogu_bind_group"),
		LuoguBindConsumer:           viper.GetString("messaging.luogu_bind_consumer"),
		LeetcodeBindTopic:           viper.GetString("messaging.leetcode_bind_topic"),
		LeetcodeBindGroup:           viper.GetString("messaging.leetcode_bind_group"),
		LeetcodeBindConsumer:        viper.GetString("messaging.leetcode_bind_consumer"),
		OJQuestionUpsertTopic:       viper.GetString("messaging.oj_question_upsert_topic"),
		OJQuestionUpsertGroup:       viper.GetString("messaging.oj_question_upsert_group"),
		OJQuestionUpsertConsumer:    viper.GetString("messaging.oj_question_upsert_consumer"),
		OJTaskExecutionTriggerTopic: viper.GetString("messaging.oj_task_execution_trigger_topic"),
		OJTaskExecutionTriggerGroup: viper.GetString("messaging.oj_task_execution_trigger_group"),
		OJTaskExecutionTriggerConsumer: viper.GetString(
			"messaging.oj_task_execution_trigger_consumer",
		),
		OJDailyStatsProjectionTopic: viper.GetString("messaging.oj_daily_stats_projection_topic"),
		OJDailyStatsProjectionGroup: viper.GetString("messaging.oj_daily_stats_projection_group"),
		OJDailyStatsProjectionConsumer: viper.GetString(
			"messaging.oj_daily_stats_projection_consumer",
		),
		CacheProjectionTopic:         viper.GetString("messaging.cache_projection_topic"),
		CacheProjectionGroup:         viper.GetString("messaging.cache_projection_group"),
		CacheProjectionConsumer:      viper.GetString("messaging.cache_projection_consumer"),
		PermissionProjectionTopic:    viper.GetString("messaging.permission_projection_topic"),
		PermissionProjectionGroup:    viper.GetString("messaging.permission_projection_group"),
		PermissionProjectionConsumer: viper.GetString("messaging.permission_projection_consumer"),
		PermissionPolicyReloadChannel: viper.GetString(
			"messaging.permission_policy_reload_channel",
		),
	}

	_sse := &SSE{
		HeartbeatIntervalSeconds: viper.GetInt("sse.heartbeat_interval_seconds"),
		WriteTimeoutSeconds:      viper.GetInt("sse.write_timeout_seconds"),
		QueueCapacity:            viper.GetInt("sse.queue_capacity"),
		MaxConnectionsPerSubject: viper.GetInt("sse.max_connections_per_subject"),
		ReplayLimit:              viper.GetInt("sse.replay_limit"),
		AllowedOrigins:           viper.GetStringSlice("sse.allowed_origins"),
		DrainTimeoutSeconds:      viper.GetInt("sse.drain_timeout_seconds"),
		PubSubChannelPrefix:      viper.GetString("sse.pubsub_channel_prefix"),
		ReplayStreamPrefix:       viper.GetString("sse.replay_stream_prefix"),
		AIRuntimeMode:            viper.GetString("sse.ai_runtime_mode"),
	}

	_ai := &AI{
		Provider:            viper.GetString("ai.provider"),
		APIKey:              viper.GetString("ai.api_key"),
		BaseURL:             viper.GetString("ai.base_url"),
		Model:               viper.GetString("ai.model"),
		ByAzure:             viper.GetBool("ai.by_azure"),
		APIVersion:          viper.GetString("ai.api_version"),
		SystemPrompt:        viper.GetString("ai.system_prompt"),
		Temperature:         viper.GetFloat64("ai.temperature"),
		MaxCompletionTokens: viper.GetInt("ai.max_completion_tokens"),
	}

	_qdrant := &Qdrant{
		Endpoint: viper.GetString("qdrant.endpoint"),
		APIKey:   viper.GetString("qdrant.api_key"),
	}

	_observability := &Observability{
		Enabled:     viper.GetBool("observability.enabled"),
		ServiceName: viper.GetString("observability.service_name"),
		ServiceTrace: ObservabilityServiceTrace{
			Enabled: viper.GetBool("observability.service_trace.enabled"),
			Modules: viper.GetStringSlice("observability.service_trace.modules"),
		},
		Propagation: ObservabilityPropagation{
			Enabled:         viper.GetBool("observability.propagation.enabled"),
			RequestIDHeader: viper.GetString("observability.propagation.request_id_header"),
			ParseW3C:        viper.GetBool("observability.propagation.parse_w3c"),
			InjectW3C:       viper.GetBool("observability.propagation.inject_w3c"),
		},
		Metrics: ObservabilityMetrics{
			FlushIntervalMs:   viper.GetInt("observability.metrics.flush_interval_ms"),
			DBBatchSize:       viper.GetInt("observability.metrics.db_batch_size"),
			FineRetentionDays: viper.GetInt("observability.metrics.fine_retention_days"),
			DayRetentionDays:  viper.GetInt("observability.metrics.day_retention_days"),
			WeekRetentionDays: viper.GetInt("observability.metrics.week_retention_days"),
			RollupCron:        viper.GetString("observability.metrics.rollup_cron"),
		},
		Traces: ObservabilityTraces{
			Enabled: viper.GetBool("observability.traces.enabled"),

			StreamKey:       viper.GetString("observability.traces.stream_key"),
			StreamGroup:     viper.GetString("observability.traces.stream_group"),
			StreamConsumer:  viper.GetString("observability.traces.stream_consumer"),
			StreamReadCount: viper.GetInt("observability.traces.stream_read_count"),
			StreamBlockMs:   viper.GetInt("observability.traces.stream_block_ms"),
			PendingIdleMs:   viper.GetInt("observability.traces.pending_idle_ms"),

			DBBatchSize:       viper.GetInt("observability.traces.db_batch_size"),
			DBFlushIntervalMs: viper.GetInt("observability.traces.db_flush_interval_ms"),

			NormalQueueSize:   viper.GetInt("observability.traces.normal_queue_size"),
			CriticalQueueSize: viper.GetInt("observability.traces.critical_queue_size"),
			EnqueueTimeoutMs:  viper.GetInt("observability.traces.enqueue_timeout_ms"),

			SuccessSampleRate:     viper.GetFloat64("observability.traces.success_sample_rate"),
			DropSuccessOnOverload: viper.GetBool("observability.traces.drop_success_on_overload"),
			CaptureErrorPayload:   viper.GetBool("observability.traces.capture_error_payload"),
			MaxPayloadBytes:       viper.GetInt("observability.traces.max_payload_bytes"),
			CaptureErrorStack:     viper.GetBool("observability.traces.capture_error_stack"),
			CaptureErrorDetail:    viper.GetBool("observability.traces.capture_error_detail"),
			MaxStackBytes:         viper.GetInt("observability.traces.max_stack_bytes"),
			MaxDetailBytes:        viper.GetInt("observability.traces.max_detail_bytes"),
			RedactKeys:            viper.GetStringSlice("observability.traces.redact_keys"),

			SuccessRetentionDays: viper.GetInt("observability.traces.success_retention_days"),
			ErrorRetentionDays:   viper.GetInt("observability.traces.error_retention_days"),
			CleanupCron:          viper.GetString("observability.traces.cleanup_cron"),
		},
	}

	return &Config{
		Redis:         *_redis,
		Mysql:         *_mysql,
		System:        *_system,
		Security:      *_security,
		Zap:           *_zap,
		JWT:           *_jwt,
		Upload:        *_upload,
		Captcha:       *_captcha,
		Email:         *_email,
		Storage:       *_storage,
		Static:        *_static,
		Gaode:         *_gaode,
		Website:       *_website,
		Crawler:       *_crawler,
		Task:          *_task,
		Messaging:     *_messaging,
		SSE:           *_sse,
		AI:            *_ai,
		Qdrant:        *_qdrant,
		RateLimit:     *_rateLimit,
		Observability: *_observability,
	}
}

// getCrawlerAPIPrefix 负责执行当前函数对应的核心逻辑。
// 参数：
//   - key：当前函数需要消费的输入参数。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func getCrawlerAPIPrefix(key string) string {
	if !viper.IsSet(key) {
		return "/v2"
	}
	return viper.GetString(key)
}
