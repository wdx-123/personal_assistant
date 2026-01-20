package config

import "github.com/spf13/viper"

// Config 应用全局配置结构体，包含所有核心模块配置
type Config struct {
	Redis     Redis     `json:"redis" yaml:"redis"`     // Redis配置
	Mysql     Mysql     `json:"mysql" yaml:"mysql"`     // MySQL数据库配置
	System    System    `json:"system" yaml:"system"`   // 系统服务配置
	Zap       Zap       `json:"zap" yaml:"zap"`         // 日志配置
	JWT       JWT       `json:"jwt" yaml:"jwt"`         // JWT认证配置
	Upload    Upload    `json:"upload" yaml:"upload"`   // 文件上传配置
	Captcha   Captcha   `json:"captcha" yaml:"captcha"` // 验证码配置
	Email     Email     `json:"email" yaml:"email"`     // 邮件发送配置
	Gaode     Gaode     `json:"gaode" yaml:"gaode"`     // 高德地图API配置
	Website   Website   `json:"website" yaml:"website"` // 个人网站配置
	Storage   Storage   `json:"storage" yaml:"storage"` // 存储驱动配置
	Static    Static    `json:"static" yaml:"static"`   // 静态文件配置
	Crawler   Crawler   `json:"crawler" yaml:"crawler"`
	Task      Task      `json:"task" yaml:"task"`           // 定时任务配置
	Messaging Messaging `json:"messaging" yaml:"messaging"` // 消息队列配置
}

func NewConfig() *Config {
	// Redis配置初始化
	_redis := &Redis{
		Address:  viper.GetString("redis.address"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
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
		Path:         viper.GetString("static.path"),
		Prefix:       viper.GetString("static.prefix"),
		MaxSize:      viper.GetInt("static.max_size"),
		MaxUploads:   viper.GetInt("static.max_uploads"),
		AllowedTypes: viper.GetStringSlice("static.allowed_types"),
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
			TimeoutMs:              viper.GetInt("crawler.luogu.timeout_ms"),
			MaxIdleConns:           viper.GetInt("crawler.luogu.max_idle_conns"),
			MaxIdleConnsPerHost:    viper.GetInt("crawler.luogu.max_idle_conns_per_host"),
			IdleConnTimeoutSec:     viper.GetInt("crawler.luogu.idle_conn_timeout_sec"),
			RetryCount:             viper.GetInt("crawler.luogu.retry_count"),
			RetryWaitMs:            viper.GetInt("crawler.luogu.retry_wait_ms"),
			RetryMaxWaitMs:         viper.GetInt("crawler.luogu.retry_max_wait_ms"),
			ResponseBodyLimitBytes: viper.GetInt64("crawler.luogu.response_body_limit_bytes"),
		},
	}

	_task := &Task{
		OutboxCleanupRetentionDays:            viper.GetInt("task.outbox_cleanup_retention_days"),
		LuoguQuestionBankWarmupEnabled:        viper.GetBool("task.luogu_question_bank_warmup_enabled"),
		LuoguQuestionBankWarmupBatchSize:      viper.GetInt("task.luogu_question_bank_warmup_batch_size"),
		LuoguQuestionBankWarmupLockTTLSeconds: viper.GetInt("task.luogu_question_bank_warmup_lock_ttl_seconds"),
		LuoguSyncUserIntervalSeconds:          viper.GetInt("task.luogu_sync_user_interval_seconds"),
		RankingSyncIntervalSeconds:            viper.GetInt("task.ranking_sync_interval_seconds"),
	}

	_messaging := &Messaging{
		RedisStreamReadCount: viper.GetInt("messaging.redis_stream_read_count"),
		RedisStreamBlockMs:   viper.GetInt("messaging.redis_stream_block_ms"),
		LuoguBindTopic:       viper.GetString("messaging.luogu_bind_topic"),
		LuoguBindGroup:       viper.GetString("messaging.luogu_bind_group"),
		LuoguBindConsumer:    viper.GetString("messaging.luogu_bind_consumer"),
	}

	return &Config{
		Redis:     *_redis,
		Mysql:     *_mysql,
		System:    *_system,
		Zap:       *_zap,
		JWT:       *_jwt,
		Upload:    *_upload,
		Captcha:   *_captcha,
		Email:     *_email,
		Storage:   *_storage,
		Static:    *_static,
		Gaode:     *_gaode,
		Website:   *_website,
		Crawler:   *_crawler,
		Task:      *_task,
		Messaging: *_messaging,
	}
}
