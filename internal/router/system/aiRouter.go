package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

// AIRouter 负责当前领域相关路由的注册。
type AIRouter struct{}

// InitAIRouter 负责初始化当前模块所需的运行时资源。
// 参数：
//   - router：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - 无。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIRouter) InitAIRouter(router *gin.RouterGroup) {
	aiRouter := router.Group("ai/conversations")
	aiCtrl := controller.ApiGroupApp.SystemApiGroup.GetAICtrl()
	{
		aiRouter.POST("", aiCtrl.CreateConversation) // 创建会话
		aiRouter.GET("", aiCtrl.ListConversations) // 获取会话列表
		aiRouter.GET(":id/messages", aiCtrl.ListMessages) // 获取某个会话下的消息列表
		aiRouter.DELETE(":id", aiCtrl.DeleteConversation) // 删除指定会话
		aiRouter.POST(":id/interrupts/:interrupt_id/decision", aiCtrl.SubmitDecision) // 提交决策
	}
}

// InitAISSERouter 负责初始化当前模块所需的运行时资源。
// 参数：
//   - router：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - 无。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *AIRouter) InitAISSERouter(router *gin.RouterGroup) {
	aiRouter := router.Group("ai/conversations")
	aiCtrl := controller.ApiGroupApp.SystemApiGroup.GetAICtrl()
	{
		aiRouter.POST(":id/stream", aiCtrl.StreamConversation) // 流式会话
	}
}
