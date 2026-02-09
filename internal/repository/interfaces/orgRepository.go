package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// OrgRepository 组织仓储接口
type OrgRepository interface {
	// GetByID 根据ID获取组织
	GetByID(ctx context.Context, id uint) (*entity.Org, error)
	// Create 创建组织
	Create(ctx context.Context, org *entity.Org) error
	// Update 更新组织
	Update(ctx context.Context, org *entity.Org) error
	// Delete 删除组织
	Delete(ctx context.Context, id uint) error

	// GetAllOrgs 获取所有组织
	GetAllOrgs(ctx context.Context) ([]*entity.Org, error)
	// GetOrgList 获取组织列表（分页）
	GetOrgList(ctx context.Context, page, pageSize int) ([]*entity.Org, int64, error)
	// GetByCode 根据邀请码获取组织
	GetByCode(ctx context.Context, code string) (*entity.Org, error)

	// ExistsByName 检查组织名称是否已存在
	ExistsByName(ctx context.Context, name string) (bool, error)
	// CountMembersByOrgID 查询组织下的成员数（user_org_roles 表去重 user_id）
	CountMembersByOrgID(ctx context.Context, orgID uint) (int64, error)
	// GetOrgsByUserID 获取用户所属的组织列表（通过 user_org_roles 关联查询）
	GetOrgsByUserID(ctx context.Context, userID uint) ([]*entity.Org, error)
	// GetOrgListWithKeyword 支持关键词搜索的分页查询
	GetOrgListWithKeyword(ctx context.Context, page, pageSize int, keyword string) ([]*entity.Org, int64, error)
	// IsUserInOrg 检查用户是否属于指定组织
	IsUserInOrg(ctx context.Context, userID, orgID uint) (bool, error)
}
