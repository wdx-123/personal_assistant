package global

import (
	"personal_assistant/internal/model/config"
	"personal_assistant/pkg/ratelimit"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/cache/local_cache"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	Config         *config.Config    // 全局配置实例
	Log            *zap.Logger       // 全局日志实例
	DB             *gorm.DB          // 全局数据库连接实例
	Redis          *redis.Client     // 全局Redis客户端实例
	BlackCache     local_cache.Cache // 全局黑名单缓存实例
	CasbinEnforcer *casbin.Enforcer  // 全局Casbin执行器实例
	Router         *gin.Engine       // 全局路由实例（用于API同步等功能）

	// 上传限流器（由 core.InitUploadRateLimiters 初始化）
	UploadGlobalLimiter *ratelimit.Limiter // 上传接口全局限流器
	UploadUserLimiter   *ratelimit.Limiter // 上传接口用户级限流器
)
