package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LanqiaoQuestionBankRepository interface {
	WithTx(tx any) LanqiaoQuestionBankRepository
	GetByProblemID(ctx context.Context, problemID int) (*entity.LanqiaoQuestionBank, error)
	GetCachedID(ctx context.Context, problemID int) (uint, bool, error)
	CacheID(ctx context.Context, problemID int, id uint) error
	EnsureQuestionID(ctx context.Context, question *entity.LanqiaoQuestionBank) (uint, error)
}
