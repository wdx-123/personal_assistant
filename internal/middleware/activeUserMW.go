package middleware

import (
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
)

// ActiveUserMW 校验当前登录用户是否仍处于可用状态。
// 账号被禁用后即使 Access Token 未过期，也会被立即拦截。
func ActiveUserMW() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := jwt.GetUserID(c)
		if userID == 0 {
			response.BizNoAuth("请先登录", c)
			c.Abort()
			return
		}
		if repository.GroupApp == nil || repository.GroupApp.SystemRepositorySupplier == nil {
			response.BizFailWithMessage("系统初始化异常", c)
			c.Abort()
			return
		}

		userRepo := repository.GroupApp.SystemRepositorySupplier.GetUserRepository()
		user, err := userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			response.BizFailWithError(err, c)
			c.Abort()
			return
		}
		if user == nil || user.Freeze || user.Status != consts.UserStatusActive {
			response.BizNoAuth("账号已禁用", c)
			c.Abort()
			return
		}
		c.Next()
	}
}
