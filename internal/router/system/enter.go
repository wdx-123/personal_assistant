// Package system 系统模块路由，包含用户、组织、权限等核心功能
package system

// RouterGroup 系统模块路由聚合器
// 通过组合模式统一管理所有系统级路由，由 router.go 统一调用初始化
type RouterGroup struct {
	// 认证相关
	RefreshTokenRouter // Token刷新路由
	BaseRouter         // 基础路由（登录、注册、验证码等公开接口）

	// 业务模块
	UserRouter // 用户管理路由
	OrgRouter  // 组织管理路由
	OJRouter   // OJ判题模块路由

	// 权限管理
	ApiRouter  // API接口管理路由
	MenuRouter // 菜单管理路由
	RoleRouter // 角色管理路由

	// 资源管理
	ImageRouter // 图片管理路由
}
