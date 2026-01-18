package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

type LeetcodeUserDetailRepository interface {
	GetByUserID(ctx context.Context, userID uint) (*entity.LeetcodeUserDetail, error)
	UpsertByUserID(ctx context.Context, detail *entity.LeetcodeUserDetail) (*entity.LeetcodeUserDetail, error)
}

