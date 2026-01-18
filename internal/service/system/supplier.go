package system

import "personal_assistant/internal/repository"

type Supplier interface {
	GetJWTSvc() *JWTService
	GetPermissionSvc() *PermissionService
	GetBaseSvc() *BaseService
	GetUserSvc() *UserService
	GetOrgSvc() *OrgService
	GetOJSvc() *OJService
}

// SetUp 工厂函数，统一管理
func SetUp(repositoryGroup *repository.Group) Supplier {
	ss := &serviceSupplier{}
	ss.jwtService = NewJWTService(repositoryGroup)
	ss.permissionService = NewPermissionService(repositoryGroup)
	ss.baseService = NewBaseService() // 用不到repo层
	ss.orgService = NewOrgService(repositoryGroup)
	ss.ojService = NewOJService(repositoryGroup)

	// UserService 需要依赖 PermissionService，所以在 permissionService 初始化后创建
	// 创建用户服务（注入权限服务）
	ss.userService = NewUserService(repositoryGroup, ss.permissionService)
	return ss
}
