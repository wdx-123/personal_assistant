package system

import (
	"context"
	"errors"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type leetcodeUserDetailRepository struct {
	db *gorm.DB
}

func NewLeetcodeUserDetailRepository(db *gorm.DB) interfaces.LeetcodeUserDetailRepository {
	return &leetcodeUserDetailRepository{db: db}
}

func (r *leetcodeUserDetailRepository) GetByUserID(ctx context.Context, userID uint) (*entity.LeetcodeUserDetail, error) {
	var detail entity.LeetcodeUserDetail
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

func (r *leetcodeUserDetailRepository) UpsertByUserID(
	ctx context.Context,
	detail *entity.LeetcodeUserDetail,
) (*entity.LeetcodeUserDetail, error) {
	if detail == nil {
		return nil, errors.New("nil leetcode detail")
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

	existing.UserSlug = detail.UserSlug
	existing.RealName = detail.RealName
	existing.UserAvatar = detail.UserAvatar
	existing.EasyNumber = detail.EasyNumber
	existing.MediumNumber = detail.MediumNumber
	existing.HardNumber = detail.HardNumber

	if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
		return nil, err
	}
	return existing, nil
}

