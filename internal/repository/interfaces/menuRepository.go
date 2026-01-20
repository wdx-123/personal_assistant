package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// MenuRepository 菜单仓储接口
type MenuRepository interface {
	// GetByID 根据ID获取菜单
	GetByID(ctx context.Context, id uint) (*entity.Menu, error)
	// GetByCode 根据代码获取菜单
	GetByCode(ctx context.Context, code string) (*entity.Menu, error)
	// Create 创建菜单
	Create(ctx context.Context, menu *entity.Menu) error
	// Update 更新菜单
	Update(ctx context.Context, menu *entity.Menu) error
	// Delete 删除菜单
	Delete(ctx context.Context, id uint) error

	// GetMenuList 获取菜单列表（分页）
	GetMenuList(ctx context.Context, page, pageSize int) ([]*entity.Menu, int64, error)
	// GetAllMenus 获取所有菜单
	GetAllMenus(ctx context.Context) ([]*entity.Menu, error)
	// GetMenuTree 获取菜单树
	GetMenuTree(ctx context.Context, parentID uint) ([]*entity.Menu, error)
	// GetActiveMenus 获取所有启用菜单
	GetActiveMenus(ctx context.Context) ([]*entity.Menu, error)
	// ExistsByCode 检查代码是否存在
	ExistsByCode(ctx context.Context, code string) (bool, error)

	// AssignAPIToMenu 为菜单分配API
	AssignAPIToMenu(ctx context.Context, menuID, apiID uint) error
	// RemoveAPIFromMenu 从菜单移除API
	RemoveAPIFromMenu(ctx context.Context, menuID, apiID uint) error
	// GetMenuAPIs 获取菜单关联的API列表
	GetMenuAPIs(ctx context.Context, menuID uint) ([]*entity.API, error)
	// GetAPIMenus 获取API所属的菜单列表
	GetAPIMenus(ctx context.Context, apiID uint) ([]*entity.Menu, error)

	// GetMenusByRoleID 获取角色的菜单列表
	GetMenusByRoleID(ctx context.Context, roleID uint) ([]*entity.Menu, error)
	// GetMenusByUserID 获取用户的菜单列表
	GetMenusByUserID(ctx context.Context, userID uint) ([]*entity.Menu, error)

	// GetAllMenuAPIRelations 获取所有菜单与API的关联关系（用于Casbin同步）
	GetAllMenuAPIRelations(ctx context.Context) ([]map[string]interface{}, error)
}
