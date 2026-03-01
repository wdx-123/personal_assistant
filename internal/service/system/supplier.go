package system

import "personal_assistant/internal/repository"

type Supplier interface {
	GetJWTSvc() *JWTService
	GetPermissionSvc() *PermissionService
	GetBaseSvc() *BaseService
	GetUserSvc() *UserService
	GetOrgSvc() *OrgService
	GetOJSvc() *OJService
	GetApiSvc() *ApiService
	GetMenuSvc() *MenuService
	GetRoleSvc() *RoleService
	GetImageSvc() *ImageService
}

// SetUp 工厂函数，统一管理
func SetUp(repositoryGroup *repository.Group) Supplier {
	ss := &serviceSupplier{}
	ss.jwtService = NewJWTService(repositoryGroup)
	ss.permissionService = NewPermissionService(repositoryGroup)
	ss.baseService = NewBaseService()
	ss.orgService = NewOrgService(repositoryGroup)
	ss.ojService = NewOJService(repositoryGroup)
	ss.apiService = NewApiService(repositoryGroup, ss.permissionService)
	ss.menuService = NewMenuService(repositoryGroup, ss.permissionService)
	ss.roleService = NewRoleService(repositoryGroup, ss.permissionService)
	ss.imageService = NewImageService(repositoryGroup)

	// UserService 需要依赖 PermissionService，所以在 permissionService 初始化后创建
	ss.userService = NewUserService(repositoryGroup, ss.permissionService)
	return ss
}
