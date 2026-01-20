package system

import (
	"context"
	"errors"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type luoguUserDetailRepository struct {
	db *gorm.DB
}

func NewLuoguUserDetailRepository(db *gorm.DB) interfaces.LuoguUserDetailRepository {
	return &luoguUserDetailRepository{db: db}
}

func (r *luoguUserDetailRepository) GetByUserID(ctx context.Context, userID uint) (*entity.LuoguUserDetail, error) {
	var detail entity.LuoguUserDetail
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&detail).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &detail, nil
}

func (r *luoguUserDetailRepository) GetByUserIDs(
	ctx context.Context,
	userIDs []uint,
) ([]*entity.LuoguUserDetail, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var details []*entity.LuoguUserDetail
	err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

func (r *luoguUserDetailRepository) ListByOrgID(
	ctx context.Context,
	orgID uint,
) ([]*entity.LuoguUserDetail, error) {
	var details []*entity.LuoguUserDetail
	err := r.db.WithContext(ctx).
		Table("luogu_user_details").
		Joins("JOIN users ON users.id = luogu_user_details.user_id").
		Where("users.current_org_id = ?", orgID).
		Select("luogu_user_details.*").
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

func (r *luoguUserDetailRepository) UpsertByUserID(
	ctx context.Context,
	detail *entity.LuoguUserDetail,
) (*entity.LuoguUserDetail, error) {
	if detail == nil {
		return nil, errors.New("nil luogu detail")
	}

	existing, err := r.GetByUserID(ctx, detail.UserID)
	if err != nil {
		return nil, err
	}

	if existing == nil {
		if err := r.db.WithContext(ctx).Create(detail).Error; err != nil {
			return nil, err
		}
		return detail, nil
	}

	existing.Identification = detail.Identification
	existing.RealName = detail.RealName
	existing.UserAvatar = detail.UserAvatar
	existing.PassedNumber = detail.PassedNumber

	if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
		return nil, err
	}
	return existing, nil
}

func (r *luoguUserDetailRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	// 由于配置了 OnDelete:CASCADE，删除 UserDetail 会自动删除关联的 UserQuestion
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&entity.LuoguUserDetail{}).Error
}

func (r *luoguUserDetailRepository) GetAll(ctx context.Context) ([]*entity.LuoguUserDetail, error) {
	var details []*entity.LuoguUserDetail
	err := r.db.WithContext(ctx).Find(&details).Error
	return details, err
}
