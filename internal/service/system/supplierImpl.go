package system

// supplier implementation 用于底层实现
type serviceSupplier struct {
	jwtService        *JWTService
	permissionService *PermissionService
	baseService       *BaseService
	userService       *UserService
	orgService        *OrgService
	ojService         *OJService
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
func (s *serviceSupplier) GetUserSvc() *UserService {
	return s.userService
}
func (s *serviceSupplier) GetOrgSvc() *OrgService {
	return s.orgService
}

func (s *serviceSupplier) GetOJSvc() *OJService {
	return s.ojService
}
