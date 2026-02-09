package system

import (
	"context"
	"strings"

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

// ExistsByName 检查组织名称是否已存在
func (r *orgRepository) ExistsByName(
	ctx context.Context,
	name string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Org{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

// CountMembersByOrgID 查询组织下的成员数（user_org_roles 表去重 user_id）
func (r *orgRepository) CountMembersByOrgID(
	ctx context.Context,
	orgID uint,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_org_roles").
		Where("org_id = ?", orgID).
		Distinct("user_id").
		Count(&count).Error
	return count, err
}

// GetOrgsByUserID 获取用户所属的组织列表（通过 user_org_roles 关联查询）
func (r *orgRepository) GetOrgsByUserID(
	ctx context.Context,
	userID uint,
) ([]*entity.Org, error) {
	var orgs []*entity.Org
	err := r.db.WithContext(ctx).
		Table("orgs").
		Joins("JOIN user_org_roles ON orgs.id = user_org_roles.org_id").
		Where("user_org_roles.user_id = ? AND orgs.deleted_at IS NULL", userID).
		Group("orgs.id").
		Find(&orgs).Error
	return orgs, err
}

// GetOrgListWithKeyword 支持关键词搜索的分页查询
func (r *orgRepository) GetOrgListWithKeyword(ctx context.Context, page, pageSize int, keyword string) ([]*entity.Org, int64, error) {
	var orgs []*entity.Org
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Org{})

	// 关键词搜索（按名称模糊匹配）
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&orgs).Error
	return orgs, total, err
}

// IsUserInOrg 检查用户是否属于指定组织
func (r *orgRepository) IsUserInOrg(
	ctx context.Context,
	userID, orgID uint,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_org_roles").
		Where("user_id = ? AND org_id = ?", userID, orgID).
		Count(&count).Error
	return count > 0, err
}
