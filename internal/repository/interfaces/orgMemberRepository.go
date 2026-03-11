package interfaces

import (
	"context"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

// OrgMemberRepository 组织成员状态仓储
type OrgMemberRepository interface {
	// 根据组织和用户获取成员信息
	GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*entity.OrgMember, error)
	// 根据组织和用户获取成员信息（用于更新，带行锁）
	GetByOrgAndUserForUpdate(ctx context.Context, orgID, userID uint) (*entity.OrgMember, error)
	// 根据组织ID获取成员列表
	Create(ctx context.Context, member *entity.OrgMember) error
	// 更新成员信息
	Update(ctx context.Context, member *entity.OrgMember) error
	// 设置成员状态（加入、退出、踢出等）
	SetStatus(
		ctx context.Context,
		orgID, userID uint,
		status consts.OrgMemberStatus,
		operatorID *uint,
		reason string,
		joinSource string,
	) error
	// 批量设置组织内所有成员为被踢出状态（如解散组织时）
	SetAllRemovedByOrg(ctx context.Context, orgID uint, operatorID *uint, reason string) error

	// 判断用户在组织内是否活跃（即状态为加入且未过期）
	IsUserActiveInOrg(ctx context.Context, userID, orgID uint) (bool, error)
	// 获取组织内的活跃成员数量
	CountActiveMembersByOrgID(ctx context.Context, orgID uint) (int64, error)
	// 获取用户加入的所有活跃组织ID列表
	ListActiveOrgIDsByUser(ctx context.Context, userID uint) ([]uint, error)
	// 事务上下文切换
	WithTx(tx any) OrgMemberRepository
}
