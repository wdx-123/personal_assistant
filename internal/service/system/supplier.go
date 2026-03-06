package system

import (
	"strings"

	"personal_assistant/global"
	obsquery "personal_assistant/internal/observability/query"
	obsdecorator "personal_assistant/internal/observability/trace/decorator"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/service/contract"
)

var defaultServiceTraceModules = []string{
	"jwt",
	"user",
	"oj",
	"image",
	"observability",
}

// SetUp 工厂函数，统一管理
func SetUp(repositoryGroup *repository.Group) contract.Supplier {
	ss := &serviceSupplier{}

	rawJWT := NewJWTService(repositoryGroup)
	rawPermission := NewPermissionService(repositoryGroup)
	rawBase := NewBaseService()
	rawHealth := NewHealthService()
	rawUser := NewUserService(repositoryGroup, rawPermission)
	rawOrg := NewOrgService(repositoryGroup)
	rawOJ := NewOJService(repositoryGroup)
	rawAPI := NewApiService(repositoryGroup, rawPermission)
	rawMenu := NewMenuService(repositoryGroup, rawPermission)
	rawRole := NewRoleService(repositoryGroup, rawPermission)
	rawImage := NewImageService(repositoryGroup)
	rawObservability := obsquery.NewQueryService(
		global.ObservabilityMetrics,
		global.ObservabilityTraces,
		repositoryGroup.SystemRepositorySupplier.GetObservabilityTraceRepository(),
	)

	jwtSvc := contract.JWTServiceContract(rawJWT)
	permissionSvc := contract.PermissionServiceContract(rawPermission)
	baseSvc := contract.BaseServiceContract(rawBase)
	healthSvc := contract.HealthServiceContract(rawHealth)
	userSvc := contract.UserServiceContract(rawUser)
	orgSvc := contract.OrgServiceContract(rawOrg)
	ojSvc := contract.OJServiceContract(rawOJ)
	apiSvc := contract.ApiServiceContract(rawAPI)
	menuSvc := contract.MenuServiceContract(rawMenu)
	roleSvc := contract.RoleServiceContract(rawRole)
	imageSvc := contract.ImageServiceContract(rawImage)
	observabilitySvc := contract.ObservabilityServiceContract(rawObservability)

	if traceModuleEnabled("jwt") {
		jwtSvc = obsdecorator.WrapJWTService(jwtSvc)
	}
	if traceModuleEnabled("user") {
		userSvc = obsdecorator.WrapUserService(userSvc)
	}
	if traceModuleEnabled("oj") {
		ojSvc = obsdecorator.WrapOJService(ojSvc)
	}
	if traceModuleEnabled("image") {
		imageSvc = obsdecorator.WrapImageService(imageSvc)
	}
	if traceModuleEnabled("observability") {
		observabilitySvc = obsdecorator.WrapObservabilityService(observabilitySvc)
	}

	ss.jwtService = jwtSvc
	ss.permissionService = permissionSvc
	ss.baseService = baseSvc
	ss.healthService = healthSvc
	ss.userService = userSvc
	ss.orgService = orgSvc
	ss.ojService = ojSvc
	ss.apiService = apiSvc
	ss.menuService = menuSvc
	ss.roleService = roleSvc
	ss.imageService = imageSvc
	ss.observabilityService = observabilitySvc
	return ss
}

func traceModuleEnabled(module string) bool {
	if global.Config == nil {
		return false
	}
	cfg := global.Config.Observability.ServiceTrace
	if !cfg.Enabled {
		return false
	}

	modules := cfg.Modules
	if len(modules) == 0 {
		modules = defaultServiceTraceModules
	}
	module = strings.ToLower(strings.TrimSpace(module))
	for _, m := range modules {
		if strings.ToLower(strings.TrimSpace(m)) == module {
			return true
		}
	}
	return false
}
