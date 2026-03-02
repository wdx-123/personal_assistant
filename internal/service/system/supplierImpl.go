package system

// supplier implementation 用于底层实现
type serviceSupplier struct {
	jwtService        *JWTService
	permissionService *PermissionService
	baseService       *BaseService
	healthService     *HealthService
	userService       *UserService
	orgService        *OrgService
	ojService         *OJService
	apiService        *ApiService
	menuService       *MenuService
	roleService       *RoleService
	imageService      *ImageService
}

func (s *serviceSupplier) GetJWTSvc() *JWTService {
	return s.jwtService
}
func (s *serviceSupplier) GetPermissionSvc() *PermissionService {
	return s.permissionService
}
func (s *serviceSupplier) GetBaseSvc() *BaseService {
	return s.baseService
}
func (s *serviceSupplier) GetHealthSvc() *HealthService {
	return s.healthService
}
func (s *serviceSupplier) GetUserSvc() *UserService {
	return s.userService
}
func (s *serviceSupplier) GetOrgSvc() *OrgService {
	return s.orgService
}

func (s *serviceSupplier) GetOJSvc() *OJService {
	return s.ojService
}

func (s *serviceSupplier) GetApiSvc() *ApiService {
	return s.apiService
}

func (s *serviceSupplier) GetMenuSvc() *MenuService {
	return s.menuService
}

func (s *serviceSupplier) GetRoleSvc() *RoleService {
	return s.roleService
}

func (s *serviceSupplier) GetImageSvc() *ImageService {
	return s.imageService
}
