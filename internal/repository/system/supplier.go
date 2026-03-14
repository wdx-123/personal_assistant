package system

import (
	"personal_assistant/internal/repository/adapter"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type Supplier interface {
	GetUserRepository() interfaces.UserRepository
	GetJWTRepository() interfaces.JWTRepository
	GetRoleRepository() interfaces.RoleRepository
	GetCapabilityRepository() interfaces.CapabilityRepository
	GetMenuRepository() interfaces.MenuRepository
	GetAPIRepository() interfaces.APIRepository
	GetOrgRepository() interfaces.OrgRepository
	GetOrgMemberRepository() interfaces.OrgMemberRepository
	GetLeetcodeUserDetailRepository() interfaces.LeetcodeUserDetailRepository
	GetLuoguUserDetailRepository() interfaces.LuoguUserDetailRepository
	GetLanqiaoUserDetailRepository() interfaces.LanqiaoUserDetailRepository
	GetLeetcodeQuestionBankRepository() interfaces.LeetcodeQuestionBankRepository
	GetLuoguQuestionBankRepository() interfaces.LuoguQuestionBankRepository
	GetLanqiaoQuestionBankRepository() interfaces.LanqiaoQuestionBankRepository
	GetLeetcodeUserQuestionRepository() interfaces.LeetcodeUserQuestionRepository
	GetLuoguUserQuestionRepository() interfaces.LuoguUserQuestionRepository
	GetLanqiaoUserQuestionRepository() interfaces.LanqiaoUserQuestionRepository
	GetOJDailyStatsRepository() interfaces.OJDailyStatsRepository
	GetOutboxRepository() interfaces.OutboxRepository
	GetRankingReadModelRepository() interfaces.RankingReadModelRepository
	GetImageRepository() interfaces.ImageRepository
	GetObservabilityMetricRepository() interfaces.ObservabilityMetricRepository
	GetObservabilityTraceRepository() interfaces.ObservabilityTraceRepository
	GetObservabilityRuntimeRepository() interfaces.ObservabilityRuntimeRepository
}

// SetUp 工厂函数，统一管理 - 现在支持配置驱动
func SetUp(factoryConfig *adapter.FactoryConfig) Supplier {
	var gormDB *gorm.DB
	var userRepo interfaces.UserRepository
	var jwtRepo interfaces.JWTRepository
	var roleRepo interfaces.RoleRepository
	var capabilityRepo interfaces.CapabilityRepository
	var menuRepo interfaces.MenuRepository
	var apiRepo interfaces.APIRepository
	var orgRepo interfaces.OrgRepository
	var orgMemberRepo interfaces.OrgMemberRepository
	var leetcodeUserDetailRepo interfaces.LeetcodeUserDetailRepository
	var luoguUserDetailRepo interfaces.LuoguUserDetailRepository
	var lanqiaoUserDetailRepo interfaces.LanqiaoUserDetailRepository
	var leetcodeQuestionBankRepo interfaces.LeetcodeQuestionBankRepository
	var luoguQuestionBankRepo interfaces.LuoguQuestionBankRepository
	var lanqiaoQuestionBankRepo interfaces.LanqiaoQuestionBankRepository
	var leetcodeUserQuestionRepo interfaces.LeetcodeUserQuestionRepository
	var luoguUserQuestionRepo interfaces.LuoguUserQuestionRepository
	var lanqiaoUserQuestionRepo interfaces.LanqiaoUserQuestionRepository
	var ojDailyStatsRepo interfaces.OJDailyStatsRepository
	var outboxRepo interfaces.OutboxRepository
	var rankingReadModelRepo interfaces.RankingReadModelRepository
	var imageRepo interfaces.ImageRepository
	var observabilityMetricRepo interfaces.ObservabilityMetricRepository
	var observabilityTraceRepo interfaces.ObservabilityTraceRepository
	var observabilityRuntimeRepo interfaces.ObservabilityRuntimeRepository

	switch factoryConfig.DatabaseType {
	case adapter.MySQL:
		if db, ok := factoryConfig.Connection.(*gorm.DB); ok {
			gormDB = db
			userRepo = NewUserRepository(db)
			jwtRepo = NewJwtRepository(db)
			roleRepo = NewRoleRepository(db)
			capabilityRepo = NewCapabilityRepository(db)
			menuRepo = NewMenuRepository(db)
			apiRepo = NewAPIRepository(db)
			orgRepo = NewOrgRepository(db)
			orgMemberRepo = NewOrgMemberRepository(db)
			leetcodeUserDetailRepo = NewLeetcodeUserDetailRepository(db)
			luoguUserDetailRepo = NewLuoguUserDetailRepository(db)
			lanqiaoUserDetailRepo = NewLanqiaoUserDetailRepository(db)
			leetcodeQuestionBankRepo = NewLeetcodeQuestionBankRepository(db)
			luoguQuestionBankRepo = NewLuoguQuestionBankRepository(db)
			lanqiaoQuestionBankRepo = NewLanqiaoQuestionBankRepository(db)
			leetcodeUserQuestionRepo = NewLeetcodeUserQuestionRepository(db)
			luoguUserQuestionRepo = NewLuoguUserQuestionRepository(db)
			lanqiaoUserQuestionRepo = NewLanqiaoUserQuestionRepository(db)
			ojDailyStatsRepo = NewOJDailyStatsRepository(db)
			outboxRepo = NewOutboxRepository(db)
			rankingReadModelRepo = NewRankingReadModelRepository(db)
			imageRepo = NewImageRepository(db)
			observabilityMetricRepo = NewObservabilityMetricRepository(db)
			observabilityTraceRepo = NewObservabilityTraceRepository(db)
			observabilityRuntimeRepo = NewObservabilityRuntimeRepository(db)
		}
	case adapter.MongoDB:
		// 未来可以添加Mongo	DB实现
		panic("MongoDB not implemented yet")
	case adapter.Redis:
		// 未来可以添加Redis实现
		panic("Redis not implemented yet")
	default:
		// 默认MySQL
		if db, ok := factoryConfig.Connection.(*gorm.DB); ok {
			gormDB = db
			userRepo = NewUserRepository(db)
			jwtRepo = NewJwtRepository(db)
			roleRepo = NewRoleRepository(db)
			capabilityRepo = NewCapabilityRepository(db)
			menuRepo = NewMenuRepository(db)
			apiRepo = NewAPIRepository(db)
			orgRepo = NewOrgRepository(db)
			orgMemberRepo = NewOrgMemberRepository(db)
			leetcodeUserDetailRepo = NewLeetcodeUserDetailRepository(db)
			luoguUserDetailRepo = NewLuoguUserDetailRepository(db)
			lanqiaoUserDetailRepo = NewLanqiaoUserDetailRepository(db)
			leetcodeQuestionBankRepo = NewLeetcodeQuestionBankRepository(db)
			luoguQuestionBankRepo = NewLuoguQuestionBankRepository(db)
			lanqiaoQuestionBankRepo = NewLanqiaoQuestionBankRepository(db)
			leetcodeUserQuestionRepo = NewLeetcodeUserQuestionRepository(db)
			luoguUserQuestionRepo = NewLuoguUserQuestionRepository(db)
			lanqiaoUserQuestionRepo = NewLanqiaoUserQuestionRepository(db)
			ojDailyStatsRepo = NewOJDailyStatsRepository(db)
			outboxRepo = NewOutboxRepository(db)
			rankingReadModelRepo = NewRankingReadModelRepository(db)
			imageRepo = NewImageRepository(db)
			observabilityMetricRepo = NewObservabilityMetricRepository(db)
			observabilityTraceRepo = NewObservabilityTraceRepository(db)
			observabilityRuntimeRepo = NewObservabilityRuntimeRepository(db)
		}
	}
	return &RepositorySupplier{
		db:                             gormDB,
		userRepository:                 userRepo,
		jwtRepository:                  jwtRepo,
		roleRepository:                 roleRepo,
		capabilityRepository:           capabilityRepo,
		menuRepository:                 menuRepo,
		apiRepository:                  apiRepo,
		orgRepository:                  orgRepo,
		orgMemberRepository:            orgMemberRepo,
		leetcodeUserDetailRepository:   leetcodeUserDetailRepo,
		luoguUserDetailRepository:      luoguUserDetailRepo,
		lanqiaoUserDetailRepository:    lanqiaoUserDetailRepo,
		leetcodeQuestionBankRepository: leetcodeQuestionBankRepo,
		luoguQuestionBankRepository:    luoguQuestionBankRepo,
		lanqiaoQuestionBankRepository:  lanqiaoQuestionBankRepo,
		leetcodeUserQuestionRepository: leetcodeUserQuestionRepo,
		luoguUserQuestionRepository:    luoguUserQuestionRepo,
		lanqiaoUserQuestionRepository:  lanqiaoUserQuestionRepo,
		ojDailyStatsRepository:         ojDailyStatsRepo,
		outboxRepository:               outboxRepo,
		rankingReadModelRepository:     rankingReadModelRepo,
		imageRepository:                imageRepo,
		observabilityMetricRepository:  observabilityMetricRepo,
		observabilityTraceRepository:   observabilityTraceRepo,
		observabilityRuntimeRepository: observabilityRuntimeRepo,
	}
}
