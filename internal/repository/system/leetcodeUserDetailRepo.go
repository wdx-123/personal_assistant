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

func (r *leetcodeUserDetailRepository) GetByUserIDs(
	ctx context.Context,
	userIDs []uint,
) ([]*entity.LeetcodeUserDetail, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var details []*entity.LeetcodeUserDetail
	err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

func (r *leetcodeUserDetailRepository) ListByOrgID(
	ctx context.Context,
	orgID uint,
) ([]*entity.LeetcodeUserDetail, error) {
	var details []*entity.LeetcodeUserDetail
	err := r.db.WithContext(ctx).
		Table("leetcode_user_details").
		Joins("JOIN users ON users.id = leetcode_user_details.user_id").
		Where("users.current_org_id = ?", orgID).
		Select("leetcode_user_details.*").
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

func (r *leetcodeUserDetailRepository) GetAll(
	ctx context.Context,
) ([]*entity.LeetcodeUserDetail, error) { // 获取全部力扣用户详情
	var details []*entity.LeetcodeUserDetail          // 声明结果集
	err := r.db.WithContext(ctx).Find(&details).Error // 执行全量查询
	return details, err                               // 返回结果与错误
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
	existing.TotalNumber = detail.TotalNumber

	if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
		return nil, err
	}
	return existing, nil
}

func (r *leetcodeUserDetailRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	// 由于配置了 OnDelete:CASCADE，删除 UserDetail 会自动删除关联的 UserQuestion
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&entity.LeetcodeUserDetail{}).Error
}
