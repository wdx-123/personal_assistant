package system

import (
	"context"
	"gorm.io/gorm"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

type menuRepository struct {
	db *gorm.DB
}

func NewMenuRepository(db *gorm.DB) interfaces.MenuRepository {
	return &menuRepository{db: db}
}

// 基础CRUD操作
func (m *menuRepository) GetByID(ctx context.Context, id uint) (*entity.Menu, error) {
	var menu entity.Menu
	err := m.db.WithContext(ctx).Preload("APIs").First(&menu, id).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (m *menuRepository) GetByCode(ctx context.Context, code string) (*entity.Menu, error) {
	var menu entity.Menu
	err := m.db.WithContext(ctx).Where("code = ?", code).Preload("APIs").First(&menu).Error
	if err != nil {
		return nil, err
	}
	return &menu, nil
}

func (m *menuRepository) Create(ctx context.Context, menu *entity.Menu) error {
	return m.db.WithContext(ctx).Create(menu).Error
}

func (m *menuRepository) Update(ctx context.Context, menu *entity.Menu) error {
	return m.db.WithContext(ctx).Save(menu).Error
}

func (m *menuRepository) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&entity.Menu{}, id).Error
}

// 业务相关查询
func (m *menuRepository) GetMenuList(ctx context.Context, page, pageSize int) ([]*entity.Menu, int64, error) {
	var menus []*entity.Menu
	var total int64

	offset := (page - 1) * pageSize
	err := m.db.WithContext(ctx).Model(&entity.Menu{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = m.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&menus).Error
	return menus, total, err
}

func (m *menuRepository) GetAllMenus(ctx context.Context) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).Find(&menus).Error
	return menus, err
}

func (m *menuRepository) GetMenuTree(ctx context.Context, parentID uint) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("sort ASC").Find(&menus).Error
	return menus, err
}

func (m *menuRepository) GetActiveMenus(ctx context.Context) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).Where("status = ?", 1).Find(&menus).Error
	return menus, err
}

func (m *menuRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&entity.Menu{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

// 菜单API关系管理

func (m *menuRepository) AssignAPIToMenu(ctx context.Context, menuID, apiID uint) error {
	return m.db.WithContext(ctx).Exec("INSERT IGNORE INTO menu_apis (menu_id, api_id) VALUES (?, ?)", menuID, apiID).Error
}

func (m *menuRepository) RemoveAPIFromMenu(ctx context.Context, menuID, apiID uint) error {
	return m.db.WithContext(ctx).Exec("DELETE FROM menu_apis WHERE menu_id = ? AND api_id = ?", menuID, apiID).Error
}

func (m *menuRepository) GetMenuAPIs(ctx context.Context, menuID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := m.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Where("menu_apis.menu_id = ? AND apis.deleted_at IS NULL", menuID).
		Find(&apis).Error
	return apis, err
}

func (m *menuRepository) GetAPIMenus(ctx context.Context, apiID uint) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN menu_apis ON menus.id = menu_apis.menu_id").
		Where("menu_apis.api_id = ? AND menus.deleted_at IS NULL", apiID).
		Find(&menus).Error
	return menus, err
}

// 角色菜单关系查询

func (m *menuRepository) GetMenusByRoleID(ctx context.Context, roleID uint) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Where("role_menus.role_id = ? AND menus.deleted_at IS NULL", roleID).
		Find(&menus).Error
	return menus, err
}

func (m *menuRepository) GetMenusByUserID(ctx context.Context, userID uint) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Joins("JOIN user_roles ON role_menus.role_id = user_roles.role_id").
		Where("user_roles.user_id = ? AND menus.deleted_at IS NULL", userID).
		Distinct().
		Find(&menus).Error
	return menus, err
}

// 权限同步相关

func (m *menuRepository) GetAllMenuAPIRelations(ctx context.Context) ([]map[string]interface{}, error) {
	var relations []map[string]interface{}
	err := m.db.WithContext(ctx).
		Table("menu_apis").
		Select("menus.code as menu_code, apis.path, apis.method").
		Joins("JOIN menus ON menu_apis.menu_id = menus.id").
		Joins("JOIN apis ON menu_apis.api_id = apis.id").
		Where("menus.deleted_at IS NULL AND apis.deleted_at IS NULL").
		Find(&relations).Error
	return relations, err
}
