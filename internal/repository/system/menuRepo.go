package system

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
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

// GetMenuList 获取菜单列表（分页，支持过滤）
func (m *menuRepository) GetMenuList(ctx context.Context, filter *request.MenuListFilter) ([]*entity.Menu, int64, error) {
	var menus []*entity.Menu
	var total int64

	query := m.db.WithContext(ctx).Model(&entity.Menu{})

	if filter != nil {
		if filter.Type != nil {
			query = query.Where("type = ?", *filter.Type)
		}
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.ParentID != nil {
			query = query.Where("parent_id = ?", *filter.ParentID)
		}
		if filter.Keyword != "" {
			query = query.Where("name LIKE ? OR code LIKE ?", "%"+filter.Keyword+"%", "%"+filter.Keyword+"%")
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
	err := query.Order("sort ASC, id ASC").Offset(offset).Limit(pageSize).Find(&menus).Error
	return menus, total, err
}

// GetAllMenus 获取所有菜单
func (m *menuRepository) GetAllMenus(
	ctx context.Context,
) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).Find(&menus).Error
	return menus, err
}

// GetAllMenusWithAPIs 获取所有菜单（预加载关联API）
func (m *menuRepository) GetAllMenusWithAPIs(ctx context.Context) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Preload("APIs").
		Order("sort ASC, id ASC").
		Find(&menus).Error
	return menus, err
}

// GetMenuTree 获取菜单树
func (m *menuRepository) GetMenuTree(
	ctx context.Context,
	parentID uint,
) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Where("parent_id = ?", parentID).
		Order("sort ASC").Find(&menus).Error
	return menus, err
}

// GetMenuChildren 获取指定菜单的直接子菜单
func (m *menuRepository) GetMenuChildren(
	ctx context.Context,
	parentID uint,
) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Where("parent_id = ?", parentID).
		Order("sort ASC").Find(&menus).Error
	return menus, err
}

// HasChildren 检查菜单是否有子菜单
func (m *menuRepository) HasChildren(ctx context.Context, menuID uint) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Model(&entity.Menu{}).Where("parent_id = ?", menuID).Count(&count).Error
	return count > 0, err
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

// RemoveAPIFromMenu 从菜单移除API
func (m *menuRepository) RemoveAPIFromMenu(ctx context.Context, menuID, apiID uint) error {
	return m.db.WithContext(ctx).
		Exec("DELETE FROM menu_apis WHERE menu_id = ? AND api_id = ?", menuID, apiID).Error
}

// RemoveAPIFromAllMenus 从所有菜单中移除指定API（删除API前解绑）
func (m *menuRepository) RemoveAPIFromAllMenus(ctx context.Context, apiID uint) error {
	return m.db.WithContext(ctx).
		Exec("DELETE FROM menu_apis WHERE api_id = ?", apiID).Error
}

// ClearMenuAPIs 清空菜单的所有API绑定（bind_api 覆盖前调用）
func (m *menuRepository) ClearMenuAPIs(ctx context.Context, menuID uint) error {
	return m.db.WithContext(ctx).
		Exec("DELETE FROM menu_apis WHERE menu_id = ?", menuID).Error
}

// ReplaceMenuAPIsSingleBinding 覆盖菜单绑定（单菜单语义）
// 先清空当前菜单旧绑定，再逐个将 apiIDs 迁移并绑定到当前菜单。
func (m *menuRepository) ReplaceMenuAPIsSingleBinding(
	ctx context.Context,
	menuID uint,
	apiIDs []uint,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM menu_apis WHERE menu_id = ?", menuID).Error; err != nil {
			return err
		}
		if len(apiIDs) == 0 {
			return nil
		}

		seen := make(map[uint]struct{}, len(apiIDs))
		for _, apiID := range apiIDs {
			if apiID == 0 {
				continue
			}
			if _, ok := seen[apiID]; ok {
				continue
			}
			seen[apiID] = struct{}{}

			// 单菜单语义：一个API仅能属于一个菜单，先按 api_id 清理历史绑定。
			if err := tx.Exec("DELETE FROM menu_apis WHERE api_id = ?", apiID).Error; err != nil {
				return err
			}
			if err := tx.Exec("INSERT INTO menu_apis (menu_id, api_id) VALUES (?, ?)", menuID, apiID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetMenuAPIs 获取菜单关联的API列表
func (m *menuRepository) GetMenuAPIs(ctx context.Context, menuID uint) ([]*entity.API, error) {
	var apis []*entity.API
	err := m.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Where("menu_apis.menu_id = ? AND apis.deleted_at IS NULL", menuID).
		Find(&apis).Error
	return apis, err
}

// GetAPIMenus 获取API所属的菜单列表
func (m *menuRepository) GetAPIMenus(ctx context.Context, apiID uint) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := m.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN menu_apis ON menus.id = menu_apis.menu_id").
		Where("menu_apis.api_id = ? AND menus.deleted_at IS NULL", apiID).
		Find(&menus).Error
	return menus, err
}

// GetAPIIDsByMenuIDs 按菜单ID集合查询绑定的API ID集合（去重）
func (m *menuRepository) GetAPIIDsByMenuIDs(
	ctx context.Context,
	menuIDs []uint,
) ([]uint, error) {
	apiIDs := make([]uint, 0)
	if len(menuIDs) == 0 {
		return apiIDs, nil
	}
	err := m.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN menu_apis ON apis.id = menu_apis.api_id").
		Where("menu_apis.menu_id IN ? AND apis.deleted_at IS NULL", menuIDs).
		Distinct().
		Pluck("apis.id", &apiIDs).Error
	return apiIDs, err
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

// GetMenusByUserID 获取用户的菜单列表
func (m *menuRepository) GetMenusByUserID(
	ctx context.Context,
	userID uint,
	orgID uint,
) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	db := m.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id")
	db = db.Joins("JOIN user_org_roles ON role_menus.role_id = user_org_roles.role_id").
		Where("user_org_roles.user_id = ? AND user_org_roles.org_id = ? AND menus.deleted_at IS NULL", userID, orgID)
	err := db.Distinct().Find(&menus).Error
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
