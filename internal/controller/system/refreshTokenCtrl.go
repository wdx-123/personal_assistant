package system

import (
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	serviceSystem "personal_assistant/internal/service/system"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"
)

type RefreshTokenCtrl struct {
	jwtService *serviceSystem.JWTService
}

func (r *RefreshTokenCtrl) RefreshToken(c *gin.Context) {
	helper := response.NewAPIHelper(c, "RefreshToken")

	refreshToken := jwt.GetRefreshToken(c)
	if refreshToken == "" {
		var req request.RefreshTokenRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			refreshToken = strings.TrimSpace(req.RefreshToken)
			if refreshToken == "" {
				refreshToken = strings.TrimSpace(req.XRefreshToken)
			}
		} else if err != io.EOF {
			helper.HandleBindError(err)
			response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
				SetCode(global.StatusBadRequest).Failed(err.Error(), "参数错误")
			return
		}
	}
	if refreshToken == "" {
		err1 := errors.New("RefreshToken token is required")
		helper.HandleBindError(err1)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(global.StatusBadRequest).Failed(err1.Error(), "未携带 RefreshToken Token")
		return
	}

	// 检查refresh token是否在黑名单中
	if r.jwtService.IsInBlacklist(refreshToken) {
		helper.CommonError("token is blacklist", global.StatusUnauthorized, nil)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).SetCode(global.StatusUnauthorized).
			Failed("token is blacklist", &resp.AuthResponse{
				Message: "token is blacklist",
				Reload:  true})
		return
	}

	// 获取用户所有信息
	refreshReq, jwtErr := r.jwtService.GetAccessToken(c.Request.Context(), refreshToken)
	if jwtErr != nil {
		helper.HandleJWTError(jwtErr)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).SetCode(jwtErr.Code).
			Failed(jwtErr.Message, &resp.AuthResponse{
				Message: jwtErr.Message,
				Reload:  true,
			})
		return
	}

	// 创建令牌
	if refreshReq != nil {
		refreshReq.RefreshToken = refreshToken
	}
	response.NewResponse[resp.RefreshTokenResponse, resp.RefreshTokenResponse](c).
		SetTrans(&resp.RefreshTokenResponse{}).
		Success("刷新成功", refreshReq)
}
