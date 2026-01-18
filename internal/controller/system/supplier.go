package system

import (
	"personal_assistant/internal/service"
)

type Supplier interface {
	GetRefreshTokenCtrl() *RefreshTokenCtrl
	GetBaseCtrl() *BaseCtrl
	GetUserCtrl() *UserCtrl
	GetOrgCtrl() *OrgCtrl
	GetOJCtrl() *OJCtrl
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
	return cs
}
