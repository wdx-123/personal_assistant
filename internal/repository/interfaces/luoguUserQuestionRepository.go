package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
)

// LuoguUserQuestionRepository 洛谷用户做题记录仓储接口
type LuoguUserQuestionRepository interface {
	// GetSolvedProblemIDs 获取指定用户已解决的所有题目ID列表
	GetSolvedProblemIDs(ctx context.Context, luoguUserDetailID uint) (map[uint]struct{}, error)
	// GetSolvedProblemIDsByDetailIDs 批量获取多个详情ID对应的已做题集合
	GetSolvedProblemIDsByDetailIDs(ctx context.Context, luoguUserDetailIDs []uint) (map[uint]map[uint]struct{}, error)
	// BatchCreate 批量创建用户做题记录
	BatchCreate(ctx context.Context, records []*entity.LuoguUserQuestion) error
	// CountSolvedByDateRange 按时间范围统计每天新增做题数
	CountSolvedByDateRange(
		ctx context.Context,
		luoguUserDetailID uint,
		start time.Time,
		end time.Time,
	) ([]*readmodel.DateSolvedCount, error)
}
