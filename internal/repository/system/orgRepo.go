package system

import (
	"context"
	"errors"
	"strings"

	"personal_assistant/internal/model/consts"
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

// WithTx 启用事务
func (r *orgRepository) WithTx(tx any) interfaces.OrgRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &orgRepository{db: transaction}
	}
	return r
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &org, nil
}

func (r *orgRepository) GetByBuiltinKey(ctx context.Context, key string) (*entity.Org, error) {
	var org entity.Org
	err := r.db.WithContext(ctx).Where("builtin_key = ?", key).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
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

// RemoveAllMembers 删除组织下的所有成员关联
func (r *orgRepository) RemoveAllMembers(ctx context.Context, orgID uint) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM user_org_roles WHERE org_id = ?", orgID).
		Error
}

// CountMembersByOrgID 查询组织下的活跃成员数
func (r *orgRepository) CountMembersByOrgID(
	ctx context.Context,
	orgID uint,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("org_id = ? AND member_status = ?", orgID, consts.OrgMemberStatusActive).
		Count(&count).Error
	return count, err
}

// GetOrgsByUserID 获取用户所属的活跃组织列表
func (r *orgRepository) GetOrgsByUserID(
	ctx context.Context,
	userID uint,
) ([]*entity.Org, error) {
	var orgs []*entity.Org
	err := r.db.WithContext(ctx).
		Table("orgs").
		Joins("JOIN org_members ON orgs.id = org_members.org_id").
		Where(
			"org_members.user_id = ? AND org_members.member_status = ? AND orgs.deleted_at IS NULL",
			userID,
			consts.OrgMemberStatusActive,
		).
		Group("orgs.id").
		Find(&orgs).Error
	return orgs, err
}

// GetOrgListWithKeyword 支持关键词搜索；page <= 0 时返回全部匹配数据
func (r *orgRepository) GetOrgListWithKeyword(ctx context.Context, page, pageSize int, keyword string) ([]*entity.Org, int64, error) {
	query := r.applyOrgKeywordFilter(
		r.db.WithContext(ctx).Model(&entity.Org{}),
		keyword,
	)
	return r.listOrgsWithQuery(query, page, pageSize)
}

// IsUserInOrg 检查用户是否属于指定组织
func (r *orgRepository) IsUserInOrg(
	ctx context.Context,
	userID, orgID uint,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("user_id = ? AND org_id = ? AND member_status = ?", userID, orgID, consts.OrgMemberStatusActive).
		Count(&count).Error
	return count > 0, err
}

// GetVisibleOrgListByUserIDWithKeyword 获取用户可见的活跃组织列表；page <= 0 时返回全部匹配数据
func (r *orgRepository) GetVisibleOrgListByUserIDWithKeyword(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]*entity.Org, int64, error) {
	query := r.applyOrgKeywordFilter(
		r.db.WithContext(ctx).
			Model(&entity.Org{}).
			Joins("JOIN org_members ON org_members.org_id = orgs.id").
			Where("org_members.user_id = ? AND org_members.member_status = ?", userID, consts.OrgMemberStatusActive),
		keyword,
	)
	return r.listOrgsWithQuery(query, page, pageSize)
}

func (r *orgRepository) applyOrgKeywordFilter(query *gorm.DB, keyword string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("orgs.name LIKE ?", "%"+keyword+"%")
	}
	return query
}

func (r *orgRepository) listOrgsWithQuery(
	query *gorm.DB,
	page, pageSize int,
) ([]*entity.Org, int64, error) {
	var (
		orgs  []*entity.Org
		total int64
	)

	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Order("orgs.id DESC").Offset(offset).Limit(pageSize)
	}

	if err := query.Find(&orgs).Error; err != nil {
		return nil, 0, err
	}
	return orgs, total, nil
}
