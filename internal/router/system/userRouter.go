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

		// 用户设置
		userRouter.PUT("profile", userCtrl.UpdateProfile)   // 更新个人资料
		userRouter.PUT("phone", userCtrl.ChangePhone)       // 换绑手机号
		userRouter.PUT("password", userCtrl.ChangePassword) // 修改密码

		// 用户管理（进阶功能）
		userRouter.GET("list", userCtrl.GetUserList)        // 获取用户列表
		userRouter.GET(":id", userCtrl.GetUserDetail)       // 获取用户详情
		userRouter.GET(":id/roles", userCtrl.GetUserRoles)  // 获取用户角色
		userRouter.POST("assign_role", userCtrl.AssignRole) // 分配角色
	}
}
