package system

import (
	"strconv"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	readmodel "personal_assistant/internal/model/readmodel"
	serviceContract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"
	"personal_assistant/pkg/util"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// OrgCtrl 组织管理控制器
type OrgCtrl struct {
	orgService serviceContract.OrgServiceContract
}

// NewOrgCtrl 创建组织控制器实例
func NewOrgCtrl(orgService serviceContract.OrgServiceContract) *OrgCtrl {
	return &OrgCtrl{
		orgService: orgService,
	}
}

// GetOrgList 获取组织列表（按当前用户可见范围过滤，支持分页、关键词搜索）
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

	userID := jwt.GetUserID(c)
	orgs, total, err := ctrl.orgService.GetOrgList(c.Request.Context(), userID, req.Page, req.PageSize, req.Keyword)
	if err != nil {
		global.Log.Error("获取组织列表失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	// 转换为响应 DTO
	items := make([]*resp.OrgItem, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, readModelToOrgItem(org))
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
	response.BizOkWithData(readModelToOrgItem(org), c)
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

// JoinOrgByInviteCode 通过邀请码加入组织
func (ctrl *OrgCtrl) JoinOrgByInviteCode(c *gin.Context) {
	var req request.JoinOrgByInviteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("加入组织参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.JoinOrgByInviteCode(c.Request.Context(), userID, req.InviteCode); err != nil {
		global.Log.Error("加入组织失败", zap.String("invite_code", req.InviteCode), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("加入成功", c)
}

// LeaveOrg 主动退出组织
func (ctrl *OrgCtrl) LeaveOrg(c *gin.Context) {
	var req request.LeaveOrgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("退出组织参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := ctrl.orgService.LeaveOrg(c.Request.Context(), userID, req.OrgID, req.Reason); err != nil {
		global.Log.Error("退出组织失败", zap.Uint("org_id", req.OrgID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("退出成功", c)
}

// KickMember 踢出组织成员
func (ctrl *OrgCtrl) KickMember(c *gin.Context) {
	orgID := util.ParseUint(c.Param("id"))
	targetUserID := util.ParseUint(c.Param("userId"))
	if orgID == 0 || targetUserID == 0 {
		response.BizFailWithMessage("参数错误", c)
		return
	}

	var req request.KickMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许 DELETE 无 body
		req.Reason = c.Query("reason")
	}

	operatorID := jwt.GetUserID(c)
	if err := ctrl.orgService.KickMember(c.Request.Context(), operatorID, uint(orgID), uint(targetUserID), req.Reason); err != nil {
		global.Log.Error(
			"踢出成员失败",
			zap.Uint("org_id", orgID),
			zap.Uint("target_user_id", targetUserID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("已踢出成员", c)
}

// RecoverMember 恢复成员
func (ctrl *OrgCtrl) RecoverMember(c *gin.Context) {
	orgID := util.ParseUint(c.Param("id"))
	targetUserID := util.ParseUint(c.Param("userId"))
	if orgID == 0 || targetUserID == 0 {
		response.BizFailWithMessage("参数错误", c)
		return
	}

	var req request.RecoverMemberReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("恢复成员参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	operatorID := jwt.GetUserID(c)
	if err := ctrl.orgService.RecoverMember(c.Request.Context(), operatorID, uint(orgID), uint(targetUserID), req.Reason); err != nil {
		global.Log.Error(
			"恢复成员失败",
			zap.Uint("org_id", orgID),
			zap.Uint("target_user_id", targetUserID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("成员已恢复", c)
}

// ==================== 辅助函数 ====================

// readModelToOrgItem 将组织读模型转换为响应DTO。
func readModelToOrgItem(org *readmodel.OrgWithMemberCount) *resp.OrgItem {
	return &resp.OrgItem{
		ID:          org.ID,
		Name:        org.Name,
		Description: org.Description,
		Code:        org.Code,
		Avatar:      org.Avatar,
		AvatarID:    org.AvatarID,
		OwnerID:     org.OwnerID,
		MemberCount: org.MemberCount,
		CreatedAt:   org.CreatedAt.Format(time.DateTime),
		UpdatedAt:   org.UpdatedAt.Format(time.DateTime),
	}
}
