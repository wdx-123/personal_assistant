package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// MenuRepository 菜单仓储接口
type MenuRepository interface {
	// 基础CRUD操作
	GetByID(ctx context.Context, id uint) (*entity.Menu, error)
	GetByCode(ctx context.Context, code string) (*entity.Menu, error)
	Create(ctx context.Context, menu *entity.Menu) error
	Update(ctx context.Context, menu *entity.Menu) error
	Delete(ctx context.Context, id uint) error

	// 业务相关查询
	GetMenuList(ctx context.Context, page, pageSize int) ([]*entity.Menu, int64, error)
	GetAllMenus(ctx context.Context) ([]*entity.Menu, error)
	GetMenuTree(ctx context.Context, parentID uint) ([]*entity.Menu, error)
	GetActiveMenus(ctx context.Context) ([]*entity.Menu, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)

	// 菜单API关系管理
	AssignAPIToMenu(ctx context.Context, menuID, apiID uint) error
	RemoveAPIFromMenu(ctx context.Context, menuID, apiID uint) error
	GetMenuAPIs(ctx context.Context, menuID uint) ([]*entity.API, error)
	GetAPIMenus(ctx context.Context, apiID uint) ([]*entity.Menu, error)

	// 角色菜单关系查询
	GetMenusByRoleID(ctx context.Context, roleID uint) ([]*entity.Menu, error)
	GetMenusByUserID(ctx context.Context, userID uint) ([]*entity.Menu, error)

	// 权限同步相关
	GetAllMenuAPIRelations(ctx context.Context) ([]map[string]interface{}, error)
}
