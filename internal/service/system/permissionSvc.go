package system

import (
	"context"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/casbin"

	"go.uber.org/zap"
)

// PermissionService 权限管理服务
type PermissionService struct {
	repositoryGroup *repository.Group
	casbinSvc       *casbin.Service
}

// NewPermissionService 创建服务权限实例
func NewPermissionService(repositoryGroup *repository.Group) *PermissionService {
	return &PermissionService{
		repositoryGroup: repositoryGroup,
		casbinSvc:       casbin.NewCasbinService(),
	}
}

// =============================================================================
// 核心同步方法：将数据库数据同步到Casbin策略
// =============================================================================

// SyncAllPermissionsToCasbin 将数据库内的所有信息同步到Casbin
func (p *PermissionService) SyncAllPermissionsToCasbin(ctx context.Context) error {
	// 清空现有权限
	if err := p.ClearAllPermission(ctx); err != nil {
		return fmt.Errorf("清空权限失败: %w", err)
	}
	// 同步用户角色关系
	if err := p.SyncUserRolesToCasbin(ctx); err != nil {
		return fmt.Errorf("同步用户角色失败：%w", err)
	}
	// 同步角色菜单权限
	if err := p.SyncRoleMenusToCasbin(ctx); err != nil {
		return fmt.Errorf("同步角色菜单失败：%w", err)
	}
	// 同步菜单api权限
	if err := p.SyncMenuAPIsToCasbin(ctx); err != nil {
		return fmt.Errorf("同步菜单API失败：%w", err)
	}

	global.Log.Info("权限同步完成")
	return nil
}

/*
	角色 → 菜单：控制用户是否能访问前端页面（如 user_manage 页面）。
	菜单 → API：控制菜单关联的 API 是否可被调用（如 /api/users:GET）。
	read：用于前端页面权限，表示“查看”或“进入”某个页面。
	access：用于后端 API 权限，表示“执行”某个 API 操作。
*/

func (p *PermissionService) SyncUserRolesToCasbin(ctx context.Context) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()
	// 获取所有角色关系
	relations, err := roleRepo.GetAllUserOrgRoleRelations(ctx)
	if err != nil {
		return fmt.Errorf("获取用户角色关系失败:%w", err)
	}
	// 添加到Casbin
	for _, relation := range relations {
		userID := fmt.Sprintf("%v", relation["user_id"])
		roleCode := fmt.Sprintf("%v", relation["role_code"])
		orgID := fmt.Sprintf("%v", relation["org_id"])
		subject := fmt.Sprintf("%s@%s", userID, orgID)

		_, err = p.casbinSvc.Enforcer.AddRoleForUser(subject, roleCode)
		if err != nil {
			global.Log.Error("添加用户失败",
				zap.String("userID", subject),
				zap.String("roleID", roleCode),
				zap.Error(err))
		}
	}
	return nil
}

// SyncRoleMenusToCasbin 同步用户角色权限到Casbin
func (p *PermissionService) SyncRoleMenusToCasbin(ctx context.Context) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()
	// 获取所有角色菜单关系
	relations, err := roleRepo.GetAllRoleMenuRelations(ctx)
	if err != nil {
		return fmt.Errorf("获取角色菜单关系失败: %w", err)
	}

	// 添加到Casbin (角色对菜单的权限)
	for _, relation := range relations {
		roleCode := fmt.Sprintf("%v", relation["role_code"])
		menuCode := fmt.Sprintf("%v", relation["menu_code"])

		_, err = p.casbinSvc.Enforcer.AddPermissionForUser(roleCode, menuCode, "read")
		if err != nil {
			global.Log.Error("添加角色菜单权限失败",
				zap.String("roleCode", roleCode),
				zap.String("menuCode", menuCode),
				zap.Error(err))
		}
	}

	return nil
}

// SyncMenuAPIsToCasbin 同步菜单API权限到Casbin
func (p *PermissionService) SyncMenuAPIsToCasbin(ctx context.Context) error {
	menuRepo := p.repositoryGroup.SystemRepositorySupplier.GetMenuRepository()

	// 获取所有菜单API关系
	relations, err := menuRepo.GetAllMenuAPIRelations(ctx)
	if err != nil {
		return fmt.Errorf("获取菜单API关系失败: %w", err)
	}

	// 添加到Casbin (菜单对API的权限)
	for _, relation := range relations {
		menuCode := fmt.Sprintf("%v", relation["menu_code"])
		apiPath := fmt.Sprintf("%v", relation["path"])
		apiMethod := fmt.Sprintf("%v", relation["method"])

		// 使用 path:method 作为资源标识
		resource := fmt.Sprintf("%s:%s", apiPath, apiMethod)
		_, err = p.casbinSvc.Enforcer.AddPermissionForUser(menuCode, resource, "access")
		if err != nil {
			global.Log.Error("添加菜单API权限失败",
				zap.String("menuCode", menuCode),
				zap.String("resource", resource),
				zap.Error(err))
		}
	}

	return nil
}

// === 权限验证功能 ===

// CheckUserAPIPermission 检查用户是否有访问指定API的权限
func (p *PermissionService) CheckUserAPIPermission(
	userID uint,
	apiPath, method string,
) (bool, error) {
	subject, _, err := p.getUserSubject(context.Background(), userID)
	if err != nil {
		return false, fmt.Errorf("获取用户上下文失败: %w", err)
	}
	resource := fmt.Sprintf("%s:%s", apiPath, method)

	// 检查用户是否有直接权限
	ok, err := p.casbinSvc.Enforcer.Enforce(subject, resource, "access")
	if err != nil {
		return false, fmt.Errorf("权限检查失败: %w", err)
	}

	return ok, nil
}

// CheckUserMenuPermission 检查用户是否有访问指定菜单的权限
func (p *PermissionService) CheckUserMenuPermission(ctx context.Context, userID uint, menuCode string) (bool, error) {
	subject, _, err := p.getUserSubject(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("获取用户上下文失败: %w", err)
	}

	// 检查用户是否有菜单权限
	ok, err := p.casbinSvc.Enforcer.Enforce(subject, menuCode, "read")
	if err != nil {
		return false, fmt.Errorf("菜单权限检查失败: %w", err)
	}

	return ok, nil
}

func (p *PermissionService) AssignRoleToUserInOrg(
	ctx context.Context,
	userID,
	orgID,
	roleID uint,
) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	role, err := roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("获取角色信息失败: %w", err)
	}

	if err = roleRepo.AssignRoleToUserInOrg(ctx, userID, orgID, roleID); err != nil {
		return fmt.Errorf("数据库分配角色失败: %w", err)
	}

	subject := fmt.Sprintf("%d@%d", userID, orgID)
	_, err = p.casbinSvc.Enforcer.AddRoleForUser(subject, role.Code)
	if err != nil {
		if rollbackErr := roleRepo.RemoveRoleFromUserInOrg(ctx, userID, orgID, roleID); rollbackErr != nil {
			global.Log.Error("数据一致性严重问题：无法回滚数据库操作",
				zap.Uint("userID", userID),
				zap.Uint("roleID", roleID),
				zap.Error(rollbackErr))
		}
		return fmt.Errorf("casbin添加用户角色失败:%w", err)
	}

	return nil
}

// ReplaceUserRolesInOrg 全量替换用户在组织下的角色
func (p *PermissionService) ReplaceUserRolesInOrg(
	ctx context.Context,
	userID, orgID uint,
	roleIDs []uint,
) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	// 1. 获取所有新角色的详细信息（需要RoleCode用于Casbin）
	var roles []entity.Role
	for _, rid := range roleIDs {
		role, err := roleRepo.GetByID(ctx, rid)
		if err != nil {
			return fmt.Errorf("获取角色信息失败(ID=%d): %w", rid, err)
		}
		roles = append(roles, *role)
	}

	// 2. 数据库事务更新
	if err := roleRepo.ReplaceUserOrgRoles(ctx, userID, orgID, roleIDs); err != nil {
		return fmt.Errorf("数据库更新用户角色失败: %w", err)
	}

	// 3. Casbin更新
	subject := fmt.Sprintf("%d@%d", userID, orgID)

	// 3.1 删除该用户在此组织下的所有角色关联
	// DeleteRolesForUser 删除用户的所有角色
	if _, err := p.casbinSvc.Enforcer.DeleteRolesForUser(subject); err != nil {
		global.Log.Error("Casbin删除用户角色失败", zap.String("subject", subject), zap.Error(err))
		// 仅记录错误，不阻断（Casbin状态可能需要后续同步修复）
	}

	// 3.2 添加新角色
	for _, role := range roles {
		if _, err := p.casbinSvc.Enforcer.AddRoleForUser(subject, role.Code); err != nil {
			global.Log.Error("Casbin添加用户角色失败",
				zap.String("subject", subject),
				zap.String("role", role.Code),
				zap.Error(err))
		}
	}

	return nil
}

// AssignMenuToRole 为角色分配菜单权限
func (p *PermissionService) AssignMenuToRole(ctx context.Context, roleID, menuID uint) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()
	menuRepo := p.repositoryGroup.SystemRepositorySupplier.GetMenuRepository()

	// 获取角色和菜单信息
	role, err := roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("获取角色信息失败: %w", err)
	}

	menu, err := menuRepo.GetByID(ctx, menuID)
	if err != nil {
		return fmt.Errorf("获取菜单信息失败: %w", err)
	}

	// 1. 先执行数据库操作
	if err := roleRepo.AssignMenuToRole(ctx, roleID, menuID); err != nil {
		return fmt.Errorf("数据库分配菜单失败: %w", err)
	}

	// 2. 执行Casbin操作
	_, err = p.casbinSvc.Enforcer.AddPermissionForUser(role.Code, menu.Code, "read")
	if err != nil {
		// 3. 补偿：回滚数据库操作
		if rollbackErr := roleRepo.RemoveMenuFromRole(ctx, roleID, menuID); rollbackErr != nil {
			global.Log.Error("数据一致性严重问题：无法回滚数据库操作",
				zap.Uint("roleID", roleID),
				zap.Uint("menuID", menuID),
				zap.Error(rollbackErr))
		}
		return fmt.Errorf("Casbin添加角色菜单权限失败: %w", err)
	}

	return nil
}

// RemoveMenuFromRole 移除角色菜单权限
func (p *PermissionService) RemoveMenuFromRole(ctx context.Context, roleID, menuID uint) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()
	menuRepo := p.repositoryGroup.SystemRepositorySupplier.GetMenuRepository()

	// 获取角色和菜单信息
	role, err := roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("获取角色信息失败: %w", err)
	}

	menu, err := menuRepo.GetByID(ctx, menuID)
	if err != nil {
		return fmt.Errorf("获取菜单信息失败: %w", err)
	}

	// 1. 先执行数据库操作
	if err = roleRepo.RemoveMenuFromRole(ctx, roleID, menuID); err != nil {
		return fmt.Errorf("数据库移除菜单失败: %w", err)
	}

	// 2. 执行Casbin操作
	_, err = p.casbinSvc.Enforcer.RemovePolicy(role.Code, menu.Code, "read")
	if err != nil {
		// 3. 补偿：回滚数据库操作
		if rollbackErr := roleRepo.AssignMenuToRole(ctx, roleID, menuID); rollbackErr != nil {
			global.Log.Error("数据一致性严重问题：无法回滚数据库操作",
				zap.Uint("roleID", roleID),
				zap.Uint("menuID", menuID),
				zap.Error(rollbackErr))
		}
		return fmt.Errorf("Casbin移除角色菜单权限失败: %w", err)
	}

	return nil
}

func (p *PermissionService) RemoveRoleFromUserInOrg(ctx context.Context, userID, orgID, roleID uint) error {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	role, err := roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("获取角色信息失败: %w", err)
	}

	if err = roleRepo.RemoveRoleFromUserInOrg(ctx, userID, orgID, roleID); err != nil {
		return fmt.Errorf("数据库移除角色失败: %w", err)
	}

	subject := fmt.Sprintf("%d@%d", userID, orgID)
	_, err = p.casbinSvc.Enforcer.DeleteRoleForUser(subject, role.Code)
	if err != nil {
		if rollbackErr := roleRepo.AssignRoleToUserInOrg(ctx, userID, orgID, roleID); rollbackErr != nil {
			global.Log.Error("数据一致性严重问题：无法回滚数据库操作",
				zap.Uint("userID", userID),
				zap.Uint("roleID", roleID),
				zap.Error(rollbackErr))
		}
		return fmt.Errorf("Casbin移除用户角色失败: %w", err)
	}

	return nil
}

// === 查询功能 ===

// GetUserRoles 获取用户的所有角色（全局角色 + 当前组织角色）
// 超级管理员拥有全局权限，不依赖组织上下文即可放行
func (p *PermissionService) GetUserRoles(ctx context.Context, userID uint) ([]entity.Role, error) {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	// 1. 先查全局角色（org_id = 0，包含超级管理员等不绑定具体组织的角色）
	globalRoles, err := roleRepo.GetUserGlobalRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取全局角色失败: %w", err)
	}

	// 2. 如果持有超级管理员角色，直接返回，无需组织上下文
	for _, role := range globalRoles {
		if role.Code == consts.RoleCodeSuperAdmin {
			return toRoleSlice(globalRoles), nil
		}
	}

	// 3. 非超管用户，需要检查当前组织
	_, orgID, err := p.getUserSubject(ctx, userID)
	if err != nil || orgID == nil || *orgID == 0 {
		// 如果有全局角色但无组织，仍然返回全局角色
		if len(globalRoles) > 0 {
			return toRoleSlice(globalRoles), nil
		}
		return nil, fmt.Errorf("未设置当前组织")
	}

	// 4. 查询当前组织内的角色
	orgRoles, err := roleRepo.GetUserRolesByOrg(ctx, userID, *orgID)
	if err != nil {
		return nil, fmt.Errorf("获取组织角色失败: %w", err)
	}

	// 5. 合并全局角色和组织角色后返回
	allRoles := append(globalRoles, orgRoles...)
	return toRoleSlice(allRoles), nil
}

// toRoleSlice 将角色指针切片转换为值类型切片
func toRoleSlice(roles []*entity.Role) []entity.Role {
	result := make([]entity.Role, len(roles))
	for i, r := range roles {
		result[i] = *r
	}
	return result
}

// GetUserMenus 获取用户可访问的菜单列表
// 超级管理员直接返回所有启用菜单，普通用户按组织角色权限查询
func (p *PermissionService) GetUserMenus(ctx context.Context, userID uint) ([]entity.Menu, error) {
	menuRepo := p.repositoryGroup.SystemRepositorySupplier.GetMenuRepository()
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	// 1. 检查是否为超级管理员（全局角色）
	globalRoles, _ := roleRepo.GetUserGlobalRoles(ctx, userID)
	for _, role := range globalRoles {
		if role.Code == consts.RoleCodeSuperAdmin {
			// 超管返回所有启用菜单
			allMenus, err := menuRepo.GetActiveMenus(ctx)
			if err != nil {
				return nil, fmt.Errorf("获取全部菜单失败: %w", err)
			}
			result := make([]entity.Menu, len(allMenus))
			for i, m := range allMenus {
				result[i] = *m
			}
			return result, nil
		}
	}

	// 2. 非超管，按原逻辑：根据当前组织查询用户可访问的菜单
	_, orgID, err := p.getUserSubject(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户上下文失败: %w", err)
	}
	if orgID == nil || *orgID == 0 {
		return nil, fmt.Errorf("未设置当前组织")
	}
	menus, err := menuRepo.GetMenusByUserID(ctx, userID, *orgID)
	if err != nil {
		return nil, fmt.Errorf("获取用户菜单失败: %w", err)
	}

	// 转换为值类型切片
	result := make([]entity.Menu, len(menus))
	for i, menu := range menus {
		result[i] = *menu
	}

	return result, nil
}

// GetRoleMenus 获取角色的菜单权限
func (p *PermissionService) GetRoleMenus(ctx context.Context, roleID uint) ([]entity.Menu, error) {
	roleRepo := p.repositoryGroup.SystemRepositorySupplier.GetRoleRepository()

	menus, err := roleRepo.GetRoleMenus(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("获取角色菜单失败: %w", err)
	}

	// 转换为值类型切片
	result := make([]entity.Menu, len(menus))
	for i, menu := range menus {
		result[i] = *menu
	}

	return result, nil
}

// GetUserPermissions 获取用户的所有权限（用于调试）
func (p *PermissionService) GetUserPermissions(ctx context.Context, userID uint) ([][]string, error) {
	subject, _, err := p.getUserSubject(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户上下文失败: %w", err)
	}

	// 重新加载策略确保数据最新
	err = p.casbinSvc.Enforcer.LoadPolicy()
	if err != nil {
		return nil, fmt.Errorf("加载策略失败: %w", err)
	}

	// 获取用户的所有权限
	permissions := p.casbinSvc.Enforcer.GetPermissionsForUser(subject)

	return permissions, nil
}

func (p *PermissionService) getUserSubject(
	ctx context.Context,
	userID uint,
) (string, *uint, error) {
	userRepo := p.repositoryGroup.SystemRepositorySupplier.GetUserRepository()
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, fmt.Errorf("用户不存在")
	}
	if user.CurrentOrgID != nil && *user.CurrentOrgID > 0 {
		subject := fmt.Sprintf("%d@%d", userID, *user.CurrentOrgID)
		return subject, user.CurrentOrgID, nil
	}
	return "", nil, fmt.Errorf("未设置当前组织")
}

// ====系统管理功能====

// ClearAllPermission 清空Casbin中的所有权限数据
func (p *PermissionService) ClearAllPermission(ctx context.Context) error {
	// 使用ClearPolicy清空所有策略和角色关系
	p.casbinSvc.Enforcer.ClearPolicy()

	// 保存策略
	err := p.casbinSvc.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("保存策略失败:%w", err)
	}
	return nil
}

// RefreshAllPermissions 刷新所有权限（重新同步）
func (p *PermissionService) RefreshAllPermissions(ctx context.Context) error {
	return p.SyncAllPermissionsToCasbin(ctx)
}

// ClearAllPermissions 清空所有权限
func (p *PermissionService) ClearAllPermissions(ctx context.Context) {
	p.casbinSvc.Enforcer.ClearPolicy()
}
