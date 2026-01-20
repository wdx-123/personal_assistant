package system

import "personal_assistant/internal/repository/interfaces"

type RepositorySupplier struct {
	userRepository interfaces.UserRepository
	jwtRepository  interfaces.JWTRepository
	roleRepository interfaces.RoleRepository
	menuRepository interfaces.MenuRepository
	apiRepository  interfaces.APIRepository
	orgRepository  interfaces.OrgRepository

	leetcodeUserDetailRepository interfaces.LeetcodeUserDetailRepository
	luoguUserDetailRepository    interfaces.LuoguUserDetailRepository
	luoguQuestionBankRepository  interfaces.LuoguQuestionBankRepository
	luoguUserQuestionRepository  interfaces.LuoguUserQuestionRepository
	outboxRepository             interfaces.OutboxRepository
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

func (r *RepositorySupplier) GetMenuRepository() interfaces.MenuRepository {
	return r.menuRepository
}

func (r *RepositorySupplier) GetAPIRepository() interfaces.APIRepository {
	return r.apiRepository
}

func (r *RepositorySupplier) GetOrgRepository() interfaces.OrgRepository {
	return r.orgRepository
}

func (r *RepositorySupplier) GetLeetcodeUserDetailRepository() interfaces.LeetcodeUserDetailRepository {
	return r.leetcodeUserDetailRepository
}

func (r *RepositorySupplier) GetLuoguUserDetailRepository() interfaces.LuoguUserDetailRepository {
	return r.luoguUserDetailRepository
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
