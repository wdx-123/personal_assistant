package interfaces

import (
	"context"

	readmodel "personal_assistant/internal/model/readmodel"
)

// RankingReadModelRepository 负责排行榜/详情缓存所需的只读聚合查询。
type RankingReadModelRepository interface {
	// GetByUserID 根据用户 ID 获取单个排行榜读模型数据，返回 nil 代表用户不存在或无效。
	GetByUserID(ctx context.Context, userID uint) (*readmodel.Ranking, error)

	// GetByUserIDs 根据多个用户 ID 批量获取排行榜读模型数据，返回的列表可能包含 nil 元素代表对应用户不存在或无效。
	GetByUserIDs(ctx context.Context, userIDs []uint) ([]*readmodel.Ranking, error)

	// ListAll 获取所有用户的排行榜读模型数据，通常用于全量缓存重建等场景
	ListAll(ctx context.Context) ([]*readmodel.Ranking, error)
}
