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

		// 分配菜单权限（需在:id前注册，避免路由冲突）
		roleGroup.POST("assign_menu", roleCtrl.AssignMenus)
		// 分配角色API权限（直绑，全量替换）
		roleGroup.POST("assign_api", roleCtrl.AssignAPIs)

		// CRUD接口
		roleGroup.POST("", roleCtrl.CreateRole)
		roleGroup.PUT(":id", roleCtrl.UpdateRole)
		roleGroup.DELETE(":id", roleCtrl.DeleteRole)

		// 获取角色菜单权限
		roleGroup.GET(":id/menus", roleCtrl.GetRoleMenuIDs)
		// 获取角色菜单/API映射（一次性渲染大对象）
		roleGroup.GET(":id/menu_api_map", roleCtrl.GetRoleMenuAPIMap)
	}
}
