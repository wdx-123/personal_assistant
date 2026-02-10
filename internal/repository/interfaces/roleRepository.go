package interfaces

import (
	"context"
	"personal_assistant/internal/model/dto/request"
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
	// GetRoleListWithFilter 获取角色列表（分页，支持过滤）
	GetRoleListWithFilter(ctx context.Context, filter *request.RoleListFilter) ([]*entity.Role, int64, error)
	// GetAllRoles 获取所有角色
	GetAllRoles(ctx context.Context) ([]*entity.Role, error)
	// ExistsByCode 检查角色代码是否存在
	ExistsByCode(ctx context.Context, code string) (bool, error)
	// ExistsByCodeExcludeID 检查角色代码是否存在（排除指定ID）
	ExistsByCodeExcludeID(ctx context.Context, code string, excludeID uint) (bool, error)
	// GetActiveRoles 获取所有启用的角色
	GetActiveRoles(ctx context.Context) ([]*entity.Role, error)
	// IsRoleInUse 检查角色是否正在被使用（有用户关联）
	IsRoleInUse(ctx context.Context, roleID uint) (bool, error)

	// AssignMenuToRole 为角色分配菜单
	AssignMenuToRole(ctx context.Context, roleID, menuID uint) error
	// RemoveMenuFromRole 从角色移除菜单
	RemoveMenuFromRole(ctx context.Context, roleID, menuID uint) error
	// GetRoleMenus 获取角色关联的菜单列表
	GetRoleMenus(ctx context.Context, roleID uint) ([]*entity.Menu, error)
	// GetRoleMenuIDs 获取角色关联的菜单ID列表
	GetRoleMenuIDs(ctx context.Context, roleID uint) ([]uint, error)
	// ReplaceRoleMenus 全量替换角色的菜单权限（事务）
	ReplaceRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error
	// GetMenuRoles 获取菜单所属的角色列表
	GetMenuRoles(ctx context.Context, menuID uint) ([]*entity.Role, error)
	// ClearRoleMenus 清空角色的所有菜单关联
	ClearRoleMenus(ctx context.Context, roleID uint) error

	// AssignRoleToUserInOrg 为用户在组织中分配角色
	AssignRoleToUserInOrg(ctx context.Context, userID, orgID, roleID uint) error
	// RemoveRoleFromUserInOrg 从用户在组织中移除角色
	RemoveRoleFromUserInOrg(ctx context.Context, userID, orgID, roleID uint) error
	// GetUserRolesByOrg 获取用户在组织中的角色列表
	GetUserRolesByOrg(ctx context.Context, userID, orgID uint) ([]*entity.Role, error)
	// GetUserGlobalRoles 获取用户的全局角色（org_id = 0，如超级管理员等不绑定具体组织的角色）
	GetUserGlobalRoles(ctx context.Context, userID uint) ([]*entity.Role, error)
	// ClearRoleUserRelations 清空角色的所有用户关联
	ClearRoleUserRelations(ctx context.Context, roleID uint) error

	// GetAllRoleMenuRelations 获取所有角色与菜单的关联关系（用于Casbin同步）
	GetAllRoleMenuRelations(ctx context.Context) ([]map[string]interface{}, error)
	// GetAllUserOrgRoleRelations 获取所有用户在组织中的角色关联关系（用于Casbin同步）
	GetAllUserOrgRoleRelations(ctx context.Context) ([]map[string]interface{}, error)

	// WithTx 启用事务（返回支持事务的新实例）
	WithTx(tx any) RoleRepository
}
