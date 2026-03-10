package middleware

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

		// 先尝试从缓存读取用户活跃态，避免每次请求都访问数据库
		userRepo := repository.GroupApp.SystemRepositorySupplier.GetUserRepository()
		active, found, err := userRepo.GetCachedActiveState(c.Request.Context(), userID)
		if err != nil {
			global.Log.Error("读取用户活跃态缓存失败", zap.Uint("userID", userID), zap.Error(err))
		} else if found {
			if !active {
				response.BizNoAuth("账号已禁用", c)
				c.Abort()
				return
			}
			c.Next()
			return
		}

		user, err := userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			response.BizFailWithError(err, c)
			c.Abort()
			return
		}

		active = isUserActive(user)
		if err := userRepo.CacheActiveState(c.Request.Context(), userID, active); err != nil {
			global.Log.Error("回填用户活跃态缓存失败", zap.Uint("userID", userID), zap.Error(err))
		}

		if !active {
			response.BizNoAuth("账号已禁用", c)
			c.Abort()
			return
		}
		c.Next()
	}
}

func isUserActive(user *entity.User) bool {
	return user != nil && !user.Freeze && user.Status == consts.UserStatusActive
}
