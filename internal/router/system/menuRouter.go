package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// MenuRouter 菜单管理路由
type MenuRouter struct{}

// InitMenuRouter 初始化菜单路由，挂载到 SystemGroup（需JWT+权限）
func (r *MenuRouter) InitMenuRouter(router *gin.RouterGroup) {
	menuGroup := router.Group("api/system/menu")
	menuCtrl := controller.ApiGroupApp.SystemApiGroup.GetMenuCtrl()
	{
	 	menuGroup.GET("tree", menuCtrl.GetMenuTree)     // 获取完整菜单树
		menuGroup.GET("my", menuCtrl.GetMyMenus)        // 获取当前用户菜单
		menuGroup.GET("list", menuCtrl.GetMenuList)     // 获取菜单列表（分页）
		menuGroup.POST("bind_api", menuCtrl.BindAPIs)   // 绑定API到菜单（需在:id前注册）
		menuGroup.POST("", menuCtrl.CreateMenu)         // 创建菜单
		menuGroup.GET(":id", menuCtrl.GetMenuByID)      // 获取菜单详情
		menuGroup.PUT(":id", menuCtrl.UpdateMenu)       // 更新菜单
		menuGroup.DELETE(":id", menuCtrl.DeleteMenu)    // 删除菜单
	}
}
