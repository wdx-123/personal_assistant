package system

import "personal_assistant/internal/service/contract"

var (
	_ contract.JWTServiceContract        = (*JWTService)(nil)
	_ contract.PermissionServiceContract = (*PermissionService)(nil)
	_ contract.BaseServiceContract       = (*BaseService)(nil)
	_ contract.HealthServiceContract     = (*HealthService)(nil)
	_ contract.UserServiceContract       = (*UserService)(nil)
	_ contract.OrgServiceContract        = (*OrgService)(nil)
	_ contract.OJServiceContract         = (*OJService)(nil)
	_ contract.ApiServiceContract        = (*ApiService)(nil)
	_ contract.MenuServiceContract       = (*MenuService)(nil)
	_ contract.RoleServiceContract       = (*RoleService)(nil)
	_ contract.ImageServiceContract      = (*ImageService)(nil)
)
