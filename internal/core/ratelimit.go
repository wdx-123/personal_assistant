package core

import (
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/ratelimit"

	"go.uber.org/zap"
)

// InitUploadRateLimiters 初始化上传限流器并注册到全局变量
// 依赖：global.Redis、global.Config 必须已初始化
// 调用时机：Redis 初始化之后、Server 启动之前
func InitUploadRateLimiters() {
	cfg := global.Config.RateLimit.Upload

	// 全局限流器参数（含零值兜底）
	globalLimit := cfg.GlobalLimit
	if globalLimit <= 0 {
		globalLimit = 100
	}
	globalWindow := cfg.GlobalWindowSec
	if globalWindow <= 0 {
		globalWindow = 60
	}

	// 用户级限流器参数（含零值兜底）
	userLimit := cfg.UserLimit
	if userLimit <= 0 {
		userLimit = 10
	}
	userWindow := cfg.UserWindowSec
	if userWindow <= 0 {
		userWindow = 60
	}

	// 创建并注册到全局变量
	global.UploadGlobalLimiter = ratelimit.NewLimiter(
		global.Redis,
		"ratelimit:upload:global",
		globalLimit,
		time.Duration(globalWindow)*time.Second,
	)
	global.UploadUserLimiter = ratelimit.NewLimiter(
		global.Redis,
		"ratelimit:upload:user",
		userLimit,
		time.Duration(userWindow)*time.Second,
	)

	global.Log.Info("上传限流器初始化完成",
		zap.Int("global_limit", globalLimit),
		zap.Int("global_window_sec", globalWindow),
		zap.Int("user_limit", userLimit),
		zap.Int("user_window_sec", userWindow))
}

// InitOJBindRateLimiters 初始化 OJ 绑定接口滑动窗口限流器。
func InitOJBindRateLimiters() {
	cfg := global.Config.RateLimit.OJBind

	limit := cfg.Limit
	if limit <= 0 {
		limit = 3
	}
	windowSec := cfg.WindowSec
	if windowSec <= 0 {
		windowSec = 10
	}

	window := time.Duration(windowSec) * time.Second
	global.OJBindLimiters = map[string]*ratelimit.SlidingWindowLimiter{
		"leetcode": ratelimit.NewSlidingWindowLimiter(global.Redis, "ratelimit:oj_bind:leetcode", limit, window),
		"luogu":    ratelimit.NewSlidingWindowLimiter(global.Redis, "ratelimit:oj_bind:luogu", limit, window),
		"lanqiao":  ratelimit.NewSlidingWindowLimiter(global.Redis, "ratelimit:oj_bind:lanqiao", limit, window),
	}

	global.Log.Info("OJ 绑定限流器初始化完成",
		zap.Int("limit", limit),
		zap.Int("window_sec", windowSec))
}
