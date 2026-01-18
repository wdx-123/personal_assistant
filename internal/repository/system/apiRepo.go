package system

import (
	"context"
	"gorm.io/gorm"
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

func (a *apiRepository) GetAPIList(ctx context.Context, page, pageSize int) ([]*entity.API, int64, error) {
	var apis []*entity.API
	var total int64

	offset := (page - 1) * pageSize
	err := a.db.WithContext(ctx).Model(&entity.API{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = a.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&apis).Error
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

func (a *apiRepository) GetAPIsByUserID(ctx context.Context, userID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := a.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Joins("JOIN role_menus ON menu_apis.menu_id = role_menus.menu_id").
		Joins("JOIN user_roles ON role_menus.role_id = user_roles.role_id").
		Where("user_roles.user_id = ? AND apis.deleted_at IS NULL", userID).
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

func (a *apiRepository) CheckUserAPIPermission(ctx context.Context, userID uint, path, method string) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Joins("JOIN role_menus ON menu_apis.menu_id = role_menus.menu_id").
		Joins("JOIN user_roles ON role_menus.role_id = user_roles.role_id").
		Where("user_roles.user_id = ? AND apis.path = ? AND apis.method = ? AND apis.deleted_at IS NULL", userID, path, method).
		Count(&count).Error
	return count > 0, err
}
