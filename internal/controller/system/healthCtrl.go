package system

import (
	"personal_assistant/global"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthCtrl 健康检查控制器
type HealthCtrl struct {
	healthService *serviceSystem.HealthService
}

// Health 健康检查
func (c *HealthCtrl) Health(ctx *gin.Context) {
	data, err := c.healthService.Health(ctx.Request.Context())
	if err != nil {
		global.Log.Error("health check failed", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	response.BizOkWithDetailed(data, "服务健康", ctx)
}

// Ping 轻量探活
func (c *HealthCtrl) Ping(ctx *gin.Context) {
	data, err := c.healthService.Ping(ctx.Request.Context())
	if err != nil {
		global.Log.Error("ping check failed", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	response.BizOkWithDetailed(data, "pong", ctx)
}
