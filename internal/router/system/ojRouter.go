package system

import (
	"github.com/gin-gonic/gin"

	"personal_assistant/internal/controller"
)

type OJRouter struct{}

func (r *OJRouter) InitOJRouter(router *gin.RouterGroup) {
	ojRouter := router.Group("oj")
	ojCtrl := controller.ApiGroupApp.SystemApiGroup.GetOJCtrl()
	{
		ojRouter.POST("bind", ojCtrl.BindOJAccount)
		ojRouter.POST("ranking_list", ojCtrl.GetRankingList)
		ojRouter.POST("stats", ojCtrl.GetStats)
	}
}
