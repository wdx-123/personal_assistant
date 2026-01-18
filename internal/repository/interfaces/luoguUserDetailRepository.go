package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

type LuoguUserDetailRepository interface {
	GetByUserID(ctx context.Context, userID uint) (*entity.LuoguUserDetail, error)
	UpsertByUserID(ctx context.Context, detail *entity.LuoguUserDetail) (*entity.LuoguUserDetail, error)
}

