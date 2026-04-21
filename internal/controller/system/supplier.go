package system

import (
	"personal_assistant/internal/service"
)

// Supplier 用于集中提供当前模块依赖对象。
type Supplier interface {
	GetAICtrl() *AICtrl
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
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	cs := &controllerSupplier{}
	cs.refreshTokenCtrl = &RefreshTokenCtrl{
		jwtService: service.SystemServiceSupplier.GetJWTSvc(),
	}
	cs.aiCtrl = &AICtrl{
		aiService: service.SystemServiceSupplier.GetAISvc(),
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
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
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
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return cs
}
