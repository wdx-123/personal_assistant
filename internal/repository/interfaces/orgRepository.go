package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// OrgRepository 组织仓储接口
type OrgRepository interface {
	// 基础CRUD操作
	GetByID(ctx context.Context, id uint) (*entity.Org, error)
	Create(ctx context.Context, org *entity.Org) error
	Update(ctx context.Context, org *entity.Org) error
	Delete(ctx context.Context, id uint) error

	// 业务相关查询
	GetAllOrgs(ctx context.Context) ([]*entity.Org, error)
	GetOrgList(ctx context.Context, page, pageSize int) ([]*entity.Org, int64, error)
	GetByCode(ctx context.Context, code string) (*entity.Org, error)
}
