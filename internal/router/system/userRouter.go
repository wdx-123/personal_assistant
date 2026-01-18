package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

type UserRouter struct{}

func (u *UserRouter) InitUserRouter(router *gin.RouterGroup) {
	userRouter := router.Group("user")
	userCtrl := controller.ApiGroupApp.SystemApiGroup.GetUserCtrl()
	{
		userRouter.POST("register", userCtrl.Register) // 注册
		userRouter.POST("login", userCtrl.Login)       // 登录
		userRouter.POST("logout", userCtrl.Logout)     // 登出
	}
}
