package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

type LanqiaoUserDetailRepository interface {
	WithTx(tx any) LanqiaoUserDetailRepository
	GetByUserID(ctx context.Context, userID uint) (*entity.LanqiaoUserDetail, error)
	GetByCredentialHash(ctx context.Context, credentialHash string) (*entity.LanqiaoUserDetail, error)
	UpsertByUserID(ctx context.Context, detail *entity.LanqiaoUserDetail) (*entity.LanqiaoUserDetail, error)
	DeleteByUserID(ctx context.Context, userID uint) error
	GetAll(ctx context.Context) ([]*entity.LanqiaoUserDetail, error)
}
