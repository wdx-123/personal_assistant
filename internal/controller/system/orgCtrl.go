package system

import (
	"strconv"
	"time"

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

// OrgCtrl 组织管理控制器
type OrgCtrl struct {
	orgService *serviceSystem.OrgService
}

// NewOrgCtrl 创建组织控制器实例
func NewOrgCtrl(orgService *serviceSystem.OrgService) *OrgCtrl {
	return &OrgCtrl{
		orgService: orgService,
	}
}

// GetOrgList 获取组织列表（支持分页、关键词搜索）
func (ctrl *OrgCtrl) GetOrgList(c *gin.Context) {
	var req request.OrgListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	// 默认值处理：如果只传了pageSize没传page，默认第一页；如果什么都没传，默认不分页
	if req.PageSize > 0 && req.Page == 0 {
		req.Page = 1
	}

	orgs, total, err := ctrl.orgService.GetOrgList(c.Request.Context(), req.Page, req.PageSize, req.Keyword)
	if err != nil {
		global.Log.Error("获取组织列表失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	// 转换为响应 DTO
	items := make([]*resp.OrgItem, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, entityToOrgItem(org))
	}
	response.BizOkWithPage(items, total, req.Page, req.PageSize, c)
}

// GetOrgDetail 获取组织详情
func (ctrl *OrgCtrl) GetOrgDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	org, err := ctrl.orgService.GetOrgDetail(c.Request.Context(), userID, uint(id))
	if err != nil {
		global.Log.Error("获取组织详情失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(entityToOrgItem(org), c)
}

// CreateOrg 创建组织
func (ctrl *OrgCtrl) CreateOrg(c *gin.Context) {
	var req request.CreateOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("创建组织参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.CreateOrg(c.Request.Context(), userID, &req); err != nil {
		global.Log.Error("创建组织失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("创建成功", c)
}

// UpdateOrg 更新组织信息
func (ctrl *OrgCtrl) UpdateOrg(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", c)
		return
	}

	var req request.UpdateOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("更新组织参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.UpdateOrg(c.Request.Context(), userID, uint(id), &req); err != nil {
		global.Log.Error("更新组织失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("更新成功", c)
}

// DeleteOrg 删除组织
func (ctrl *OrgCtrl) DeleteOrg(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BizFailWithMessage("ID格式错误", c)
		return
	}

	// 解析 force 参数
	forceStr := c.DefaultQuery("force", "false")
	force := forceStr == "true" || forceStr == "1"

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.DeleteOrg(c.Request.Context(), userID, uint(id), force); err != nil {
		global.Log.Error("删除组织失败", zap.Uint64("id", id), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("删除成功", c)
}

// SetCurrentOrg 切换当前组织
func (ctrl *OrgCtrl) SetCurrentOrg(c *gin.Context) {
	var req request.SetCurrentOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("切换组织参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.SetCurrentOrg(c.Request.Context(), userID, req.OrgID); err != nil {
		global.Log.Error("切换组织失败", zap.Uint("orgID", req.OrgID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("切换成功", c)
}

// GetMyOrgs 获取我的组织列表
func (ctrl *OrgCtrl) GetMyOrgs(c *gin.Context) {
	userID := jwt.GetUserID(c)
	items, err := ctrl.orgService.GetMyOrgs(c.Request.Context(), userID)
	if err != nil {
		global.Log.Error("获取我的组织失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithData(items, c)
}

// ==================== 辅助函数 ====================

// entityToOrgItem 将组织实体转换为响应DTO
func entityToOrgItem(org *entity.Org) *resp.OrgItem {
	return &resp.OrgItem{
		ID:          org.ID,
		Name:        org.Name,
		Description: org.Description,
		Code:        org.Code,
		Avatar:      org.Avatar,
		AvatarID:    org.AvatarID,
		OwnerID:     org.OwnerID,
		CreatedAt:   org.CreatedAt.Format(time.DateTime),
		UpdatedAt:   org.UpdatedAt.Format(time.DateTime),
	}
}
