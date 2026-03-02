package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LeetcodeUserQuestionRepository interface {
	GetSolvedProblemIDs(ctx context.Context, leetcodeUserDetailID uint) (map[uint]struct{}, error)
	BatchCreate(ctx context.Context, records []*entity.LeetcodeUserQuestion) error
}
