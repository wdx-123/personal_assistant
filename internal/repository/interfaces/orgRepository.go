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
}
