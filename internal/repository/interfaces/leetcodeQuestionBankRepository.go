package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LeetcodeQuestionBankRepository interface {
	GetAllTitleSlugMap(ctx context.Context) (map[string]uint, error)
	BatchCreate(ctx context.Context, questions []*entity.LeetcodeQuestionBank) error
	GetByTitleSlug(ctx context.Context, titleSlug string) (*entity.LeetcodeQuestionBank, error)
	GetCachedID(ctx context.Context, titleSlug string) (uint, bool, error)
	CacheID(ctx context.Context, titleSlug string, id uint) error
	EnsureQuestionID(ctx context.Context, question *entity.LeetcodeQuestionBank) (uint, error)
	ListTitleSlugIDAfterID(ctx context.Context, lastID uint, limit int) ([]*entity.LeetcodeQuestionBank, error)
}
