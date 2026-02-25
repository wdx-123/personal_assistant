package system

import (
	"context"
	"strings"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type roleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) interfaces.RoleRepository {
	return &roleRepository{db: db}
}

// WithTx 启用事务
func (r *roleRepository) WithTx(tx any) interfaces.RoleRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &roleRepository{db: transaction}
	}
	return r
}

// ==================== CRUD 相关 ====================

// GetByID 通过ID获取角色
func (r *roleRepository) GetByID(ctx context.Context, id uint) (*entity.Role, error) {
	var role entity.Role
	err := r.db.WithContext(ctx).Preload("Menus").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// GetByCode 通过Code获取角色
func (r *roleRepository) GetByCode(ctx context.Context, code string) (*entity.Role, error) {
	var role entity.Role
	err := r.db.WithContext(ctx).Where("code = ?", code).Preload("Menus").First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// Create 创建角色
func (r *roleRepository) Create(ctx context.Context, role *entity.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// Update 更新角色
func (r *roleRepository) Update(ctx context.Context, role *entity.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete 删除角色（软删除）
func (r *roleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.Role{}, id).Error
}

// ==================== 业务相关查询 ====================

// GetRoleList 获取角色列表（基础分页）
func (r *roleRepository) GetRoleList(ctx context.Context, page, pageSize int) ([]*entity.Role, int64, error) {
	var roles []*entity.Role
	var total int64

	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).Model(&entity.Role{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&roles).Error
	return roles, total, err
}

// GetRoleListWithFilter 获取角色列表（支持过滤条件）
func (r *roleRepository) GetRoleListWithFilter(
	ctx context.Context,
	filter *request.RoleListFilter,
) ([]*entity.Role, int64, error) {
	var roles []*entity.Role
	var total int64

	query := r.db.WithContext(ctx).Model(&entity.Role{})

	// 状态过滤
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	// 关键词搜索（名称或代码）
	if filter.Keyword != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", keyword, keyword)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (filter.Page - 1) * filter.PageSize
	err := query.Order("id DESC").Offset(offset).Limit(filter.PageSize).Find(&roles).Error
	return roles, total, err
}

// GetAllRoles 获取所有角色
func (r *roleRepository) GetAllRoles(ctx context.Context) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).Find(&roles).Error
	return roles, err
}

// ExistsByCode 检查角色代码是否存在
func (r *roleRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Role{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

// ExistsByCodeExcludeID 检查角色代码是否存在（排除指定ID，用于更新时校验）
func (r *roleRepository) ExistsByCodeExcludeID(ctx context.Context, code string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Role{}).
		Where("code = ? AND id != ?", code, excludeID).
		Count(&count).Error
	return count > 0, err
}

// GetActiveRoles 获取所有启用的角色
func (r *roleRepository) GetActiveRoles(ctx context.Context) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).Where("status = ?", 1).Find(&roles).Error
	return roles, err
}

// IsRoleInUse 检查角色是否正在被使用（有用户关联）
func (r *roleRepository) IsRoleInUse(ctx context.Context, roleID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_org_roles").
		Where("role_id = ?", roleID).
		Count(&count).Error
	return count > 0, err
}

// ==================== 角色菜单关系管理 ====================

// AssignMenuToRole 为角色分配单个菜单
func (r *roleRepository) AssignMenuToRole(
	ctx context.Context,
	roleID,
	menuID uint,
) error {
	return r.db.WithContext(ctx).
		Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (?, ?)", roleID, menuID).
		Error
}

// RemoveMenuFromRole 从角色移除单个菜单
func (r *roleRepository) RemoveMenuFromRole(
	ctx context.Context,
	roleID,
	menuID uint,
) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM role_menus WHERE role_id = ? AND menu_id = ?", roleID, menuID).Error
}

// GetRoleMenus 获取角色关联的菜单列表
func (r *roleRepository) GetRoleMenus(
	ctx context.Context,
	roleID uint,
) ([]*entity.Menu, error) {
	var menus []*entity.Menu
	err := r.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Where("role_menus.role_id = ? AND menus.deleted_at IS NULL", roleID).
		Find(&menus).Error
	return menus, err
}

// GetRoleMenuIDs 获取角色关联的菜单ID列表
func (r *roleRepository) GetRoleMenuIDs(ctx context.Context, roleID uint) ([]uint, error) {
	var menuIDs []uint
	err := r.db.WithContext(ctx).
		Table("menus").
		Joins("JOIN role_menus ON menus.id = role_menus.menu_id").
		Where("role_menus.role_id = ? AND menus.deleted_at IS NULL", roleID).
		Pluck("menus.id", &menuIDs).Error
	return menuIDs, err
}

// ReplaceRoleMenus 全量替换角色的菜单权限（事务，先删后增）
func (r *roleRepository) ReplaceRoleMenus(
	ctx context.Context,
	roleID uint,
	menuIDs []uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 删除旧关联 (硬删除关联表记录)
		if err := tx.Exec("DELETE FROM role_menus WHERE role_id = ?", roleID).Error; err != nil {
			return err
		}

		// 2. 批量插入新关联
		if len(menuIDs) == 0 {
			return nil
		}

		// 3. 构造 Role 对象并使用 Association 更新，这样更安全且兼容性好
		role := entity.Role{}
		role.ID = roleID

		menus := make([]entity.Menu, len(menuIDs))
		for i, id := range menuIDs {
			menus[i].ID = id
		}

		// 使用 GORM 的 Association 模式，它会自动处理中间表的插入
		// 由于我们刚才已经手动 DELETE 清空了，这里 Append 即可
		return tx.Model(&role).Association("Menus").Append(menus)
	})
}

// ClearRoleMenus 清空角色的所有菜单关联
func (r *roleRepository) ClearRoleMenus(ctx context.Context, roleID uint) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM role_menus WHERE role_id = ?", roleID).Error
}

// ReplaceRoleAPIs 全量替换角色直绑API权限（事务，先删后增）
func (r *roleRepository) ReplaceRoleAPIs(
	ctx context.Context,
	roleID uint,
	apiIDs []uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM role_apis WHERE role_id = ?", roleID).Error; err != nil {
			return err
		}
		if len(apiIDs) == 0 {
			return nil
		}

		var values []string
		var args []interface{}
		for _, apiID := range apiIDs {
			values = append(values, "(?, ?)")
			args = append(args, roleID, apiID)
		}
		sql := "INSERT IGNORE INTO role_apis (role_id, api_id) VALUES " + strings.Join(values, ",")
		return tx.Exec(sql, args...).Error
	})
}

// GetRoleAPIIDs 获取角色直绑API ID列表（仅返回未软删API）
func (r *roleRepository) GetRoleAPIIDs(
	ctx context.Context,
	roleID uint,
) ([]uint, error) {
	var apiIDs []uint
	err := r.db.WithContext(ctx).
		Table("apis").
		Joins("JOIN role_apis ON apis.id = role_apis.api_id").
		Where("role_apis.role_id = ? AND apis.deleted_at IS NULL", roleID).
		Pluck("apis.id", &apiIDs).Error
	return apiIDs, err
}

// ClearRoleAPIs 清空角色的所有API直绑关联
func (r *roleRepository) ClearRoleAPIs(ctx context.Context, roleID uint) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM role_apis WHERE role_id = ?", roleID).Error
}

// RemoveAPIFromAllRoles 从所有角色中移除指定API（删除API前解绑）
func (r *roleRepository) RemoveAPIFromAllRoles(ctx context.Context, apiID uint) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM role_apis WHERE api_id = ?", apiID).Error
}

// GetMenuRoles 获取菜单所属的角色列表
func (r *roleRepository) GetMenuRoles(
	ctx context.Context,
	menuID uint,
) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN role_menus ON roles.id = role_menus.role_id").
		Where("role_menus.menu_id = ? AND roles.deleted_at IS NULL", menuID).
		Find(&roles).Error
	return roles, err
}

// ==================== 用户角色关系管理 ====================

// AssignRoleToUserInOrg 添加角色到用户组织中
func (r *roleRepository) AssignRoleToUserInOrg(
	ctx context.Context,
	userID,
	orgID,
	roleID uint,
) error {
	return r.db.WithContext(ctx).
		Exec("INSERT IGNORE INTO user_org_roles (user_id, org_id, role_id) VALUES (?, ?, ?)",
			userID, orgID, roleID).
		Error
}

// RemoveRoleFromUserInOrg 移除角色从用户组织中
func (r *roleRepository) RemoveRoleFromUserInOrg(
	ctx context.Context,
	userID,
	orgID,
	roleID uint,
) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM user_org_roles WHERE user_id = ? AND org_id = ? AND role_id = ?",
			userID, orgID, roleID).
		Error
}

// ReplaceUserOrgRoles 全量替换用户在组织下的角色
func (r *roleRepository) ReplaceUserOrgRoles(
	ctx context.Context,
	userID, orgID uint,
	roleIDs []uint,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 删除旧关联
		if err := tx.Exec("DELETE FROM user_org_roles WHERE user_id = ? AND org_id = ?", userID, orgID).Error; err != nil {
			return err
		}

		// 2. 批量插入新关联
		if len(roleIDs) == 0 {
			return nil
		}

		var values []string
		var args []interface{}
		for _, roleID := range roleIDs {
			values = append(values, "(?, ?, ?)")
			args = append(args, userID, orgID, roleID)
		}

		sql := "INSERT INTO user_org_roles (user_id, org_id, role_id) VALUES " + strings.Join(values, ",")
		return tx.Exec(sql, args...).Error
	})
}

// GetUserRolesByOrg 获取用户组织中的角色
func (r *roleRepository) GetUserRolesByOrg(ctx context.Context, userID, orgID uint) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_org_roles ON roles.id = user_org_roles.role_id").
		Where("user_org_roles.user_id = ? AND user_org_roles.org_id = ? AND roles.deleted_at IS NULL",
			userID, orgID).
		Find(&roles).Error
	return roles, err
}

// GetUserGlobalRoles 获取用户的全局角色（org_id = 0，如超级管理员等不绑定具体组织的角色）
func (r *roleRepository) GetUserGlobalRoles(ctx context.Context, userID uint) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN user_org_roles ON roles.id = user_org_roles.role_id").
		Where("user_org_roles.user_id = ? AND user_org_roles.org_id = 0 AND roles.deleted_at IS NULL", userID).
		Find(&roles).Error
	return roles, err
}

// ClearRoleUserRelations 清空角色的所有用户关联
func (r *roleRepository) ClearRoleUserRelations(ctx context.Context, roleID uint) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM user_org_roles WHERE role_id = ?", roleID).Error
}

func (r *roleRepository) GetAllRoleMenuRelations(ctx context.Context) ([]map[string]interface{}, error) {
	var relations []map[string]interface{}
	err := r.db.WithContext(ctx).
		Table("role_menus").
		Select("roles.code as role_code, menus.code as menu_code").
		Joins("JOIN roles ON role_menus.role_id = roles.id").
		Joins("JOIN menus ON role_menus.menu_id = menus.id").
		Where("roles.deleted_at IS NULL AND menus.deleted_at IS NULL").
		Find(&relations).Error
	return relations, err
}

// GetAllRoleAPIRelations 获取所有角色与直绑API关联关系（用于Casbin同步）
func (r *roleRepository) GetAllRoleAPIRelations(ctx context.Context) ([]map[string]interface{}, error) {
	var relations []map[string]interface{}
	err := r.db.WithContext(ctx).
		Table("role_apis").
		Select("roles.code as role_code, apis.path, apis.method").
		Joins("JOIN roles ON role_apis.role_id = roles.id").
		Joins("JOIN apis ON role_apis.api_id = apis.id").
		Where("roles.deleted_at IS NULL AND apis.deleted_at IS NULL").
		Find(&relations).Error
	return relations, err
}

func (r *roleRepository) GetAllUserOrgRoleRelations(
	ctx context.Context,
) ([]map[string]interface{}, error) {
	var relations []map[string]interface{}
	err := r.db.WithContext(ctx).
		Table("user_org_roles").
		Select("users.id as user_id, roles.code as role_code, user_org_roles.org_id as org_id").
		Joins("JOIN users ON user_org_roles.user_id = users.id").
		Joins("JOIN roles ON user_org_roles.role_id = roles.id").
		Where("users.deleted_at IS NULL AND roles.deleted_at IS NULL").
		Find(&relations).Error
	return relations, err
}
