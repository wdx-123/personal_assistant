package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// HealthRouter 健康检查路由
type HealthRouter struct{}

// InitHealthRouter 初始化健康检查路由（公开路由）
func (r *HealthRouter) InitHealthRouter(router *gin.RouterGroup) {
	healthGroup := router.Group("api/v1")
	healthCtrl := controller.ApiGroupApp.SystemApiGroup.GetHealthCtrl()
	{
		healthGroup.GET("health", healthCtrl.Health)
		healthGroup.GET("ping", healthCtrl.Ping)
	}
}
