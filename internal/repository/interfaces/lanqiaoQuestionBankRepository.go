package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LanqiaoQuestionBankRepository interface {
	WithTx(tx any) LanqiaoQuestionBankRepository
	Create(ctx context.Context, question *entity.LanqiaoQuestionBank) error
	Update(ctx context.Context, question *entity.LanqiaoQuestionBank) error
	GetByID(ctx context.Context, id uint) (*entity.LanqiaoQuestionBank, error)
	GetByProblemID(ctx context.Context, problemID int) (*entity.LanqiaoQuestionBank, error)
	ListByExactTitle(ctx context.Context, title string) ([]*entity.LanqiaoQuestionBank, error)
	SearchByTitle(ctx context.Context, keyword string, limit int) ([]*entity.LanqiaoQuestionBank, error)
	GetCachedID(ctx context.Context, problemID int) (uint, bool, error)
	CacheID(ctx context.Context, problemID int, id uint) error
	EnsureQuestionID(ctx context.Context, question *entity.LanqiaoQuestionBank) (uint, error)
}
