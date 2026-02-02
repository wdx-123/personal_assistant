package system

import (
	"context"
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

// CRUD 相关的

// GetByID 采用通过id获取角色
func (r *roleRepository) GetByID(ctx context.Context, id uint) (*entity.Role, error) {
	var role entity.Role
	err := r.db.WithContext(ctx).Preload("Menus").First(&role, id).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepository) GetByCode(ctx context.Context, code string) (*entity.Role, error) {
	var role entity.Role
	err := r.db.WithContext(ctx).Where("code = ?", code).Preload("Menus").First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}
func (r *roleRepository) Create(ctx context.Context, role *entity.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *roleRepository) Update(ctx context.Context, role *entity.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *roleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.Role{}, id).Error
}

// 业务相关查询

// GetRoleList 获取角色列表
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

func (r *roleRepository) GetAllRoles(ctx context.Context) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).Find(&roles).Error
	return roles, err
}

func (r *roleRepository) ExistsByCode(ctx context.Context, code string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.Role{}).Where("code = ?", code).Count(&count).Error
	return count > 0, err
}

func (r *roleRepository) GetActiveRoles(ctx context.Context) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).Where("status = ?", 1).Find(&roles).Error
	return roles, err
}

// 角色菜单关系管理

func (r *roleRepository) AssignMenuToRole(
	ctx context.Context,
	roleID,
	menuID uint,
) error {
	return r.db.WithContext(ctx).
		Exec("INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (?, ?)", roleID, menuID).
		Error
}
func (r *roleRepository) RemoveMenuFromRole(
	ctx context.Context,
	roleID,
	menuID uint,
) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM role_menus WHERE role_id = ? AND menu_id = ?", roleID, menuID).Error
}

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

func (r *roleRepository) GetMenuRoles(ctx context.Context, menuID uint) ([]*entity.Role, error) {
	var roles []*entity.Role
	err := r.db.WithContext(ctx).
		Table("roles").
		Joins("JOIN role_menus ON roles.id = role_menus.role_id").
		Where("role_menus.menu_id = ? AND roles.deleted_at IS NULL", menuID).
		Find(&roles).Error
	return roles, err
}

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
