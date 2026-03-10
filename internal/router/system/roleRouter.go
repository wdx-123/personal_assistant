/**
 * @projectName: personal_assistant
 * @package: system
 * @className: roleRouter
 * @author: lijunqi
 * @description: 角色管理路由，注册角色CRUD及菜单权限分配接口
 * @date: 2026-02-02
 * @Version: 1.0
 */
package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// RoleRouter 角色管理路由
type RoleRouter struct{}

// InitRoleRouter 初始化角色路由，挂载到 SystemGroup（需JWT+权限）
func (r *RoleRouter) InitRoleRouter(router *gin.RouterGroup) {
	roleGroup := router.Group("system/role")
	roleCtrl := controller.ApiGroupApp.SystemApiGroup.GetRoleCtrl()
	{
		// 列表接口（需在动态路由前注册）
		roleGroup.GET("list", roleCtrl.GetRoleList)

		// 合并分配角色权限（菜单 + 直绑API）
		roleGroup.POST("assign_permission", roleCtrl.AssignPermissions)

		// CRUD接口
		roleGroup.POST("", roleCtrl.CreateRole)
		roleGroup.PUT(":id", roleCtrl.UpdateRole)
		roleGroup.DELETE(":id", roleCtrl.DeleteRole)

		// 获取角色菜单/API映射（支持可选max_level层级裁剪）
		roleGroup.GET(":id/menu_api_map", roleCtrl.GetRoleMenuAPIMap)
	}
}
