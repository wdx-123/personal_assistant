package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
)

type LanqiaoUserQuestionRepository interface {
	WithTx(tx any) LanqiaoUserQuestionRepository
	GetSolvedProblemIDs(ctx context.Context, lanqiaoUserDetailID uint) (map[uint]struct{}, error)
	GetSolvedProblemIDsByDetailIDs(ctx context.Context, lanqiaoUserDetailIDs []uint) (map[uint]map[uint]struct{}, error)
	BatchCreate(ctx context.Context, records []*entity.LanqiaoUserQuestion) error
	CountSolvedByDateRange(
		ctx context.Context,
		lanqiaoUserDetailID uint,
		start time.Time,
		end time.Time,
	) ([]*readmodel.DateSolvedCount, error)
	CountPassed(ctx context.Context, lanqiaoUserDetailID uint) (int64, error)
}
