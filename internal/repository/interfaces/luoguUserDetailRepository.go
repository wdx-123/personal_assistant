package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// LuoguUserDetailRepository 洛谷用户详情仓储接口
type LuoguUserDetailRepository interface {
	// GetByUserID 根据用户ID获取洛谷详情
	GetByUserID(ctx context.Context, userID uint) (*entity.LuoguUserDetail, error)
	// GetByUserIDs 批量获取用户洛谷详情
	GetByUserIDs(ctx context.Context, userIDs []uint) ([]*entity.LuoguUserDetail, error)
	// ListByOrgID 获取指定组织的洛谷详情
	ListByOrgID(ctx context.Context, orgID uint) ([]*entity.LuoguUserDetail, error)
	// UpsertByUserID 更新或插入洛谷详情
	UpsertByUserID(ctx context.Context, detail *entity.LuoguUserDetail) (*entity.LuoguUserDetail, error)
	// DeleteByUserID 删除用户的洛谷详情
	DeleteByUserID(ctx context.Context, userID uint) error
	// GetAll 获取所有已绑定洛谷的用户
	GetAll(ctx context.Context) ([]*entity.LuoguUserDetail, error)
}
