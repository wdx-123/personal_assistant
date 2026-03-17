package system

import (
	"github.com/gin-gonic/gin"

	"personal_assistant/internal/controller"
)

// OJTaskRouter OJ 任务路由
type OJTaskRouter struct{}

// InitOJTaskRouter 初始化 OJ 任务业务路由（需JWT，无严格权限控制）
// 挂载到 BusinessGroup
// 路由前缀: /oj/task
func (r *OJTaskRouter) InitOJTaskRouter(router *gin.RouterGroup) {
	ojTaskRouter := router.Group("oj/task")
	ojTaskCtrl := controller.ApiGroupApp.SystemApiGroup.GetOJTaskCtrl()
	{
		// 分析与列表接口（静态路由需在 :id 前注册，避免路径冲突）
		// POST /analyze - 分析题目标题并返回候选信息
		ojTaskRouter.POST("analyze", ojTaskCtrl.AnalyzeTaskTitles)
		// POST / - 创建任务
		ojTaskRouter.POST("", ojTaskCtrl.CreateTask)
		// GET /list - 查询当前用户可见的任务列表
		ojTaskRouter.GET("list", ojTaskCtrl.GetVisibleTaskList)

		// 任务写操作
		// PUT /:id - 更新任务
		ojTaskRouter.PUT(":id", ojTaskCtrl.UpdateTask)
		// DELETE /:id - 删除任务
		ojTaskRouter.DELETE(":id", ojTaskCtrl.DeleteTask)
		// POST /:id/execute-now - 立即执行任务
		ojTaskRouter.POST(":id/execute-now", ojTaskCtrl.ExecuteTaskNow)
		// POST /:id/revise - 基于当前任务派生新版本
		ojTaskRouter.POST(":id/revise", ojTaskCtrl.ReviseTask)
		// POST /:id/retry - 重试任务
		ojTaskRouter.POST(":id/retry", ojTaskCtrl.RetryTask)

		// 版本与执行记录查询
		// GET /:id/versions - 查询任务版本列表
		ojTaskRouter.GET(":id/versions", ojTaskCtrl.GetTaskVersions)
		// GET /:id/executions/:executionId - 查询任务执行详情
		ojTaskRouter.GET(":id/executions/:executionId", ojTaskCtrl.GetTaskExecutionDetail)
		// GET /:id/executions/:executionId/users - 分页查询执行用户列表
		ojTaskRouter.GET(":id/executions/:executionId/users", ojTaskCtrl.GetTaskExecutionUsers)
		// GET /:id/executions/:executionId/users/:userId - 查询指定用户的执行详情
		ojTaskRouter.GET(":id/executions/:executionId/users/:userId", ojTaskCtrl.GetTaskExecutionUserDetail)

		// GET /:id - 查询任务详情
		ojTaskRouter.GET(":id", ojTaskCtrl.GetTaskDetail)
	}
}
