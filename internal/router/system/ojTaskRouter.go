package system

import (
	"github.com/gin-gonic/gin"

	"personal_assistant/internal/controller"
)

type OJTaskRouter struct{}

func (r *OJTaskRouter) InitOJTaskRouter(router *gin.RouterGroup) {
	ojTaskRouter := router.Group("oj/task")
	ojTaskCtrl := controller.ApiGroupApp.SystemApiGroup.GetOJTaskCtrl()
	{
		ojTaskRouter.POST("", ojTaskCtrl.CreateTask)
		ojTaskRouter.GET("list", ojTaskCtrl.GetVisibleTaskList)
		ojTaskRouter.PUT(":id", ojTaskCtrl.UpdateTask)
		ojTaskRouter.DELETE(":id", ojTaskCtrl.DeleteTask)
		ojTaskRouter.POST(":id/execute-now", ojTaskCtrl.ExecuteTaskNow)
		ojTaskRouter.POST(":id/revise", ojTaskCtrl.ReviseTask)
		ojTaskRouter.POST(":id/retry", ojTaskCtrl.RetryTask)
		ojTaskRouter.GET(":id/versions", ojTaskCtrl.GetTaskVersions)
		ojTaskRouter.GET(":id/executions/:executionId", ojTaskCtrl.GetTaskExecutionDetail)
		ojTaskRouter.GET(":id/executions/:executionId/users", ojTaskCtrl.GetTaskExecutionUsers)
		ojTaskRouter.GET(":id/executions/:executionId/users/:userId", ojTaskCtrl.GetTaskExecutionUserDetail)
		ojTaskRouter.GET(":id", ojTaskCtrl.GetTaskDetail)
	}
}
