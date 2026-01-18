package system

import (
	"github.com/gin-gonic/gin"
	"personal_assistant/internal/controller"
)

type RefreshTokenRouter struct{}

func (r *RefreshTokenRouter) InitRefreshTokenRouter(router *gin.RouterGroup) {
	refreshTokenRouter := router.Group("refreshToken")
	refreshTokenApi := controller.ApiGroupApp.SystemApiGroup.GetRefreshTokenCtrl()
	{
		// 刷新Api
		refreshTokenRouter.GET("", refreshTokenApi.RefreshToken)
	}

}
