package system

import (
	"personal_assistant/internal/service"
)

type Supplier interface {
	GetRefreshTokenCtrl() *RefreshTokenCtrl
	GetBaseCtrl() *BaseCtrl
	GetHealthCtrl() *HealthCtrl
	GetUserCtrl() *UserCtrl
	GetOrgCtrl() *OrgCtrl
	GetOJCtrl() *OJCtrl
	GetOJTaskCtrl() *OJTaskCtrl
	GetApiCtrl() *ApiCtrl
	GetMenuCtrl() *MenuCtrl
	GetRoleCtrl() *RoleCtrl
	GetImageCtrl() *ImageCtrl
	GetObservabilityCtrl() *ObservabilityCtrl
}

// SetUp 工厂函数-单例
func SetUp(service *service.Group) Supplier {
	cs := &controllerSupplier{}
	cs.refreshTokenCtrl = &RefreshTokenCtrl{
		jwtService: service.SystemServiceSupplier.GetJWTSvc(),
	}
	cs.baseCtrl = &BaseCtrl{
		baseService: service.SystemServiceSupplier.GetBaseSvc(),
	}
	cs.healthCtrl = &HealthCtrl{
		healthService: service.SystemServiceSupplier.GetHealthSvc(),
	}
	cs.userCtrl = &UserCtrl{
		userService: service.SystemServiceSupplier.GetUserSvc(),
		jwtService:  service.SystemServiceSupplier.GetJWTSvc(),
	}
	cs.orgCtrl = &OrgCtrl{
		orgService: service.SystemServiceSupplier.GetOrgSvc(),
	}
	cs.ojCtrl = &OJCtrl{
		ojService: service.SystemServiceSupplier.GetOJSvc(),
	}
	cs.ojTaskCtrl = &OJTaskCtrl{
		ojTaskService: service.SystemServiceSupplier.GetOJTaskSvc(),
	}
	cs.apiCtrl = &ApiCtrl{
		apiService: service.SystemServiceSupplier.GetApiSvc(),
	}
	cs.menuCtrl = &MenuCtrl{
		menuService: service.SystemServiceSupplier.GetMenuSvc(),
	}
	cs.roleCtrl = &RoleCtrl{
		roleService: service.SystemServiceSupplier.GetRoleSvc(),
	}
	cs.imageCtrl = &ImageCtrl{
		imageService: service.SystemServiceSupplier.GetImageSvc(),
	}
	cs.observabilityCtrl = &ObservabilityCtrl{
		observabilityService: service.SystemServiceSupplier.GetObservabilitySvc(),
	}
	return cs
}
