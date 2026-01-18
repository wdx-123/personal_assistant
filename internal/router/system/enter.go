package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

type RouterGroup struct {
	RefreshTokenRouter
	BaseRouter
	UserRouter
	OrgRouter
	OJRouter
}

type OJRouter struct{}

func (r *OJRouter) InitOJRouter(router *gin.RouterGroup) {
	ojRouter := router.Group("oj")
	ojCtrl := controller.ApiGroupApp.SystemApiGroup.GetOJCtrl()
	{
		ojRouter.POST("bind", ojCtrl.BindOJAccount)
	}
}
