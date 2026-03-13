package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
)

type LeetcodeUserQuestionRepository interface {
	GetSolvedProblemIDs(ctx context.Context, leetcodeUserDetailID uint) (map[uint]struct{}, error)
	BatchCreate(ctx context.Context, records []*entity.LeetcodeUserQuestion) error
	CountSolvedByDateRange(
		ctx context.Context,
		leetcodeUserDetailID uint,
		start time.Time,
		end time.Time,
	) ([]*readmodel.DateSolvedCount, error)
}
