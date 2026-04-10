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
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	ss := &serviceSupplier{}

	rawJWT := NewJWTService(repositoryGroup)
	rawAuthorization := NewAuthorizationService(repositoryGroup)
	rawPermissionProjection := NewPermissionProjectionService(repositoryGroup)
	rawBase := NewBaseService()
	rawHealth := NewHealthService(repositoryGroup)
	rawCacheProjection := NewCacheProjectionService(repositoryGroup)
	rawOJDailyStatsProjection := NewOJDailyStatsProjectionService(repositoryGroup)
	rawUser := NewUserService(repositoryGroup, rawAuthorization, rawPermissionProjection)
	rawOrg := NewOrgService(repositoryGroup, rawAuthorization, rawPermissionProjection)
	rawOJ := NewOJService(repositoryGroup, rawCacheProjection, rawOJDailyStatsProjection)
	rawOJTask := NewOJTaskService(repositoryGroup, rawAuthorization)
	rawAPI := NewApiService(repositoryGroup, rawPermissionProjection)
	rawMenu := NewMenuService(repositoryGroup, rawPermissionProjection)
	rawRole := NewRoleService(repositoryGroup, rawPermissionProjection)
	rawImage := NewImageService(repositoryGroup)
	rawAI := NewAIService(repositoryGroup)
	rawObservability := obsquery.NewQueryService(
		global.ObservabilityMetrics,
		global.ObservabilityTraces,
		repositoryGroup.SystemRepositorySupplier.GetObservabilityTraceRepository(),
		repositoryGroup.SystemRepositorySupplier.GetObservabilityRuntimeRepository(),
	)

	jwtSvc := contract.JWTServiceContract(rawJWT)
	authorizationSvc := contract.AuthorizationServiceContract(rawAuthorization)
	permissionProjectionSvc := contract.PermissionProjectionServiceContract(rawPermissionProjection)
	baseSvc := contract.BaseServiceContract(rawBase)
	healthSvc := contract.HealthServiceContract(rawHealth)
	userSvc := contract.UserServiceContract(rawUser)
	orgSvc := contract.OrgServiceContract(rawOrg)
	ojSvc := contract.OJServiceContract(rawOJ)
	ojTaskSvc := contract.OJTaskServiceContract(rawOJTask)
	apiSvc := contract.ApiServiceContract(rawAPI)
	menuSvc := contract.MenuServiceContract(rawMenu)
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	roleSvc := contract.RoleServiceContract(rawRole)
	imageSvc := contract.ImageServiceContract(rawImage)
	aiSvc := contract.AIServiceContract(rawAI)
	observabilitySvc := contract.ObservabilityServiceContract(rawObservability)
	cacheProjectionSvc := contract.CacheProjectionServiceContract(rawCacheProjection)
	ojDailyStatsProjectionSvc := contract.OJDailyStatsProjectionServiceContract(rawOJDailyStatsProjection)

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
	ss.authorizationService = authorizationSvc
	ss.permissionProjectionService = permissionProjectionSvc
	ss.baseService = baseSvc
	ss.healthService = healthSvc
	ss.userService = userSvc
	ss.orgService = orgSvc
	ss.ojService = ojSvc
	ss.ojTaskService = ojTaskSvc
	ss.apiService = apiSvc
	ss.menuService = menuSvc
	ss.roleService = roleSvc
	ss.imageService = imageSvc
	ss.aiService = aiSvc
	ss.observabilityService = observabilitySvc
	ss.cacheProjectionService = cacheProjectionSvc
	ss.ojDailyStatsProjectionService = ojDailyStatsProjectionSvc
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return ss
}

// traceModuleEnabled 负责执行当前函数对应的核心逻辑。
// 参数：
//   - module：当前函数需要消费的输入参数。
//
// 返回值：
//   - bool：表示当前操作是否成功、命中或可继续执行。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func traceModuleEnabled(module string) bool {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if global.Config == nil {
		return false
	}
	cfg := global.Config.Observability.ServiceTrace
	if !cfg.Enabled {
		return false
	}

	modules := cfg.Modules
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if len(modules) == 0 {
		modules = defaultServiceTraceModules
	}
	module = strings.ToLower(strings.TrimSpace(module))
	for _, m := range modules {
		if strings.ToLower(strings.TrimSpace(m)) == module {
			return true
		}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return false
}
