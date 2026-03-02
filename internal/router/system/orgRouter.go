package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// OrgRouter 组织管理路由
type OrgRouter struct{}

// InitOrgRouter 初始化组织公共路由（无需JWT）
// 目前无公共接口，保留入口以兼容现有调用
func (r *OrgRouter) InitOrgRouter(router *gin.RouterGroup) {
	// 公共接口暂无，预留
}

// InitOrgAuthRouter 初始化组织管理路由（需JWT+权限中间件）
// 挂载到 SystemGroup
func (r *OrgRouter) InitOrgAuthRouter(router *gin.RouterGroup) {
	orgGroup := router.Group("system/org")
	orgCtrl := controller.ApiGroupApp.SystemApiGroup.GetOrgCtrl()
	{
		// 列表与详情（需在动态路由前注册）
		orgGroup.GET("list", orgCtrl.GetOrgList)

		// CRUD 接口
		// 创建组织
		orgGroup.POST("", orgCtrl.CreateOrg)
		// 获取组织详情
		orgGroup.GET(":id", orgCtrl.GetOrgDetail)
		// 更新组织
		orgGroup.PUT(":id", orgCtrl.UpdateOrg)
		// 删除组织
		orgGroup.DELETE(":id", orgCtrl.DeleteOrg)
	}
}

// InitOrgBusinessRouter 初始化组织业务路由（需JWT，无严格权限控制）
// 挂载到 BusinessGroup
func (r *OrgRouter) InitOrgBusinessRouter(router *gin.RouterGroup) {
	orgGroup := router.Group("system/org")
	orgCtrl := controller.ApiGroupApp.SystemApiGroup.GetOrgCtrl()
	{
		orgGroup.GET("my", orgCtrl.GetMyOrgs)          // 获取我的组织
		orgGroup.PUT("current", orgCtrl.SetCurrentOrg) // 切换当前组织
	}
}
