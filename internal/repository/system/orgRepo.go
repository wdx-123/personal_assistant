package system

import (
	"context"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type orgRepository struct {
	db *gorm.DB
}

func NewOrgRepository(db *gorm.DB) interfaces.OrgRepository {
	return &orgRepository{db: db}
}

// GetByID 根据ID获取组织
func (r *orgRepository) GetByID(ctx context.Context, id uint) (*entity.Org, error) {
	var org entity.Org
	err := r.db.WithContext(ctx).First(&org, id).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// Create 创建组织
func (r *orgRepository) Create(ctx context.Context, org *entity.Org) error {
	return r.db.WithContext(ctx).Create(org).Error
}

// Update 更新组织
func (r *orgRepository) Update(ctx context.Context, org *entity.Org) error {
	return r.db.WithContext(ctx).Save(org).Error
}

// Delete 删除组织
func (r *orgRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.Org{}, id).Error
}

// GetAllOrgs 获取所有组织
func (r *orgRepository) GetAllOrgs(ctx context.Context) ([]*entity.Org, error) {
	var orgs []*entity.Org
	err := r.db.WithContext(ctx).Find(&orgs).Error
	return orgs, err
}

// GetOrgList 分页获取组织列表
func (r *orgRepository) GetOrgList(ctx context.Context, page, pageSize int) ([]*entity.Org, int64, error) {
	var orgs []*entity.Org
	var total int64

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).Model(&entity.Org{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&orgs).Error
	return orgs, total, err
}

// GetByCode 根据邀请码获取组织
func (r *orgRepository) GetByCode(ctx context.Context, code string) (*entity.Org, error) {
	var org entity.Org
	err := r.db.WithContext(ctx).Where("code = ?", code).First(&org).Error
	if err != nil {
		return nil, err
	}
	return &org, nil
}
