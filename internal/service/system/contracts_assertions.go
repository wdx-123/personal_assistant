package system

import "personal_assistant/internal/service/contract"

// 静态类型断言，确保实现了接口
var (
	_ contract.JWTServiceContract                    = (*JWTService)(nil)
	_ contract.AuthorizationServiceContract          = (*AuthorizationService)(nil)
	_ contract.PermissionProjectionServiceContract   = (*PermissionProjectionService)(nil)
	_ contract.BaseServiceContract                   = (*BaseService)(nil)
	_ contract.HealthServiceContract                 = (*HealthService)(nil)
	_ contract.UserServiceContract                   = (*UserService)(nil)
	_ contract.OrgServiceContract                    = (*OrgService)(nil)
	_ contract.OJServiceContract                     = (*OJService)(nil)
	_ contract.OJDailyStatsProjectionServiceContract = (*OJDailyStatsProjectionService)(nil)
	_ contract.CacheProjectionServiceContract        = (*CacheProjectionService)(nil)
	_ contract.ApiServiceContract                    = (*ApiService)(nil)
	_ contract.MenuServiceContract                   = (*MenuService)(nil)
	_ contract.RoleServiceContract                   = (*RoleService)(nil)
	_ contract.ImageServiceContract                  = (*ImageService)(nil)
)
