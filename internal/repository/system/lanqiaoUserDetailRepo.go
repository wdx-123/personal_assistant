package system

import (
	"context"
	"errors"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type lanqiaoUserDetailRepository struct {
	db *gorm.DB
}

func NewLanqiaoUserDetailRepository(db *gorm.DB) interfaces.LanqiaoUserDetailRepository {
	return &lanqiaoUserDetailRepository{db: db}
}

func (r *lanqiaoUserDetailRepository) WithTx(tx any) interfaces.LanqiaoUserDetailRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &lanqiaoUserDetailRepository{db: transaction}
	}
	return r
}

func (r *lanqiaoUserDetailRepository) GetByUserID(
	ctx context.Context,
	userID uint,
) (*entity.LanqiaoUserDetail, error) {
	var detail entity.LanqiaoUserDetail
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&detail).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &detail, nil
}

func (r *lanqiaoUserDetailRepository) GetByUserIDs(
	ctx context.Context,
	userIDs []uint,
) ([]*entity.LanqiaoUserDetail, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var details []*entity.LanqiaoUserDetail
	err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&details).Error
	if err != nil {
		return nil, err
	}
	return details, nil
}

func (r *lanqiaoUserDetailRepository) GetByCredentialHash(
	ctx context.Context,
	credentialHash string,
) (*entity.LanqiaoUserDetail, error) {
	var detail entity.LanqiaoUserDetail
	err := r.db.WithContext(ctx).Where("credential_hash = ?", credentialHash).First(&detail).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &detail, nil
}

func (r *lanqiaoUserDetailRepository) UpsertByUserID(
	ctx context.Context,
	detail *entity.LanqiaoUserDetail,
) (*entity.LanqiaoUserDetail, error) {
	if detail == nil {
		return nil, errors.New("nil lanqiao detail")
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

	existing.CredentialHash = detail.CredentialHash
	existing.PhoneCipher = detail.PhoneCipher
	existing.PasswordCipher = detail.PasswordCipher
	existing.MaskedPhone = detail.MaskedPhone
	existing.SubmitSuccessCount = detail.SubmitSuccessCount
	existing.SubmitFailedCount = detail.SubmitFailedCount
	existing.SubmitStatsUpdatedAt = detail.SubmitStatsUpdatedAt
	existing.LastBindAt = detail.LastBindAt
	existing.LastSyncAt = detail.LastSyncAt

	if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
		return nil, err
	}
	return existing, nil
}

func (r *lanqiaoUserDetailRepository) DeleteByUserID(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&entity.LanqiaoUserDetail{}).Error
}

func (r *lanqiaoUserDetailRepository) GetAll(ctx context.Context) ([]*entity.LanqiaoUserDetail, error) {
	var details []*entity.LanqiaoUserDetail
	err := r.db.WithContext(ctx).Find(&details).Error
	return details, err
}
