package system

import (
	"errors"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OJCtrl struct {
	ojService *serviceSystem.OJService
}

func (ctrl *OJCtrl) BindOJAccount(c *gin.Context) {
	var req request.BindOJAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(global.StatusUnauthorized).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.BindOJAccount(c.Request.Context(), userID, &req)
	if err != nil {
		code := global.StatusInternalServerError
		if errors.Is(err, serviceSystem.ErrInvalidPlatform) || errors.Is(err, serviceSystem.ErrInvalidIdentifier) {
			code = global.StatusBadRequest
		} else if errors.Is(err, serviceSystem.ErrBindCoolDown) {
			code = global.StatusTooManyRequests
		}
		global.Log.Error("绑定OJ账号失败",
			zap.Uint("user_id", userID),
			zap.String("platform", req.Platform),
			zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(code).
			Failed(fmt.Sprintf("绑定OJ账号失败: %v", err), nil)
		return
	}

	response.NewResponse[resp.BindOJAccountResp, resp.BindOJAccountResp](c).
		SetCode(global.StatusOK).
		Success("绑定成功", out)
}

func (ctrl *OJCtrl) GetRankingList(c *gin.Context) {
	var req request.OJRankingListReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("参数错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(global.StatusUnauthorized).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.GetRankingList(c.Request.Context(), userID, &req)
	if err != nil {
		code := global.StatusInternalServerError
		if errors.Is(err, serviceSystem.ErrInvalidPlatform) {
			code = global.StatusBadRequest
		}
		global.Log.Error("获取排行榜失败",
			zap.Uint("user_id", userID),
			zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(code).
			Failed(fmt.Sprintf("获取排行榜失败: %v", err), nil)
		return
	}

	response.NewResponse[resp.OJRankingListResp, resp.OJRankingListResp](c).
		SetCode(global.StatusOK).
		Success("获取成功", out)
}

func (ctrl *OJCtrl) GetStats(c *gin.Context) {
	var req request.OJStatsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(global.StatusBadRequest).
			Failed(fmt.Sprintf("参数错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(global.StatusUnauthorized).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.GetUserStats(c.Request.Context(), userID, &req)
	if err != nil {
		code := global.StatusInternalServerError
		if errors.Is(err, serviceSystem.ErrInvalidPlatform) || errors.Is(err, serviceSystem.ErrOJAccountNotBound) {
			code = global.StatusBadRequest
		}
		global.Log.Error("获取用户卡片信息失败",
			zap.Uint("user_id", userID),
			zap.String("platform", req.Platform),
			zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(code).
			Failed(fmt.Sprintf("获取用户卡片信息失败: %v", err), nil)
		return
	}

	response.NewResponse[resp.BindOJAccountResp, resp.BindOJAccountResp](c).
		SetCode(global.StatusOK).
		Success("获取成功", out)
}
