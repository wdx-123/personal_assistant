package system

import (
	"context"
	"errors"

	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

// RepositorySupplier 用于集中提供当前模块依赖对象。
type RepositorySupplier struct {
	db                   *gorm.DB
	aiRepository         interfaces.AIRepository
	aiMemoryRepository   interfaces.AIMemoryRepository
	userRepository       interfaces.UserRepository
	jwtRepository        interfaces.JWTRepository
	roleRepository       interfaces.RoleRepository
	capabilityRepository interfaces.CapabilityRepository
	menuRepository       interfaces.MenuRepository
	apiRepository        interfaces.APIRepository
	orgRepository        interfaces.OrgRepository
	orgMemberRepository  interfaces.OrgMemberRepository

	leetcodeUserDetailRepository   interfaces.LeetcodeUserDetailRepository
	luoguUserDetailRepository      interfaces.LuoguUserDetailRepository
	lanqiaoUserDetailRepository    interfaces.LanqiaoUserDetailRepository
	leetcodeQuestionBankRepository interfaces.LeetcodeQuestionBankRepository
	luoguQuestionBankRepository    interfaces.LuoguQuestionBankRepository
	lanqiaoQuestionBankRepository  interfaces.LanqiaoQuestionBankRepository
	leetcodeUserQuestionRepository interfaces.LeetcodeUserQuestionRepository
	luoguUserQuestionRepository    interfaces.LuoguUserQuestionRepository
	lanqiaoUserQuestionRepository  interfaces.LanqiaoUserQuestionRepository
	ojTaskRepository               interfaces.OJTaskRepository
	ojTaskExecutionRepository      interfaces.OJTaskExecutionRepository
	ojDailyStatsRepository         interfaces.OJDailyStatsRepository
	outboxRepository               interfaces.OutboxRepository
	rankingReadModelRepository     interfaces.RankingReadModelRepository
	imageRepository                interfaces.ImageRepository
	observabilityMetricRepository  interfaces.ObservabilityMetricRepository
	observabilityTraceRepository   interfaces.ObservabilityTraceRepository
	observabilityRuntimeRepository interfaces.ObservabilityRuntimeRepository
}

// GetAIRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.AIRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetAIRepository() interfaces.AIRepository {
	return r.aiRepository
}

// GetAIMemoryRepository 返回记忆模块正式仓储依赖。
func (r *RepositorySupplier) GetAIMemoryRepository() interfaces.AIMemoryRepository {
	return r.aiMemoryRepository
}

// GetUserRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.UserRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetUserRepository() interfaces.UserRepository {
	return r.userRepository
}

// GetJWTRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.JWTRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetJWTRepository() interfaces.JWTRepository {
	return r.jwtRepository
}

// GetRoleRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.RoleRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetRoleRepository() interfaces.RoleRepository {
	return r.roleRepository
}

// GetCapabilityRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.CapabilityRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetCapabilityRepository() interfaces.CapabilityRepository {
	return r.capabilityRepository
}

// GetMenuRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.MenuRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetMenuRepository() interfaces.MenuRepository {
	return r.menuRepository
}

// GetAPIRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.APIRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetAPIRepository() interfaces.APIRepository {
	return r.apiRepository
}

// GetOrgRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OrgRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOrgRepository() interfaces.OrgRepository {
	return r.orgRepository
}

// GetOrgMemberRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OrgMemberRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOrgMemberRepository() interfaces.OrgMemberRepository {
	return r.orgMemberRepository
}

// GetLeetcodeUserDetailRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LeetcodeUserDetailRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLeetcodeUserDetailRepository() interfaces.LeetcodeUserDetailRepository {
	return r.leetcodeUserDetailRepository
}

// GetLuoguUserDetailRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LuoguUserDetailRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLuoguUserDetailRepository() interfaces.LuoguUserDetailRepository {
	return r.luoguUserDetailRepository
}

// GetLanqiaoUserDetailRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LanqiaoUserDetailRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLanqiaoUserDetailRepository() interfaces.LanqiaoUserDetailRepository {
	return r.lanqiaoUserDetailRepository
}

// GetLeetcodeQuestionBankRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LeetcodeQuestionBankRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLeetcodeQuestionBankRepository() interfaces.LeetcodeQuestionBankRepository {
	return r.leetcodeQuestionBankRepository
}

// GetLeetcodeUserQuestionRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LeetcodeUserQuestionRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLeetcodeUserQuestionRepository() interfaces.LeetcodeUserQuestionRepository {
	return r.leetcodeUserQuestionRepository
}

// GetLuoguQuestionBankRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LuoguQuestionBankRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLuoguQuestionBankRepository() interfaces.LuoguQuestionBankRepository {
	return r.luoguQuestionBankRepository
}

// GetLanqiaoQuestionBankRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LanqiaoQuestionBankRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLanqiaoQuestionBankRepository() interfaces.LanqiaoQuestionBankRepository {
	return r.lanqiaoQuestionBankRepository
}

// GetLuoguUserQuestionRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LuoguUserQuestionRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLuoguUserQuestionRepository() interfaces.LuoguUserQuestionRepository {
	return r.luoguUserQuestionRepository
}

// GetLanqiaoUserQuestionRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.LanqiaoUserQuestionRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetLanqiaoUserQuestionRepository() interfaces.LanqiaoUserQuestionRepository {
	return r.lanqiaoUserQuestionRepository
}

// GetOJTaskRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OJTaskRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOJTaskRepository() interfaces.OJTaskRepository {
	return r.ojTaskRepository
}

// GetOJTaskExecutionRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OJTaskExecutionRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOJTaskExecutionRepository() interfaces.OJTaskExecutionRepository {
	return r.ojTaskExecutionRepository
}

// GetOutboxRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OutboxRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOutboxRepository() interfaces.OutboxRepository {
	return r.outboxRepository
}

// GetOJDailyStatsRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.OJDailyStatsRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetOJDailyStatsRepository() interfaces.OJDailyStatsRepository {
	return r.ojDailyStatsRepository
}

// GetRankingReadModelRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.RankingReadModelRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetRankingReadModelRepository() interfaces.RankingReadModelRepository {
	return r.rankingReadModelRepository
}

// GetImageRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.ImageRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetImageRepository() interfaces.ImageRepository {
	return r.imageRepository
}

// GetObservabilityMetricRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.ObservabilityMetricRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetObservabilityMetricRepository() interfaces.ObservabilityMetricRepository {
	return r.observabilityMetricRepository
}

// GetObservabilityTraceRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.ObservabilityTraceRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetObservabilityTraceRepository() interfaces.ObservabilityTraceRepository {
	return r.observabilityTraceRepository
}

// GetObservabilityRuntimeRepository 用于获取当前场景需要的对象或数据。
// 参数：
//   - 无。
//
// 返回值：
//   - interfaces.ObservabilityRuntimeRepository：当前函数返回的处理结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) GetObservabilityRuntimeRepository() interfaces.ObservabilityRuntimeRepository {
	return r.observabilityRuntimeRepository
}

// InTx 在事务边界内执行回调逻辑，并把提交与回滚交给底层实现处理。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//   - fn：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) InTx(ctx context.Context, fn func(tx any) error) error {
	if r == nil || r.db == nil {
		return errors.New("repository db is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

// Ping 用于探测底层依赖当前是否可用。
// 参数：
//   - ctx：链路上下文，用于取消、超时控制和日志透传。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (r *RepositorySupplier) Ping(ctx context.Context) error {
	if r == nil || r.db == nil {
		return errors.New("repository db is nil")
	}
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	if ctx == nil {
		return sqlDB.Ping()
	}
	return sqlDB.PingContext(ctx)
}

var _ interface {
	Ping(context.Context) error
	InTx(context.Context, func(any) error) error
} = (*RepositorySupplier)(nil)
