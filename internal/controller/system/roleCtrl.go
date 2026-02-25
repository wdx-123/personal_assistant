/**
 * @projectName: personal_assistant
 * @package: system
 * @className: roleCtrl
 * @author: lijunqi
 * @description: 角色管理控制器，处理角色CRUD及菜单权限分配请求
 * @date: 2026-02-02
 * @Version: 1.0
 */
package system

import (
	"strconv"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RoleCtrl 角色管理控制器
type RoleCtrl struct {
	roleService *serviceSystem.RoleService
}

// NewRoleCtrl 创建角色控制器实例
func NewRoleCtrl(roleService *serviceSystem.RoleService) *RoleCtrl {
	return &RoleCtrl{roleService: roleService}
}

// GetRoleList 获取角色列表（分页，支持过滤）
// @Summary 获取角色列表
// @Tags System: Role
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param status query int false "状态过滤：1启用 0禁用"
// @Param keyword query string false "按名称或code模糊搜索"
// @Success 200 {object} response.Response
// @Router /api/system/role/list [get]
func (c *RoleCtrl) GetRoleList(ctx *gin.Context) {
	var filter request.RoleListFilter
	if err := ctx.ShouldBindQuery(&filter); err != nil {
		global.Log.Error("角色列表参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	// 默认值处理
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}

	list, total, err := c.roleService.GetRoleList(ctx.Request.Context(), &filter)
	if err != nil {
		global.Log.Error("获取角色列表失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	// 转换为响应格式
	items := make([]*resp.RoleItem, 0, len(list))
	for _, role := range list {
		items = append(items, entityToRoleItem(role))
	}
	response.BizOkWithPage(items, total, filter.Page, filter.PageSize, ctx)
}

// CreateRole 创建角色
// @Summary 创建角色
// @Tags System: Role
// @Accept json
// @Produce json
// @Param body body request.CreateRoleReq true "创建角色请求"
// @Success 200 {object} response.Response
// @Router /api/system/role [post]
func (c *RoleCtrl) CreateRole(ctx *gin.Context) {
	var req request.CreateRoleReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("创建角色参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.roleService.CreateRole(ctx.Request.Context(), &req); err != nil {
		global.Log.Error("创建角色失败", zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("创建成功", ctx)
}

// UpdateRole 更新角色
// @Summary 更新角色
// @Tags System: Role
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.UpdateRoleReq true "更新角色请求"
// @Success 200 {object} response.Response
// @Router /api/system/role/{id} [put]
func (c *RoleCtrl) UpdateRole(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	var req request.UpdateRoleReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("更新角色参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.roleService.UpdateRole(ctx.Request.Context(), uint(id), &req); err != nil {
		global.Log.Error("更新角色失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("更新成功", ctx)
}

// DeleteRole 删除角色
// @Summary 删除角色
// @Tags System: Role
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.Response
// @Router /api/system/role/{id} [delete]
func (c *RoleCtrl) DeleteRole(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	if err := c.roleService.DeleteRole(ctx.Request.Context(), uint(id)); err != nil {
		global.Log.Error("删除角色失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("删除成功", ctx)
}

// AssignMenus 分配菜单权限
// @Summary 分配菜单权限
// @Tags System: Role
// @Accept json
// @Produce json
// @Param body body request.AssignRoleMenuReq true "分配菜单权限请求"
// @Success 200 {object} response.Response
// @Router /api/system/role/assign_menu [post]
func (c *RoleCtrl) AssignMenus(ctx *gin.Context) {
	var req request.AssignRoleMenuReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("分配菜单权限参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}

	if err := c.roleService.AssignMenus(ctx.Request.Context(), req.RoleID, req.MenuIDs); err != nil {
		global.Log.Error("分配菜单权限失败", zap.Uint("roleID", req.RoleID), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("分配成功", ctx)
}

// AssignAPIs 分配角色API权限（直绑，全量替换）
func (c *RoleCtrl) AssignAPIs(ctx *gin.Context) {
	var req request.AssignRoleAPIReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		global.Log.Error("分配角色API权限参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", ctx)
		return
	}
	if req.APIIDs == nil {
		response.BizFailWithMessage("api_ids 必须传入（可为空数组）", ctx)
		return
	}

	if err := c.roleService.AssignAPIs(ctx.Request.Context(), req.RoleID, req.APIIDs); err != nil {
		global.Log.Error("分配角色API权限失败", zap.Uint("roleID", req.RoleID), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithMessage("分配成功", ctx)
}

// GetRoleMenuIDs 获取角色菜单权限（返回菜单ID列表）
// @Summary 获取角色菜单权限
// @Tags System: Role
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.Response
// @Router /api/system/role/{id}/menus [get]
func (c *RoleCtrl) GetRoleMenuIDs(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	menuIDs, err := c.roleService.GetRoleMenuIDs(ctx.Request.Context(), uint(id))
	if err != nil {
		global.Log.Error("获取角色菜单权限失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(menuIDs, ctx)
}

// GetRoleMenuAPIMap 获取角色菜单/API映射（一次性渲染大对象）
func (c *RoleCtrl) GetRoleMenuAPIMap(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", ctx)
		return
	}

	mapping, err := c.roleService.GetRoleMenuAPIMap(ctx.Request.Context(), uint(id))
	if err != nil {
		global.Log.Error("获取角色菜单/API映射失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}
	response.BizOkWithData(mapping, ctx)
}

// ==================== 辅助函数 ====================

// entityToRoleItem 将角色实体转换为响应DTO
func entityToRoleItem(role *entity.Role) *resp.RoleItem {
	return &resp.RoleItem{
		ID:        role.ID,
		Name:      role.Name,
		Code:      role.Code,
		Desc:      role.Desc,
		Status:    role.Status,
		CreatedAt: role.CreatedAt,
		UpdatedAt: role.UpdatedAt,
	}
}
