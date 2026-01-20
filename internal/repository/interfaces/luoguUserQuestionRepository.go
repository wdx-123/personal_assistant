package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// LuoguUserQuestionRepository 洛谷用户做题记录仓储接口
type LuoguUserQuestionRepository interface {
	// GetSolvedProblemIDs 获取指定用户已解决的所有题目ID列表
	GetSolvedProblemIDs(ctx context.Context, luoguUserDetailID uint) (map[uint]struct{}, error)
	// BatchCreate 批量创建用户做题记录
	BatchCreate(ctx context.Context, records []*entity.LuoguUserQuestion) error
}
