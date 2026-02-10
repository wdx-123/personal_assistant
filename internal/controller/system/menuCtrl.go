package system

import (
	"strconv"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MenuCtrl 菜单管理控制器
type MenuCtrl struct {
	menuService *serviceSystem.MenuService
}

// NewMenuCtrl 创建菜单控制器实例
func NewMenuCtrl(menuService *serviceSystem.MenuService) *MenuCtrl {
	return &MenuCtrl{menuService: menuService}
}

// GetMenuTree 获取完整菜单树（管理端配置页）
func (c *MenuCtrl) GetMenuTree(ctx *gin.Context) {
	tree, err := c.menuService.GetMenuTree(ctx.Request.Context())
	if err != nil {
		global.Log.Error("获取菜单树失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(tree, ctx)
}

// GetMyMenus 获取当前用户的菜单树（前端侧边栏）
// org_id 为可选查询参数：超级管理员无需传入，普通用户不传则返回错误提示
func (c *MenuCtrl) GetMyMenus(ctx *gin.Context) {
	var req request.MyMenuReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		global.Log.Error("获取用户菜单参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	// 从JWT获取用户信息
	userID := jwt.GetUserID(ctx)
	if userID == 0 {
		response.BizFailWithMessage("无法获取用户信息", ctx)
		return
	}

	// org_id 作为可选参数透传给 Service，由 Service 决定是否必须
	tree, err := c.menuService.GetMyMenus(ctx.Request.Context(), userID, req.OrgID)
	if err != nil {
		global.Log.Error("获取用户菜单失败", zap.Uint("userID", userID), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(tree, ctx)
}

// GetMenuList 获取菜单列表（分页，扁平）
func (c *MenuCtrl) GetMenuList(ctx *gin.Context) {
	var filter request.MenuListFilter
	if err := ctx.ShouldBindQuery(&filter); err != nil {
		global.Log.Error("菜单列表参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	list, total, err := c.menuService.GetMenuList(ctx.Request.Context(), &filter)
	if err != nil {
		global.Log.Error("获取菜单列表失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	// 转换为响应格式
	items := make([]*resp.MenuItem, 0, len(list))
	for _, m := range list {
		items = append(items, entityToMenuItem(m))
	}
	response.BizOkWithPage(items, total, filter.Page, filter.PageSize, ctx)
}

// CreateMenu 创建菜单
func (c *MenuCtrl) CreateMenu(ctx *gin.Context) {
	var req request.CreateMenuReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("创建菜单参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.menuService.CreateMenu(ctx.Request.Context(), &req); err != nil {
		global.Log.Error("创建菜单失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("创建成功", ctx)
}

// GetMenuByID 获取菜单详情
func (c *MenuCtrl) GetMenuByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	menu, err := c.menuService.GetMenuByID(ctx.Request.Context(), uint(id))
	if err != nil {
		global.Log.Error("获取菜单详情失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(entityToMenuItemWithAPIs(menu), ctx)
}

// UpdateMenu 更新菜单
func (c *MenuCtrl) UpdateMenu(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	var req request.UpdateMenuReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("更新菜单参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.menuService.UpdateMenu(ctx.Request.Context(), uint(id), &req); err != nil {
		global.Log.Error("更新菜单失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("更新成功", ctx)
}

// DeleteMenu 删除菜单
func (c *MenuCtrl) DeleteMenu(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	if err := c.menuService.DeleteMenu(ctx.Request.Context(), uint(id)); err != nil {
		global.Log.Error("删除菜单失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("删除成功", ctx)
}

// BindAPIs 绑定API到菜单
func (c *MenuCtrl) BindAPIs(ctx *gin.Context) {
	var req request.BindMenuAPIReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("绑定API参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.menuService.BindAPIs(ctx.Request.Context(), req.MenuID, req.APIIDs); err != nil {
		global.Log.Error("绑定API失败", zap.Uint("menuID", req.MenuID), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("绑定成功", ctx)
}

// entityToMenuItem 转换实体到响应DTO（不含APIs）
func entityToMenuItem(m *entity.Menu) *resp.MenuItem {
	return &resp.MenuItem{
		ID:            m.ID,
		ParentID:      m.ParentID,
		Name:          m.Name,
		Code:          m.Code,
		Type:          m.Type,
		Icon:          m.Icon,
		RouteName:     m.RouteName,
		RoutePath:     m.RoutePath,
		RouteParam:    m.RouteParam,
		ComponentPath: m.ComponentPath,
		Status:        m.Status,
		Sort:          m.Sort,
		Desc:          m.Desc,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

// entityToMenuItemWithAPIs 转换实体到响应DTO（含APIs）
func entityToMenuItemWithAPIs(m *entity.Menu) *resp.MenuItem {
	item := entityToMenuItem(m)
	if len(m.APIs) > 0 {
		item.APIs = make([]*resp.APIItemSimple, 0, len(m.APIs))
		for _, api := range m.APIs {
			item.APIs = append(item.APIs, &resp.APIItemSimple{
				ID:     api.ID,
				Path:   api.Path,
				Method: api.Method,
			})
		}
	}
	return item
}
