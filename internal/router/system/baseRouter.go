package system

import (
	"github.com/gin-gonic/gin"
	"personal_assistant/internal/controller"
)

type BaseRouter struct{}

func (b *BaseRouter) InitBaseRouter(Router *gin.RouterGroup) {
	baseRouter := Router.Group("base")

	baseCtrl := controller.ApiGroupApp.SystemApiGroup.GetBaseCtrl()
	{
		baseRouter.POST("captcha", baseCtrl.Captcha)                                     // 获取验证码
		baseRouter.POST("sendEmailVerificationCode", baseCtrl.SendEmailVerificationCode) // 验证码
	}

}
