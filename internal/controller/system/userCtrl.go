package system

import (
	"fmt"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	serviceContract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"
	"personal_assistant/pkg/util"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UserCtrl 封装当前领域的控制器入口。
type UserCtrl struct {
	userService serviceContract.UserServiceContract
	jwtService  serviceContract.JWTServiceContract
	aiService   serviceContract.AIServiceContract
}

// Register 注册
func (u *UserCtrl) Register(ctx *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var req request.RegisterReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", ctx)
		return
	}

	// 执行注册
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	user, err := u.userService.Register(ctx.Request.Context(), &req)
	if err != nil {
		global.Log.Error(
			"用户注册失败",
			zap.String("phone", req.Phone),
			zap.Error(err))
		response.BizFailWithError(err, ctx)
		return
	}

	global.Log.Info("用户注册成功",
		zap.String("phone", req.Phone),
		zap.Uint("userID", user.ID))

	// 注册成功后，直接生成 Token 并返回（自动登录）
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	u.TokenNext(ctx, *user)
}

// Login 登录接口
func (u *UserCtrl) Login(ctx *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var req request.LoginReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(bizerrors.CodeBindFailed).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}

	// 执行手机号登录
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	user, err := u.userService.PhoneLogin(ctx, &req)
	if err != nil {
		global.Log.Error("手机号登录失败",
			zap.String("phone", req.Phone),
			zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(bizerrors.CodeUnauthorized).
			Failed(fmt.Sprintf("登录失败: %v", err), nil)
		return
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	u.TokenNext(ctx, *user)
}

// TokenNext 负责执行当前函数对应的核心逻辑。
// 参数：
//   - c：调用方传入的目标对象或配置实例。
//   - user：当前函数需要消费的输入参数。
//
// 返回值：
//   - 无。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (u *UserCtrl) TokenNext(c *gin.Context, user entity.User) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	helper := response.NewAPIHelper(c, "LoginTokenNext")
	loginResp, refreshToken, refreshExpiresAt, jwtErr := u.jwtService.IssueLoginTokens(c.Request.Context(), user)
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if jwtErr != nil {
		helper.CommonError(jwtErr.Message, jwtErr.Code, jwtErr.Err)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(jwtErr.Code).
			Failed(jwtErr.Message, &resp.AuthResponse{Message: jwtErr.Message, Reload: true})
		return
	}

	// 将刷新令牌写入HttpOnly Cookie（统一使用 jwt 包的辅助函数）
	if refreshToken != "" {
		nowMs := time.Now().UnixMilli()
		ttlMs := refreshExpiresAt - nowMs
		maxAge := 0
		if ttlMs > 0 {
			maxAge = int(ttlMs / 1000)
		}
		jwt.SetRefreshToken(c, refreshToken, maxAge)
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	response.NewResponse[resp.LoginResponse, resp.LoginResponse](c).
		SetTrans(&resp.LoginResponse{}).
		Success("登录成功", loginResp)
}

// Logout 登出：清除刷新令牌 Cookie
func (u *UserCtrl) Logout(c *gin.Context) {
	// 读取必要信息（尽量复用已有的工具函数）
	uid := jwt.GetUUID(c)
	userID := jwt.GetUserID(c)
	jwtStr := jwt.GetRefreshToken(c)

	// 清除刷新令牌 Cookie（HttpOnly）
	jwt.ClearRefreshToken(c)

	// 移除Redis中的登录状态（多点登录与单点是同一个场景）
	if err := global.Redis.Del(c.Request.Context(), uid.String()).Err(); err != nil {
		global.Log.Warn("Redis 删除登录状态失败",
			zap.String("uuid", uid.String()),
			zap.Error(err))
	}

	// 将当前刷新令牌加入黑名单（防止后续再使用）
	if jwtStr != "" {
		if err := u.jwtService.JoinInBlacklist(
			c.Request.Context(),
			entity.JwtBlacklist{JWT: jwtStr}); err != nil {
			global.Log.Warn("加入刷新令牌黑名单失败", zap.Error(err))
		}
	}
	if u.aiService != nil && userID > 0 {
		u.aiService.RevokeUserSessions(c.Request.Context(), userID, "logout")
	}

	response.NewResponse[any, any](c).
		SetCode(bizerrors.CodeSuccess).
		Success("登出成功",
			map[string]any{"message": "已成功退出登录"})
}

// UpdateProfile 更新个人资料
func (u *UserCtrl) UpdateProfile(c *gin.Context) {
	var req request.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	user, err := u.userService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		global.Log.Error("更新个人资料失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithData(entityToUserDetail(user), c)
}

// ChangePhone 换绑手机号
func (u *UserCtrl) ChangePhone(c *gin.Context) {
	var req request.ChangePhoneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	user, err := u.userService.ChangePhone(c.Request.Context(), userID, &req)
	if err != nil {
		global.Log.Error("换绑手机号失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithData(entityToUserDetail(user), c)
}

// ChangePassword 修改密码
func (u *UserCtrl) ChangePassword(c *gin.Context) {
	var req request.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := u.userService.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		global.Log.Error("修改密码失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithMessage("密码修改成功，请重新登录", c)
}

// DeactivateAccount 主动注销账号（等同禁用）
func (u *UserCtrl) DeactivateAccount(c *gin.Context) {
	var req request.DeactivateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("注销账号参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	userID := jwt.GetUserID(c)
	if err := u.userService.DeactivateAccount(c.Request.Context(), userID, &req); err != nil {
		global.Log.Error("注销账号失败", zap.Uint("userID", userID), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	if u.aiService != nil {
		u.aiService.RevokeUserSessions(c.Request.Context(), userID, "deactivate")
	}
	jwt.ClearRefreshToken(c)
	response.BizOkWithMessage("账号已禁用", c)
}

// GetUserList 获取用户列表
func (u *UserCtrl) GetUserList(c *gin.Context) {
	var req request.UserListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BizFailWithMessage("参数错误", c)
		return
	}

	data, err := u.userService.GetUserList(c.Request.Context(), &req)
	if err != nil {
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithData(data, c)
}

// GetUserDetail 获取用户详情
func (u *UserCtrl) GetUserDetail(c *gin.Context) {
	id := util.ParseUint(c.Param("id"))
	if id == 0 {
		response.BizFailWithMessage("ID无效", c)
		return
	}

	user, err := u.userService.GetUserDetail(c.Request.Context(), uint(id))
	if err != nil {
		response.BizFailWithError(err, c)
		return
	}
	if user == nil {
		response.BizFailWithCode(bizerrors.CodeUserNotFound, c)
		return
	}

	response.BizOkWithData(entityToUserDetail(user), c)
}

// GetUserRoles 获取用户在组织下的角色
func (u *UserCtrl) GetUserRoles(c *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	id := util.ParseUint(c.Param("id"))
	if id == 0 {
		response.BizFailWithMessage("ID无效", c)
		return
	}
	orgID := util.ParseUint(c.Query("org_id"))
	if orgID == 0 {
		response.BizFailWithMessage("必须指定组织ID", c)
		return
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	roles, err := u.userService.GetUserRoles(c.Request.Context(), uint(id), uint(orgID))
	if err != nil {
		response.BizFailWithError(err, c)
		return
	}

	// 转换为简单响应结构
	list := make([]resp.RoleSimpleItem, len(roles))
	for i, r := range roles {
		list[i] = resp.RoleSimpleItem{
			ID:   r.ID,
			Name: r.Name,
			Code: r.Code,
		}
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	response.BizOkWithData(list, c)
}

// GetUserRoleMatrix 获取用户角色分配矩阵
func (u *UserCtrl) GetUserRoleMatrix(c *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	id := util.ParseUint(c.Param("id"))
	if id == 0 {
		response.BizFailWithMessage("ID无效", c)
		return
	}

	var query request.GetUserRoleMatrixQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		global.Log.Error("获取用户角色矩阵参数绑定失败", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeInvalidParams, "参数错误", c)
		return
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if query.OrgID == 0 {
		response.BizFailWithCodeMsg(bizerrors.CodeInvalidParams, "必须指定组织ID", c)
		return
	}

	operatorID := jwt.GetUserID(c)
	matrix, err := u.userService.GetUserRoleMatrix(c.Request.Context(), operatorID, uint(id), query.OrgID)
	if err != nil {
		global.Log.Error(
			"获取用户角色矩阵失败",
			zap.Uint("operatorID", operatorID),
			zap.Uint("targetUserID", uint(id)),
			zap.Uint("orgID", query.OrgID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	response.BizOkWithData(matrix, c)
}

// AssignRole 分配角色
func (u *UserCtrl) AssignRole(c *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var req request.AssignUserRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("分配角色参数绑定失败", zap.Error(err))
		response.BizFailWithMessage("参数错误", c)
		return
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	operatorID := jwt.GetUserID(c)
	if err := u.userService.AssignRole(c.Request.Context(), operatorID, &req); err != nil {
		global.Log.Error(
			"分配角色失败",
			zap.Uint("operatorID", operatorID),
			zap.Uint("targetUserID", req.UserID),
			zap.Uint("orgID", req.OrgID),
			zap.Error(err),
		)
		response.BizFailWithError(err, c)
		return
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	response.BizOk(c)
}

// UpdateUserStatus 管理员启用/禁用账号
func (u *UserCtrl) UpdateUserStatus(c *gin.Context) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	id := util.ParseUint(c.Param("id"))
	if id == 0 {
		response.BizFailWithMessage("ID无效", c)
		return
	}

	var req request.AdminUpdateUserStatusReq
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BizFailWithMessage("参数错误", c)
		return
	}

	operatorID := jwt.GetUserID(c)
	if err := u.userService.UpdateUserStatus(c.Request.Context(), operatorID, uint(id), &req); err != nil {
		global.Log.Error("更新用户状态失败", zap.Uint("targetUserID", uint(id)), zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	response.BizOkWithMessage("用户状态更新成功", c)
}

// ==================== 辅助函数 ====================

// entityToUserDetail 将用户实体转换为详情DTO
func entityToUserDetail(user *entity.User) *resp.UserDetailItem {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	item := &resp.UserDetailItem{
		ID:           user.ID,
		UUID:         user.UUID.String(),
		Username:     user.Username,
		Phone:        util.DesensitizePhone(user.Phone),
		Email:        user.Email,
		Avatar:       user.Avatar,
		AvatarID:     user.AvatarID,
		Address:      user.Address,
		Signature:    user.Signature,
		Register:     int(user.Register),
		Freeze:       user.Freeze,
		Status:       int(user.Status),
		IsSuperAdmin: user.IsSuperAdmin,
		CreatedAt:    user.CreatedAt.Format(time.DateTime),
		UpdatedAt:    user.UpdatedAt.Format(time.DateTime),
		CurrentOrgID: user.CurrentOrgID,
	}
	if user.DisabledAt != nil {
		item.DisabledAt = user.DisabledAt.Format(time.DateTime)
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if user.CurrentOrg != nil {
		item.CurrentOrg = &resp.OrgSimpleItem{
			ID:   user.CurrentOrg.ID,
			Name: user.CurrentOrg.Name,
		}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return item
}
