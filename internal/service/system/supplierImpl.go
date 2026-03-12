package system

import "personal_assistant/internal/service/contract"

// supplier implementation 用于底层实现
type serviceSupplier struct {
	jwtService             contract.JWTServiceContract
	authorizationService   contract.AuthorizationServiceContract
	permissionProjectionService contract.PermissionProjectionServiceContract
	baseService            contract.BaseServiceContract
	healthService          contract.HealthServiceContract
	userService            contract.UserServiceContract
	orgService             contract.OrgServiceContract
	ojService              contract.OJServiceContract
	apiService             contract.ApiServiceContract
	menuService            contract.MenuServiceContract
	roleService            contract.RoleServiceContract
	imageService           contract.ImageServiceContract
	observabilityService   contract.ObservabilityServiceContract
	cacheProjectionService contract.CacheProjectionServiceContract
}

func (s *serviceSupplier) GetJWTSvc() contract.JWTServiceContract {
	return s.jwtService
}
func (s *serviceSupplier) GetAuthorizationSvc() contract.AuthorizationServiceContract {
	return s.authorizationService
}

func (s *serviceSupplier) GetPermissionProjectionSvc() contract.PermissionProjectionServiceContract {
	return s.permissionProjectionService
}
func (s *serviceSupplier) GetBaseSvc() contract.BaseServiceContract {
	return s.baseService
}
func (s *serviceSupplier) GetHealthSvc() contract.HealthServiceContract {
	return s.healthService
}
func (s *serviceSupplier) GetUserSvc() contract.UserServiceContract {
	return s.userService
}
func (s *serviceSupplier) GetOrgSvc() contract.OrgServiceContract {
	return s.orgService
}

func (s *serviceSupplier) GetOJSvc() contract.OJServiceContract {
	return s.ojService
}

func (s *serviceSupplier) GetApiSvc() contract.ApiServiceContract {
	return s.apiService
}

func (s *serviceSupplier) GetMenuSvc() contract.MenuServiceContract {
	return s.menuService
}

func (s *serviceSupplier) GetRoleSvc() contract.RoleServiceContract {
	return s.roleService
}

func (s *serviceSupplier) GetImageSvc() contract.ImageServiceContract {
	return s.imageService
}

func (s *serviceSupplier) GetObservabilitySvc() contract.ObservabilityServiceContract {
	return s.observabilityService
}

func (s *serviceSupplier) GetCacheProjectionSvc() contract.CacheProjectionServiceContract {
	return s.cacheProjectionService
}
