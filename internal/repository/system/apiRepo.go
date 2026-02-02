package system

import (
	"context"

	"gorm.io/gorm"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

type apiRepository struct {
	db *gorm.DB
}

func NewAPIRepository(db *gorm.DB) interfaces.APIRepository {
	return &apiRepository{db: db}
}

// 基础CRUD操作

func (a *apiRepository) GetByID(ctx context.Context, id uint) (*entity.API, error) {
	var api entity.API
	err := a.db.WithContext(ctx).First(&api, id).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}
func (a *apiRepository) GetByPathAndMethod(ctx context.Context, path, method string) (*entity.API, error) {
	var api entity.API
	err := a.db.WithContext(ctx).Where("path = ? AND method = ?", path, method).First(&api).Error
	if err != nil {
		return nil, err
	}
	return &api, nil
}

func (a *apiRepository) Create(ctx context.Context, api *entity.API) error {
	return a.db.WithContext(ctx).Create(api).Error
}

func (a *apiRepository) Update(ctx context.Context, api *entity.API) error {
	return a.db.WithContext(ctx).Save(api).Error
}

func (a *apiRepository) Delete(ctx context.Context, id uint) error {
	return a.db.WithContext(ctx).Delete(&entity.API{}, id).Error
}

// 业务相关查询

// GetAPIList 获取API列表（分页，支持过滤）
func (a *apiRepository) GetAPIList(ctx context.Context, filter *request.ApiListFilter) ([]*entity.API, int64, error) {
	var apis []*entity.API
	var total int64

	query := a.db.WithContext(ctx).Model(&entity.API{})

	if filter != nil {
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.GroupID != nil && *filter.GroupID > 0 {
			query = query.Where("group_id = ?", *filter.GroupID)
		}
		if filter.Method != "" {
			query = query.Where("method = ?", filter.Method)
		}
		if filter.Keyword != "" {
			query = query.Where("path LIKE ? OR detail LIKE ?", "%"+filter.Keyword+"%", "%"+filter.Keyword+"%")
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := 1
	pageSize := 10
	if filter != nil {
		if filter.Page > 0 {
			page = filter.Page
		}
		if filter.PageSize > 0 {
			pageSize = filter.PageSize
		}
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id ASC").Find(&apis).Error
	return apis, total, err
}

func (a *apiRepository) GetAllAPIs(ctx context.Context) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).Find(&apis).Error
	return apis, err
}

func (a *apiRepository) GetAPIsByGroup(ctx context.Context, groupID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&apis).Error
	return apis, err
}

func (a *apiRepository) GetActiveAPIs(ctx context.Context) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).Where("status = ?", 1).Find(&apis).Error
	return apis, err
}
func (a *apiRepository) ExistsByPathAndMethod(ctx context.Context, path, method string) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).Model(&entity.API{}).Where("path = ? AND method = ?", path, method).Count(&count).Error
	return count > 0, err
}

// 权限查询

func (a *apiRepository) GetAPIsByUserID(ctx context.Context, userID, orgID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Joins("JOIN role_menus ON menu_apis.menu_id = role_menus.menu_id").
		Joins("JOIN user_org_roles ON role_menus.role_id = user_org_roles.role_id").
		Where("user_org_roles.user_id = ? AND user_org_roles.org_id = ? AND apis.deleted_at IS NULL", userID, orgID).
		Distinct().
		Find(&apis).Error
	return apis, err
}

func (a *apiRepository) GetAPIsByRoleID(ctx context.Context, roleID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Joins("JOIN role_menus ON menu_apis.menu_id = role_menus.menu_id").
		Where("role_menus.role_id = ? AND apis.deleted_at IS NULL", roleID).
		Find(&apis).Error
	return apis, err
}

func (a *apiRepository) CheckUserAPIPermission(ctx context.Context, userID, orgID uint, path, method string) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Joins("JOIN role_menus ON menu_apis.menu_id = role_menus.menu_id").
		Joins("JOIN user_org_roles ON role_menus.role_id = user_org_roles.role_id").
		Where("user_org_roles.user_id = ? AND user_org_roles.org_id = ? AND apis.path = ? AND apis.method = ? AND apis.deleted_at IS NULL", userID, orgID, path, method).
		Count(&count).Error
	return count > 0, err
}
