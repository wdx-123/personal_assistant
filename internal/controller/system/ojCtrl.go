package system

import (
	"errors"
	"fmt"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	serviceContract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type OJCtrl struct {
	ojService serviceContract.OJServiceContract
}

func (ctrl *OJCtrl) BindOJAccount(c *gin.Context) {
	var req request.BindOJAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("绑定数据错误", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(bizerrors.CodeBindFailed).
			Failed(fmt.Sprintf("绑定数据错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(bizerrors.CodeLoginRequired).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.BindOJAccount(c.Request.Context(), userID, &req)
	if err != nil {
		code := bizerrors.CodeInternalError
		if errors.Is(err, serviceContract.ErrInvalidPlatform) {
			code = bizerrors.CodeOJPlatformInvalid
		} else if errors.Is(err, serviceContract.ErrInvalidIdentifier) {
			code = bizerrors.CodeOJIdentifierInvalid
		} else if errors.Is(err, serviceContract.ErrBindCoolDown) {
			code = bizerrors.CodeOJBindCoolDown
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
		SetCode(bizerrors.CodeSuccess).
		Success("绑定成功", out)
}

func (ctrl *OJCtrl) GetRankingList(c *gin.Context) {
	var req request.OJRankingListReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(bizerrors.CodeBindFailed).
			Failed(fmt.Sprintf("参数错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(bizerrors.CodeLoginRequired).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.GetRankingList(c.Request.Context(), userID, &req)
	if err != nil {
		code := bizerrors.CodeInternalError
		if errors.Is(err, serviceContract.ErrInvalidPlatform) {
			code = bizerrors.CodeOJPlatformInvalid
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
		SetCode(bizerrors.CodeSuccess).
		Success("获取成功", out)
}

// GetStats 获取用户OJ统计数据，包括总AC数、总提交数、AC题目列表等信息
func (ctrl *OJCtrl) GetStats(c *gin.Context) {
	var req request.OJStatsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("参数绑定失败", zap.Error(err))
		response.NewResponse[any, any](c).
			SetCode(bizerrors.CodeBindFailed).
			Failed(fmt.Sprintf("参数错误: %v", err), nil)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(bizerrors.CodeLoginRequired).
			Failed("用户未登录", &resp.AuthResponse{Message: "用户未登录", Reload: true})
		return
	}

	out, err := ctrl.ojService.GetUserStats(c.Request.Context(), userID, &req)
	if err != nil {
		code := bizerrors.CodeInternalError
		if errors.Is(err, serviceContract.ErrInvalidPlatform) {
			code = bizerrors.CodeOJPlatformInvalid
		} else if errors.Is(err, serviceContract.ErrOJAccountNotBound) {
			code = bizerrors.CodeOJAccountNotBound
		}
		logFields := []zap.Field{
			zap.Uint("user_id", userID),
			zap.String("platform", req.Platform),
			zap.Error(err),
		}
		if errors.Is(err, serviceContract.ErrOJAccountNotBound) {
			global.Log.Warn("用户未绑定 OJ 账号", logFields...)
		} else {
			global.Log.Error("获取用户卡片信息失败", logFields...)
		}
		response.NewResponse[any, any](c).
			SetCode(code).
			Failed(fmt.Sprintf("获取用户卡片信息失败: %v", err), nil)
		return
	}

	response.NewResponse[resp.BindOJAccountResp, resp.BindOJAccountResp](c).
		SetCode(bizerrors.CodeSuccess).
		Success("获取成功", out)
}

func (ctrl *OJCtrl) GetCurve(c *gin.Context) {
	var req request.OJCurveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("刷题曲线参数绑定失败", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, fmt.Sprintf("参数错误: %v", err), c)
		return
	}

	userID := jwt.GetUserID(c)
	if userID == 0 {
		response.BizResult(bizerrors.CodeLoginRequired, gin.H{"reload": true}, "用户未登录", c)
		return
	}

	out, err := ctrl.ojService.GetCurve(c.Request.Context(), userID, &req)
	if err != nil {
		global.Log.Error("获取刷题曲线失败",
			zap.Uint("user_id", userID),
			zap.String("platform", req.Platform),
			zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithData(out, c)
}
