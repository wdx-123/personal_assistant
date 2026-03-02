package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// MenuRouter 菜单管理路由
type MenuRouter struct{}

// InitMenuRouter 初始化菜单路由，挂载到 SystemGroup（需JWT+权限校验）
// 路由前缀: /api/system/menu
func (r *MenuRouter) InitMenuRouter(router *gin.RouterGroup) {
	menuGroup := router.Group("system/menu")
	menuCtrl := controller.ApiGroupApp.SystemApiGroup.GetMenuCtrl()
	{
		// 查询类接口
		// GET /tree - 获取完整菜单树（管理端配置页，含禁用菜单）
		menuGroup.GET("tree", menuCtrl.GetMenuTree)
		// GET /my?org_id=x - 获取当前用户可访问的菜单树（前端侧边栏渲染）
		menuGroup.GET("my", menuCtrl.GetMyMenus)
		// GET /list?page=1&page_size=10&keyword=xx - 分页查询菜单列表（扁平结构）
		menuGroup.GET("list", menuCtrl.GetMenuList)

		// 写操作接口（bind_api 需在 :id 路由前注册，避免路径冲突）
		// POST /bind_api - 绑定API到菜单（覆盖式，单菜单语义：API已归属其他菜单时自动迁移）
		menuGroup.POST("bind_api", menuCtrl.BindAPIs)
		// POST / - 创建菜单/目录/按钮
		menuGroup.POST("", menuCtrl.CreateMenu)

		// 单资源操作接口
		// GET /:id - 获取菜单详情（含绑定的API列表）
		menuGroup.GET(":id", menuCtrl.GetMenuByID)
		// PUT /:id - 更新菜单（支持部分更新）
		menuGroup.PUT(":id", menuCtrl.UpdateMenu)
		// DELETE /:id - 删除菜单（有子菜单时禁止删除）
		menuGroup.DELETE(":id", menuCtrl.DeleteMenu)
	}
}
