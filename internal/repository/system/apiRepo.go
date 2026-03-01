package system

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
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

// CreateWithMenu 创建API并绑定菜单（事务）
func (a *apiRepository) CreateWithMenu(ctx context.Context, api *entity.API, menuID uint) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(api).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO menu_apis (menu_id, api_id) VALUES (?, ?)", menuID, api.ID).Error
	})
}

func (a *apiRepository) Update(ctx context.Context, api *entity.API) error {
	return a.db.WithContext(ctx).Save(api).Error
}

// UpdateWithMenu 更新API并按三态更新菜单绑定（事务）
func (a *apiRepository) UpdateWithMenu(
	ctx context.Context,
	api *entity.API,
	menuID *uint,
) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先更新API基本信息
		if err := tx.Save(api).Error; err != nil {
			return err
		}

		// nil 表示不改菜单绑定
		if menuID == nil {
			return nil
		}

		// 先清空当前API在 menu_apis 上的历史绑定
		if err := tx.Exec("DELETE FROM menu_apis WHERE api_id = ?", api.ID).Error; err != nil {
			return err
		}

		// 0 表示清空绑定，不插入新关系
		if *menuID == 0 {
			return nil
		}

		// >0 表示迁移并绑定到指定菜单
		return tx.Exec("INSERT INTO menu_apis (menu_id, api_id) VALUES (?, ?)", *menuID, api.ID).Error
	})
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

// GetMenuByAPIID 获取API归属菜单
func (a *apiRepository) GetMenuByAPIID(ctx context.Context, apiID uint) (*entity.Menu, error) {
	var menu entity.Menu
	err := a.db.WithContext(ctx).
		Table("menus").
		Select("menus.*").
		Joins("JOIN menu_apis ON menus.id = menu_apis.menu_id").
		Where("menu_apis.api_id = ? AND menus.deleted_at IS NULL", apiID).
		Limit(1).
		Find(&menu).Error
	if err != nil {
		return nil, err
	}
	if menu.ID == 0 {
		return nil, nil
	}
	return &menu, nil
}

// GetMenusByAPIIDs 批量获取API归属菜单（key: api_id）
func (a *apiRepository) GetMenusByAPIIDs(
	ctx context.Context,
	apiIDs []uint,
) (map[uint]*entity.Menu, error) {
	result := make(map[uint]*entity.Menu)
	if len(apiIDs) == 0 {
		return result, nil
	}

	type row struct {
		APIID    uint   `gorm:"column:api_id"`
		MenuID   uint   `gorm:"column:menu_id"`
		MenuName string `gorm:"column:menu_name"`
	}

	var rows []row
	err := a.db.WithContext(ctx).
		Table("menu_apis").
		Select("menu_apis.api_id, menus.id AS menu_id, menus.name AS menu_name").
		Joins("JOIN menus ON menu_apis.menu_id = menus.id").
		Where("menu_apis.api_id IN ? AND menus.deleted_at IS NULL", apiIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		menu := &entity.Menu{
			Name: r.MenuName,
		}
		menu.ID = r.MenuID
		result[r.APIID] = menu
	}
	return result, nil
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
