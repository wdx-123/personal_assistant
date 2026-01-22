package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// LeetcodeUserDetailRepository 力扣用户详情仓储接口
type LeetcodeUserDetailRepository interface {
	// GetByUserID 根据用户ID获取力扣详情
	GetByUserID(ctx context.Context, userID uint) (*entity.LeetcodeUserDetail, error)
	// GetByUserIDs 批量获取用户力扣详情
	GetByUserIDs(ctx context.Context, userIDs []uint) ([]*entity.LeetcodeUserDetail, error)
	// ListByOrgID 获取指定组织的力扣详情
	ListByOrgID(ctx context.Context, orgID uint) ([]*entity.LeetcodeUserDetail, error)
	// GetAll 获取所有已绑定力扣的用户
	GetAll(ctx context.Context) ([]*entity.LeetcodeUserDetail, error)
	// UpsertByUserID 更新或插入力扣详情
	UpsertByUserID(ctx context.Context, detail *entity.LeetcodeUserDetail) (*entity.LeetcodeUserDetail, error)
	// DeleteByUserID 删除用户的力扣详情
	DeleteByUserID(ctx context.Context, userID uint) error
}
