package system

import (
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserCtrl struct {
	userService *serviceSystem.UserService
	jwtService  *serviceSystem.JWTService
}

// Register 注册
func (u *UserCtrl) Register(ctx *gin.Context) {
	var req request.RegisterReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		global.Log.Error("绑定数据错误",
			zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}

	// 执行注册
	user, err := u.userService.Register(ctx, &req)
	if err != nil {
		global.Log.Error(
			"用户注册失败",
			zap.String("phone", req.Phone),
			zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(global.StatusInternalServerError).
			Failed(fmt.Sprintf("用户注册失败: %v", err), nil)
		return
	}

	global.Log.Info("用户注册成功",
		zap.String("phone", req.Phone),
		zap.Uint("userID", user.ID))

	// 注册成功后，直接生成 Token 并返回（自动登录）
	u.TokenNext(ctx, *user)
}

// Login 登录接口
func (u *UserCtrl) Login(ctx *gin.Context) {
	var req request.LoginReq
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}

	// 执行手机号登录
	user, err := u.userService.PhoneLogin(ctx, &req)
	if err != nil {
		global.Log.Error("手机号登录失败",
			zap.String("phone", req.Phone),
			zap.Error(err))
		response.NewResponse[any, any](ctx).
			SetCode(global.StatusUnauthorized).
			Failed(fmt.Sprintf("登录失败: %v", err), nil)
		return
	}

	u.TokenNext(ctx, *user)
}

func (u *UserCtrl) TokenNext(c *gin.Context, user entity.User) {
	helper := response.NewAPIHelper(c, "LoginTokenNext")
	loginResp, refreshToken, refreshExpiresAt, jwtErr := u.jwtService.IssueLoginTokens(c.Request.Context(), user)
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

	response.NewResponse[resp.LoginResponse, resp.LoginResponse](c).
		SetTrans(&resp.LoginResponse{}).
		Success("登录成功", loginResp)
}

// Logout 登出：清除刷新令牌 Cookie
func (u *UserCtrl) Logout(c *gin.Context) {
	// 读取必要信息（尽量复用已有的工具函数）
	uid := jwt.GetUUID(c)
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

	response.NewResponse[any, any](c).
		SetCode(global.StatusOK).
		Success("登出成功",
			map[string]any{"message": "已成功退出登录"})
}
