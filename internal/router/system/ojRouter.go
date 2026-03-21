package system

import (
	"github.com/gin-gonic/gin"

	"personal_assistant/internal/controller"
)

type OJRouter struct{}

func (r *OJRouter) InitOJRouter(router *gin.RouterGroup, bindRateLimitMW gin.HandlerFunc) {
	ojRouter := router.Group("oj")
	ojCtrl := controller.ApiGroupApp.SystemApiGroup.GetOJCtrl()
	{
		ojRouter.POST("bind", bindRateLimitMW, ojCtrl.BindOJAccount) // 绑定OJ账号接口
		ojRouter.POST("lanqiao/bind", bindRateLimitMW, ojCtrl.BindLanqiaoAccount)
		ojRouter.POST("ranking_list", ojCtrl.GetRankingList) // 获取OJ排行榜接口
		ojRouter.POST("stats", ojCtrl.GetStats)              // 获取OJ统计数据接口
		ojRouter.POST("curve", ojCtrl.GetCurve)              // 新增获取成绩曲线接口
	}
}
