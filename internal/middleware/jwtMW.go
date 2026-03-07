package middleware

import (
	"errors"

	resp "personal_assistant/internal/model/dto/response"
	bizerrors "personal_assistant/pkg/errors"
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
				SetCode(bizerrors.CodeLoginRequired).
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
			code := bizerrors.CodeTokenInvalid
			if errors.Is(err, jwt.TokenExpired) {
				errName = "Access token expired, please refresh"
				code = bizerrors.CodeTokenExpired
			} else {
				errName = "Invalid access token"
			}
			response.NewResponse[resp.AuthResponse, resp.AuthResponse](c).
				SetCode(code).
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
