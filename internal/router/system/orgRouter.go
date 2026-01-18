package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

type OrgRouter struct{}

func (r *OrgRouter) InitOrgRouter(router *gin.RouterGroup) {
	orgRouter := router.Group("org")
	orgCtrl := controller.ApiGroupApp.SystemApiGroup.GetOrgCtrl()
	{
		orgRouter.GET("list", orgCtrl.GetOrgList) // 获取组织列表（支持分页与不分页）
	}
}
