package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// ObservabilityRouter 观测查询路由
type ObservabilityRouter struct{}

func (r *ObservabilityRouter) InitObservabilityRouter(router *gin.RouterGroup) {
	obsGroup := router.Group("system/observability")
	obsCtrl := controller.ApiGroupApp.SystemApiGroup.GetObservabilityCtrl()
	{
		obsGroup.GET("traces/detail/:id", obsCtrl.QueryTraceDetail)
		obsGroup.POST("traces/query", obsCtrl.QueryTrace)
		obsGroup.POST("metrics/query", obsCtrl.QueryMetrics)
		obsGroup.POST("runtime/query", obsCtrl.QueryRuntimeMetrics)
	}
}
