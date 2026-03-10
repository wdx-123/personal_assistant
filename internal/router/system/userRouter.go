package system

import (
	"personal_assistant/internal/controller"

	"github.com/gin-gonic/gin"
)

type UserRouter struct{}

// InitUserRouter 初始化用户公共路由（无需JWT）
func (u *UserRouter) InitUserRouter(router *gin.RouterGroup) {
	userRouter := router.Group("user")
	userCtrl := controller.ApiGroupApp.SystemApiGroup.GetUserCtrl()
	{
		userRouter.POST("register", userCtrl.Register) // 注册
		userRouter.POST("login", userCtrl.Login)       // 登录
	}
}

// InitUserBusinessRouter 初始化用户业务路由（需JWT，无严格权限控制）
func (u *UserRouter) InitUserBusinessRouter(router *gin.RouterGroup) {
	userRouter := router.Group("user")
	userCtrl := controller.ApiGroupApp.SystemApiGroup.GetUserCtrl()
	{
		userRouter.POST("logout", userCtrl.Logout)                // 登出
		userRouter.PUT("profile", userCtrl.UpdateProfile)         // 更新个人资料
		userRouter.PUT("phone", userCtrl.ChangePhone)             // 换绑手机号
		userRouter.PUT("password", userCtrl.ChangePassword)       // 修改密码
		userRouter.POST("deactivate", userCtrl.DeactivateAccount) // 主动注销账号
	}
}

// InitUserAuthRouter 初始化用户管理路由（需JWT+权限校验）
func (u *UserRouter) InitUserAuthRouter(router *gin.RouterGroup) {
	userRouter := router.Group("system/user")
	userCtrl := controller.ApiGroupApp.SystemApiGroup.GetUserCtrl()
	{
		userRouter.GET("list", userCtrl.GetUserList)            // 获取用户列表
		userRouter.POST("assign_role", userCtrl.AssignRole)     // 分配角色
		userRouter.GET(":id/roles", userCtrl.GetUserRoles)      // 获取用户角色
		userRouter.GET(":id", userCtrl.GetUserDetail)           // 获取用户详情
		userRouter.PUT(":id/status", userCtrl.UpdateUserStatus) // 管理员 启用/禁用用户
	}
}
