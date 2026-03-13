package system

import (
	"context"
	"errors"

	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type RepositorySupplier struct {
	db                   *gorm.DB
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
	leetcodeQuestionBankRepository interfaces.LeetcodeQuestionBankRepository
	luoguQuestionBankRepository    interfaces.LuoguQuestionBankRepository
	leetcodeUserQuestionRepository interfaces.LeetcodeUserQuestionRepository
	luoguUserQuestionRepository    interfaces.LuoguUserQuestionRepository
	ojDailyStatsRepository         interfaces.OJDailyStatsRepository
	outboxRepository               interfaces.OutboxRepository
	rankingReadModelRepository     interfaces.RankingReadModelRepository
	imageRepository                interfaces.ImageRepository
	observabilityMetricRepository  interfaces.ObservabilityMetricRepository
	observabilityTraceRepository   interfaces.ObservabilityTraceRepository
	observabilityRuntimeRepository interfaces.ObservabilityRuntimeRepository
}

func (r *RepositorySupplier) GetUserRepository() interfaces.UserRepository {
	return r.userRepository
}

func (r *RepositorySupplier) GetJWTRepository() interfaces.JWTRepository {
	return r.jwtRepository
}

func (r *RepositorySupplier) GetRoleRepository() interfaces.RoleRepository {
	return r.roleRepository
}

func (r *RepositorySupplier) GetCapabilityRepository() interfaces.CapabilityRepository {
	return r.capabilityRepository
}

func (r *RepositorySupplier) GetMenuRepository() interfaces.MenuRepository {
	return r.menuRepository
}

func (r *RepositorySupplier) GetAPIRepository() interfaces.APIRepository {
	return r.apiRepository
}

func (r *RepositorySupplier) GetOrgRepository() interfaces.OrgRepository {
	return r.orgRepository
}

func (r *RepositorySupplier) GetOrgMemberRepository() interfaces.OrgMemberRepository {
	return r.orgMemberRepository
}

func (r *RepositorySupplier) GetLeetcodeUserDetailRepository() interfaces.LeetcodeUserDetailRepository {
	return r.leetcodeUserDetailRepository
}

func (r *RepositorySupplier) GetLuoguUserDetailRepository() interfaces.LuoguUserDetailRepository {
	return r.luoguUserDetailRepository
}

func (r *RepositorySupplier) GetLeetcodeQuestionBankRepository() interfaces.LeetcodeQuestionBankRepository {
	return r.leetcodeQuestionBankRepository
}

func (r *RepositorySupplier) GetLeetcodeUserQuestionRepository() interfaces.LeetcodeUserQuestionRepository {
	return r.leetcodeUserQuestionRepository
}

func (r *RepositorySupplier) GetLuoguQuestionBankRepository() interfaces.LuoguQuestionBankRepository {
	return r.luoguQuestionBankRepository
}

func (r *RepositorySupplier) GetLuoguUserQuestionRepository() interfaces.LuoguUserQuestionRepository {
	return r.luoguUserQuestionRepository
}

func (r *RepositorySupplier) GetOutboxRepository() interfaces.OutboxRepository {
	return r.outboxRepository
}

func (r *RepositorySupplier) GetOJDailyStatsRepository() interfaces.OJDailyStatsRepository {
	return r.ojDailyStatsRepository
}

func (r *RepositorySupplier) GetRankingReadModelRepository() interfaces.RankingReadModelRepository {
	return r.rankingReadModelRepository
}

func (r *RepositorySupplier) GetImageRepository() interfaces.ImageRepository {
	return r.imageRepository
}

func (r *RepositorySupplier) GetObservabilityMetricRepository() interfaces.ObservabilityMetricRepository {
	return r.observabilityMetricRepository
}

func (r *RepositorySupplier) GetObservabilityTraceRepository() interfaces.ObservabilityTraceRepository {
	return r.observabilityTraceRepository
}

func (r *RepositorySupplier) GetObservabilityRuntimeRepository() interfaces.ObservabilityRuntimeRepository {
	return r.observabilityRuntimeRepository
}

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
