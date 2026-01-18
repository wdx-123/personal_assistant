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
	GetMenuRepository() interfaces.MenuRepository
	GetAPIRepository() interfaces.APIRepository
	GetOrgRepository() interfaces.OrgRepository
	GetLeetcodeUserDetailRepository() interfaces.LeetcodeUserDetailRepository
	GetLuoguUserDetailRepository() interfaces.LuoguUserDetailRepository
	GetOutboxRepository() interfaces.OutboxRepository
}

// SetUp 工厂函数，统一管理 - 现在支持配置驱动
func SetUp(factoryConfig *adapter.FactoryConfig) Supplier {
	var userRepo interfaces.UserRepository
	var jwtRepo interfaces.JWTRepository
	var roleRepo interfaces.RoleRepository
	var menuRepo interfaces.MenuRepository
	var apiRepo interfaces.APIRepository
	var orgRepo interfaces.OrgRepository
	var leetcodeUserDetailRepo interfaces.LeetcodeUserDetailRepository
	var luoguUserDetailRepo interfaces.LuoguUserDetailRepository
	var outboxRepo interfaces.OutboxRepository

	switch factoryConfig.DatabaseType {
	case adapter.MySQL:
		if db, ok := factoryConfig.Connection.(*gorm.DB); ok {
			userRepo = NewUserRepository(db)
			jwtRepo = NewJwtRepository(db)
			roleRepo = NewRoleRepository(db)
			menuRepo = NewMenuRepository(db)
			apiRepo = NewAPIRepository(db)
			orgRepo = NewOrgRepository(db)
			leetcodeUserDetailRepo = NewLeetcodeUserDetailRepository(db)
			luoguUserDetailRepo = NewLuoguUserDetailRepository(db)
			outboxRepo = NewOutboxRepository(db)
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
			userRepo = NewUserRepository(db)
			jwtRepo = NewJwtRepository(db)
			roleRepo = NewRoleRepository(db)
			menuRepo = NewMenuRepository(db)
			apiRepo = NewAPIRepository(db)
			orgRepo = NewOrgRepository(db)
			leetcodeUserDetailRepo = NewLeetcodeUserDetailRepository(db)
			luoguUserDetailRepo = NewLuoguUserDetailRepository(db)
			outboxRepo = NewOutboxRepository(db)
		}
	}
	return &RepositorySupplier{
		userRepository:               userRepo,
		jwtRepository:                jwtRepo,
		roleRepository:               roleRepo,
		menuRepository:               menuRepo,
		apiRepository:                apiRepo,
		orgRepository:                orgRepo,
		leetcodeUserDetailRepository: leetcodeUserDetailRepo,
		luoguUserDetailRepository:    luoguUserDetailRepo,
		outboxRepository:             outboxRepo,
	}
}
