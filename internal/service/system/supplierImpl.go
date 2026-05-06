package system

import "personal_assistant/internal/service/contract"

// supplier implementation 用于底层实现
type serviceSupplier struct {
	jwtService                    contract.JWTServiceContract
	authorizationService          contract.AuthorizationServiceContract
	permissionProjectionService   contract.PermissionProjectionServiceContract
	baseService                   contract.BaseServiceContract
	healthService                 contract.HealthServiceContract
	userService                   contract.UserServiceContract
	orgService                    contract.OrgServiceContract
	ojService                     contract.OJServiceContract
	ojTaskService                 contract.OJTaskServiceContract
	apiService                    contract.ApiServiceContract
	menuService                   contract.MenuServiceContract
	roleService                   contract.RoleServiceContract
	imageService                  contract.ImageServiceContract
	observabilityService          contract.ObservabilityServiceContract
	cacheProjectionService        contract.CacheProjectionServiceContract
	ojDailyStatsProjectionService contract.OJDailyStatsProjectionServiceContract
	aiService                     contract.AIServiceContract
}

// GetJWTSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.JWTServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetJWTSvc() contract.JWTServiceContract {
	return s.jwtService
}

// GetAuthorizationSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.AuthorizationServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetAuthorizationSvc() contract.AuthorizationServiceContract {
	return s.authorizationService
}

// GetPermissionProjectionSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.PermissionProjectionServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetPermissionProjectionSvc() contract.PermissionProjectionServiceContract {
	return s.permissionProjectionService
}

// GetBaseSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.BaseServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetBaseSvc() contract.BaseServiceContract {
	return s.baseService
}

// GetHealthSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.HealthServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetHealthSvc() contract.HealthServiceContract {
	return s.healthService
}

// GetUserSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.UserServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetUserSvc() contract.UserServiceContract {
	return s.userService
}

// GetOrgSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.OrgServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetOrgSvc() contract.OrgServiceContract {
	return s.orgService
}

// GetOJSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.OJServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetOJSvc() contract.OJServiceContract {
	return s.ojService
}

// GetOJTaskSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.OJTaskServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetOJTaskSvc() contract.OJTaskServiceContract {
	return s.ojTaskService
}

// GetApiSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.ApiServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetApiSvc() contract.ApiServiceContract {
	return s.apiService
}

// GetMenuSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.MenuServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetMenuSvc() contract.MenuServiceContract {
	return s.menuService
}

// GetRoleSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.RoleServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetRoleSvc() contract.RoleServiceContract {
	return s.roleService
}

// GetImageSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.ImageServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetImageSvc() contract.ImageServiceContract {
	return s.imageService
}

// GetObservabilitySvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.ObservabilityServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetObservabilitySvc() contract.ObservabilityServiceContract {
	return s.observabilityService
}

// GetCacheProjectionSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.CacheProjectionServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetCacheProjectionSvc() contract.CacheProjectionServiceContract {
	return s.cacheProjectionService
}

// GetOJDailyStatsProjectionSvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.OJDailyStatsProjectionServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetOJDailyStatsProjectionSvc() contract.OJDailyStatsProjectionServiceContract {
	return s.ojDailyStatsProjectionService
}

// GetAISvc 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - contract.AIServiceContract：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (s *serviceSupplier) GetAISvc() contract.AIServiceContract {
	return s.aiService
}
