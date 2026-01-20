package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// RoleRepository 角色仓储接口
type RoleRepository interface {
	// GetByID 根据ID获取角色
	GetByID(ctx context.Context, id uint) (*entity.Role, error)
	// GetByCode 根据代码获取角色
	GetByCode(ctx context.Context, code string) (*entity.Role, error)
	// Create 创建角色
	Create(ctx context.Context, role *entity.Role) error
	// Update 更新角色
	Update(ctx context.Context, role *entity.Role) error
	// Delete 删除角色
	Delete(ctx context.Context, id uint) error

	// GetRoleList 获取角色列表（分页）
	GetRoleList(ctx context.Context, page, pageSize int) ([]*entity.Role, int64, error)
	// GetAllRoles 获取所有角色
	GetAllRoles(ctx context.Context) ([]*entity.Role, error)
	// ExistsByCode 检查角色代码是否存在
	ExistsByCode(ctx context.Context, code string) (bool, error)
	// GetActiveRoles 获取所有启用的角色
	GetActiveRoles(ctx context.Context) ([]*entity.Role, error)

	// AssignMenuToRole 为角色分配菜单
	AssignMenuToRole(ctx context.Context, roleID, menuID uint) error
	// RemoveMenuFromRole 从角色移除菜单
	RemoveMenuFromRole(ctx context.Context, roleID, menuID uint) error
	// GetRoleMenus 获取角色关联的菜单列表
	GetRoleMenus(ctx context.Context, roleID uint) ([]*entity.Menu, error)
	// GetMenuRoles 获取菜单所属的角色列表
	GetMenuRoles(ctx context.Context, menuID uint) ([]*entity.Role, error)

	// AssignRoleToUser 为用户分配角色
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error
	// RemoveRoleFromUser 从用户移除角色
	RemoveRoleFromUser(ctx context.Context, userID, roleID uint) error
	// GetUsersByRole 获取具有特定角色的用户列表
	GetUsersByRole(ctx context.Context, roleID uint) ([]*entity.User, error)
	// GetUserRoles 获取用户的角色列表
	GetUserRoles(ctx context.Context, userID uint) ([]*entity.Role, error)

	// GetAllRoleMenuRelations 获取所有角色与菜单的关联关系（用于Casbin同步）
	GetAllRoleMenuRelations(ctx context.Context) ([]map[string]interface{}, error)
	// GetAllUserRoleRelations 获取所有用户与角色的关联关系（用于Casbin同步）
	GetAllUserRoleRelations(ctx context.Context) ([]map[string]interface{}, error)
}
