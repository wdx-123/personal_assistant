package system

import (
	"errors"
	"github.com/gin-gonic/gin"
	"personal_assistant/global"
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

	RefreshToken := jwt.GetRefreshToken(c)
	if RefreshToken == "" {
		err1 := errors.New("RefreshToken token is required")
		helper.HandleBindError(err1)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
			SetCode(global.StatusBadRequest).Failed(err1.Error(), "未携带 RefreshToken Token")
		return
	}

	// 检查refresh token是否在黑名单中
	if r.jwtService.IsInBlacklist(RefreshToken) {
		helper.CommonError("token is blacklist", global.StatusUnauthorized, nil)
		response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).SetCode(global.StatusUnauthorized).
			Failed("token is blacklist", &resp.AuthResponse{
				Message: "token is blacklist",
				Reload:  true})
		return
	}

	// 获取用户所有信息
	refreshReq, jwtErr := r.jwtService.GetAccessToken(c.Request.Context(), RefreshToken)
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
	response.NewResponse[resp.RefreshTokenResponse, resp.RefreshTokenResponse](c).
		SetTrans(&resp.RefreshTokenResponse{}).
		Success("刷新成功", refreshReq)
}
