package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// RoleRepository 角色仓储接口
type RoleRepository interface {
	// 基础CRUD操作
	GetByID(ctx context.Context, id uint) (*entity.Role, error)
	GetByCode(ctx context.Context, code string) (*entity.Role, error)
	Create(ctx context.Context, role *entity.Role) error
	Update(ctx context.Context, role *entity.Role) error
	Delete(ctx context.Context, id uint) error

	// 业务相关查询
	GetRoleList(ctx context.Context, page, pageSize int) ([]*entity.Role, int64, error)
	GetAllRoles(ctx context.Context) ([]*entity.Role, error)
	ExistsByCode(ctx context.Context, code string) (bool, error)
	GetActiveRoles(ctx context.Context) ([]*entity.Role, error)

	// 角色菜单关系管理
	AssignMenuToRole(ctx context.Context, roleID, menuID uint) error
	RemoveMenuFromRole(ctx context.Context, roleID, menuID uint) error
	GetRoleMenus(ctx context.Context, roleID uint) ([]*entity.Menu, error)
	GetMenuRoles(ctx context.Context, menuID uint) ([]*entity.Role, error)

	// 用户角色关系管理
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error
	RemoveRoleFromUser(ctx context.Context, userID, roleID uint) error
	GetUsersByRole(ctx context.Context, roleID uint) ([]*entity.User, error)
	GetUserRoles(ctx context.Context, userID uint) ([]*entity.Role, error)

	// 权限同步相关
	GetAllRoleMenuRelations(ctx context.Context) ([]map[string]interface{}, error)
	GetAllUserRoleRelations(ctx context.Context) ([]map[string]interface{}, error)
}
