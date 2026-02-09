package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// ApiRouter API接口管理路由
type ApiRouter struct{}

// InitApiRouter 初始化API路由，挂载到 SystemGroup（需JWT+权限）
func (r *ApiRouter) InitApiRouter(router *gin.RouterGroup) {
	apiGroup := router.Group("system/api")
	apiCtrl := controller.ApiGroupApp.SystemApiGroup.GetApiCtrl()
	{
		apiGroup.GET("list", apiCtrl.GetAPIList)       // 获取API列表
		apiGroup.POST("sync", apiCtrl.SyncAPI)         // 同步路由到API表（需在:id前注册）
		apiGroup.POST("", apiCtrl.CreateAPI)           // 创建API
		apiGroup.GET(":id", apiCtrl.GetAPIByID)        // 获取API详情
		apiGroup.PUT(":id", apiCtrl.UpdateAPI)         // 更新API
		apiGroup.DELETE(":id", apiCtrl.DeleteAPI)      // 删除API
	}
}
