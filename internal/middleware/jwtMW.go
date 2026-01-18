package middleware

import (
	"errors"
	"personal_assistant/global"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
)

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只获取Access Token
		accessToken := jwt.GetAccessToken(c)
		// 检查access Token是否为空
		if accessToken == "" {
			response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
				SetCode(global.StatusUnauthorized).
				Failed("Access token is required", &resp.AuthResponse{
					Message: "Access token is required",
					Reload:  true,
				})
			c.Abort()
			return
		}
		j := jwt.NewJWT()

		// 解析Access Token
		claims, err := j.ParseAccessToken(accessToken)
		if err != nil {
			// 根据错误类型返回不同的响应
			var errName string
			if errors.Is(err, jwt.TokenExpired) {
				errName = "Access token expired, please refresh"
			} else {
				errName = "Invalid access token"
			}
			response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
				SetCode(global.StatusUnauthorized).
				Failed(errName, &resp.AuthResponse{
					Message: errName,
					Reload:  true,
				})
			c.Abort()
			return
		}

		// 设置用户信息到context
		c.Set("claims", claims)
		c.Next()
	}
}
