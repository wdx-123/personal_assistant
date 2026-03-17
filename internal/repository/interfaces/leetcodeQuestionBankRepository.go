package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LeetcodeQuestionBankRepository interface {
	WithTx(tx any) LeetcodeQuestionBankRepository
	GetAllTitleSlugMap(ctx context.Context) (map[string]uint, error)
	BatchCreate(ctx context.Context, questions []*entity.LeetcodeQuestionBank) error
	Create(ctx context.Context, question *entity.LeetcodeQuestionBank) error
	Update(ctx context.Context, question *entity.LeetcodeQuestionBank) error
	GetByID(ctx context.Context, id uint) (*entity.LeetcodeQuestionBank, error)
	GetByTitleSlug(ctx context.Context, titleSlug string) (*entity.LeetcodeQuestionBank, error)
	ListByExactTitle(ctx context.Context, title string) ([]*entity.LeetcodeQuestionBank, error)
	SearchByTitle(ctx context.Context, keyword string, limit int) ([]*entity.LeetcodeQuestionBank, error)
	GetCachedID(ctx context.Context, titleSlug string) (uint, bool, error)
	CacheID(ctx context.Context, titleSlug string, id uint) error
	EnsureQuestionID(ctx context.Context, question *entity.LeetcodeQuestionBank) (uint, error)
	ListTitleSlugIDAfterID(ctx context.Context, lastID uint, limit int) ([]*entity.LeetcodeQuestionBank, error)
}
