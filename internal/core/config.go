package core

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// InitConfig 初始化配置 - 以环境变量优先
func InitConfig(path string) {
	// 条件初始化
	viper.AllowEmptyEnv(true)
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetDefault("system.auto_migrate", true)
	// 读取并监听配置
	viper.WatchConfig() // 监视配置文件的更改
	viper.AddConfigPath(path)
	viper.SetConfigType("yaml")
	viper.SetConfigName("configs")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			global.Log.Warn("配置文件未找到，将仅使用环境变量", zap.Error(err))
		} else {
			global.Log.Fatal("配置文件加载失败", zap.Error(err))
		}
	}

	// 绑定验证码相关配置到环境变量
	_ = viper.BindEnv("captcha.height", "CAPTCHA_HEIGHT")
	_ = viper.BindEnv("captcha.width", "CAPTCHA_WIDTH")
	_ = viper.BindEnv("captcha.length", "CAPTCHA_LENGTH")
	_ = viper.BindEnv("captcha.max_skew", "CAPTCHA_MAX_SKEW")
	_ = viper.BindEnv("captcha.dot_count", "CAPTCHA_DOT_COUNT")

	// 绑定邮件相关配置到环境变量
	_ = viper.BindEnv("email.host", "EMAIL_HOST")
	_ = viper.BindEnv("email.port", "EMAIL_PORT")
	_ = viper.BindEnv("email.from", "EMAIL_FROM")
	_ = viper.BindEnv("email.nickname", "EMAIL_NICKNAME")
	_ = viper.BindEnv("email.secret", "EMAIL_SECRET")
	_ = viper.BindEnv("email.is_ssl", "EMAIL_IS_SSL")

	// 绑定存储驱动相关配置到环境变量
	_ = viper.BindEnv("storage.current", "STORAGE_CURRENT")
	_ = viper.BindEnv("storage.local.base_url", "STORAGE_LOCAL_BASE_URL")
	_ = viper.BindEnv("storage.local.key_prefix", "STORAGE_LOCAL_KEY_PREFIX")
	_ = viper.BindEnv("crawler.leetcode.base_url", "CRAWLER_LEETCODE_BASE_URL", "LEETCODE_BASE_URL")
	_ = viper.BindEnv("crawler.leetcode.timeout_ms", "LEETCODE_TIMEOUT_MS")
	_ = viper.BindEnv("crawler.leetcode.max_idle_conns", "LEETCODE_MAX_IDLE_CONNS")
	_ = viper.BindEnv("crawler.leetcode.max_idle_conns_per_host", "LEETCODE_MAX_IDLE_CONNS_PER_HOST")
	_ = viper.BindEnv("crawler.leetcode.idle_conn_timeout_sec", "LEETCODE_IDLE_CONN_TIMEOUT_SEC")
	_ = viper.BindEnv("crawler.leetcode.retry_count", "LEETCODE_RETRY_COUNT")
	_ = viper.BindEnv("crawler.leetcode.retry_wait_ms", "LEETCODE_RETRY_WAIT_MS")
	_ = viper.BindEnv("crawler.leetcode.retry_max_wait_ms", "LEETCODE_RETRY_MAX_WAIT_MS")
	_ = viper.BindEnv("crawler.leetcode.response_body_limit_bytes", "LEETCODE_RESPONSE_BODY_LIMIT_BYTES")
	_ = viper.BindEnv("crawler.luogu.base_url", "CRAWLER_LUOGU_BASE_URL", "LUOGU_BASE_URL")
	_ = viper.BindEnv("crawler.luogu.timeout_ms", "LUOGU_TIMEOUT_MS")
	_ = viper.BindEnv("crawler.luogu.max_idle_conns", "LUOGU_MAX_IDLE_CONNS")
	_ = viper.BindEnv("crawler.luogu.max_idle_conns_per_host", "LUOGU_MAX_IDLE_CONNS_PER_HOST")
	_ = viper.BindEnv("crawler.luogu.idle_conn_timeout_sec", "LUOGU_IDLE_CONN_TIMEOUT_SEC")
	_ = viper.BindEnv("crawler.luogu.retry_count", "LUOGU_RETRY_COUNT")
	_ = viper.BindEnv("crawler.luogu.retry_wait_ms", "LUOGU_RETRY_WAIT_MS")
	_ = viper.BindEnv("crawler.luogu.retry_max_wait_ms", "LUOGU_RETRY_MAX_WAIT_MS")
	_ = viper.BindEnv("crawler.luogu.response_body_limit_bytes", "LUOGU_RESPONSE_BODY_LIMIT_BYTES")
	_ = viper.BindEnv("storage.qiniu.bucket", "STORAGE_QINIU_BUCKET")
	_ = viper.BindEnv("storage.qiniu.domain", "STORAGE_QINIU_DOMAIN")
	_ = viper.BindEnv("storage.qiniu.key_prefix", "STORAGE_QINIU_KEY_PREFIX")
	_ = viper.BindEnv("storage.qiniu.access_key", "STORAGE_QINIU_ACCESS_KEY")
	_ = viper.BindEnv("storage.qiniu.secret_key", "STORAGE_QINIU_SECRET_KEY")

	// 绑定静态文件相关配置到环境变量
	_ = viper.BindEnv("static.path", "STATIC_PATH")
	_ = viper.BindEnv("static.prefix", "STATIC_PREFIX")
	_ = viper.BindEnv("static.max_size", "STATIC_MAX_SIZE")
	_ = viper.BindEnv("static.max_uploads", "STATIC_MAX_UPLOADS")

	// 绑定高德地图相关配置到环境变量
	_ = viper.BindEnv("gaode.enable", "GAODE_ENABLE")
	_ = viper.BindEnv("gaode.key", "GAODE_KEY")

	// 绑定JWT相关配置到环境变量
	_ = viper.BindEnv("jwt.access_token_secret", "JWT_ACCESS_TOKEN_SECRET")
	_ = viper.BindEnv("jwt.refresh_token_secret", "JWT_REFRESH_TOKEN_SECRET")
	_ = viper.BindEnv("jwt.access_token_expiry_time", "JWT_ACCESS_TOKEN_EXPIRY_TIME")
	_ = viper.BindEnv("jwt.refresh_token_expiry_time", "JWT_REFRESH_TOKEN_EXPIRY_TIME")
	_ = viper.BindEnv("jwt.issuer", "JWT_ISSUER")

	// 绑定MySQL相关配置到环境变量（补充完整）
	_ = viper.BindEnv("mysql.host", "DB_HOST")
	_ = viper.BindEnv("mysql.port", "DB_PORT")
	_ = viper.BindEnv("mysql.config", "DB_CONFIG")
	_ = viper.BindEnv("mysql.db_name", "DB_NAME")
	_ = viper.BindEnv("mysql.username", "DB_USERNAME")
	_ = viper.BindEnv("mysql.password", "DB_PASSWORD")
	_ = viper.BindEnv("mysql.max_idle_conns", "DB_MAX_IDLE_CONNS")
	_ = viper.BindEnv("mysql.max_open_conns", "DB_MAX_OPEN_CONNS")
	_ = viper.BindEnv("mysql.log_mode", "DB_LOG_MODE")

	// 旧版 qiniu.* 环境变量绑定已废弃，统一使用 storage.qiniu.*

	// 绑定QQ登录相关配置到环境变量
	_ = viper.BindEnv("qq.enable", "QQ_ENABLE")
	_ = viper.BindEnv("qq.app_id", "QQ_APP_ID")
	_ = viper.BindEnv("qq.app_key", "QQ_APP_KEY")
	_ = viper.BindEnv("qq.redirect_uri", "QQ_REDIRECT_URI")

	// 绑定Redis相关配置到环境变量（补充完整）
	_ = viper.BindEnv("redis.address", "REDIS_ADDRESS")
	_ = viper.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = viper.BindEnv("redis.db", "REDIS_DB")

	_ = viper.BindEnv("task.outbox_cleanup_retention_days", "TASK_OUTBOX_CLEANUP_RETENTION_DAYS")
	_ = viper.BindEnv("task.luogu_question_bank_warmup_enabled", "TASK_LUOGU_QUESTION_BANK_WARMUP_ENABLED")
	_ = viper.BindEnv("task.luogu_question_bank_warmup_batch_size", "TASK_LUOGU_QUESTION_BANK_WARMUP_BATCH_SIZE")
	_ = viper.BindEnv("task.luogu_question_bank_warmup_lock_ttl_seconds", "TASK_LUOGU_QUESTION_BANK_WARMUP_LOCK_TTL_SECONDS")
	_ = viper.BindEnv("task.ranking_sync_interval_seconds", "TASK_RANKING_SYNC_INTERVAL_SECONDS")
	_ = viper.BindEnv("messaging.redis_stream_read_count", "MESSAGING_REDIS_STREAM_READ_COUNT")
	_ = viper.BindEnv("messaging.redis_stream_block_ms", "MESSAGING_REDIS_STREAM_BLOCK_MS")
	_ = viper.BindEnv("messaging.luogu_bind_topic", "MESSAGING_LUOGU_BIND_TOPIC")
	_ = viper.BindEnv("messaging.luogu_bind_group", "MESSAGING_LUOGU_BIND_GROUP")
	_ = viper.BindEnv("messaging.luogu_bind_consumer", "MESSAGING_LUOGU_BIND_CONSUMER")

	// 绑定系统服务相关配置到环境变量（补充完整）
	_ = viper.BindEnv("system.host", "SYSTEM_HOST")
	_ = viper.BindEnv("system.port", "SYSTEM_PORT", "PORT")
	_ = viper.BindEnv("system.env", "SYSTEM_ENV")
	_ = viper.BindEnv("system.router_prefix", "SYSTEM_ROUTER_PREFIX")
	_ = viper.BindEnv("system.auto_migrate", "AUTO_MIGRATE")
	_ = viper.BindEnv("system.use_multipoint", "SYSTEM_USE_MULTIPOINT")
	_ = viper.BindEnv("system.sessions_secret", "SYSTEM_SESSIONS_SECRET")
	// 已简化：不再绑定 system.oss_type，统一使用 storage.current 控制驱动

	// 绑定文件上传相关配置到环境变量
	_ = viper.BindEnv("upload.size", "UPLOAD_SIZE")
	_ = viper.BindEnv("upload.path", "UPLOAD_PATH")

	// 绑定网站信息相关配置到环境变量
	_ = viper.BindEnv("website.logo", "WEBSITE_LOGO")
	_ = viper.BindEnv("website.full_logo", "WEBSITE_FULL_LOGO")
	_ = viper.BindEnv("website.title", "WEBSITE_TITLE")
	_ = viper.BindEnv("website.slogan", "WEBSITE_SLOGAN")
	_ = viper.BindEnv("website.slogan_en", "WEBSITE_SLOGAN_EN")
	_ = viper.BindEnv("website.description", "WEBSITE_DESCRIPTION")
	_ = viper.BindEnv("website.version", "WEBSITE_VERSION")
	_ = viper.BindEnv("website.created_at", "WEBSITE_CREATED_AT")
	_ = viper.BindEnv("website.icp_filing", "WEBSITE_ICP_FILING")
	_ = viper.BindEnv("website.public_security_filing", "WEBSITE_PUBLIC_SECURITY_FILING")
	_ = viper.BindEnv("website.bilibili_url", "WEBSITE_BILIBILI_URL")
	_ = viper.BindEnv("website.gitee_url", "WEBSITE_GITEE_URL")
	_ = viper.BindEnv("website.github_url", "WEBSITE_GITHUB_URL")
	_ = viper.BindEnv("website.blog_url", "WEBSITE_BLOG_URL")
	_ = viper.BindEnv("website.name", "WEBSITE_NAME")
	_ = viper.BindEnv("website.job", "WEBSITE_JOB")
	_ = viper.BindEnv("website.address", "WEBSITE_ADDRESS")
	_ = viper.BindEnv("website.email", "WEBSITE_EMAIL")
	_ = viper.BindEnv("website.qq_image", "WEBSITE_QQ_IMAGE")
	_ = viper.BindEnv("website.wechat_image", "WEBSITE_WECHAT_IMAGE")

	// 绑定Zap日志相关配置到环境变量
	_ = viper.BindEnv("zap.level", "ZAP_LEVEL")
	_ = viper.BindEnv("zap.filename", "ZAP_FILENAME")
	_ = viper.BindEnv("zap.max_size", "ZAP_MAX_SIZE")
	_ = viper.BindEnv("zap.max_backups", "ZAP_MAX_BACKUPS")
	_ = viper.BindEnv("zap.max_age", "ZAP_MAX_AGE")
	_ = viper.BindEnv("zap.is_console_print", "ZAP_IS_CONSOLE_PRINT")

	_ = viper.BindEnv("messaging.redis_stream_read_count", "MESSAGING_REDIS_STREAM_READ_COUNT")
	_ = viper.BindEnv("messaging.redis_stream_block_ms", "MESSAGING_REDIS_STREAM_BLOCK_MS")
	_ = viper.BindEnv("messaging.luogu_bind_topic", "MESSAGING_LUOGU_BIND_TOPIC")
	_ = viper.BindEnv("messaging.luogu_bind_group", "MESSAGING_LUOGU_BIND_GROUP")
	_ = viper.BindEnv("messaging.luogu_bind_consumer", "MESSAGING_LUOGU_BIND_CONSUMER")
	_ = viper.BindEnv("messaging.leetcode_bind_topic", "MESSAGING_LEETCODE_BIND_TOPIC")
	_ = viper.BindEnv("messaging.leetcode_bind_group", "MESSAGING_LEETCODE_BIND_GROUP")
	_ = viper.BindEnv("messaging.leetcode_bind_consumer", "MESSAGING_LEETCODE_BIND_CONSUMER")

	global.Log.Info("--------- configs list--------\n")
	for _, key := range viper.AllKeys() {
		global.Log.Info("configs",
			zap.String("key", key),
			zap.Any("value", viper.Get(key)))
	}
	global.Log.Info("-----------------------------\n")
	// 传递到全局
	global.Config = config.NewConfig()
}
