/**
 * @projectName: personal_assistant
 * @package: system
 * @className: roleSvc
 * @author: lijunqi
 * @description: 角色管理服务，提供角色CRUD及菜单权限分配功能
 * @date: 2026-02-02
 * @Version: 1.0
 */
package system

import (
	"context"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/redislock"

	"go.uber.org/zap"
)

// RoleService 角色管理服务
type RoleService struct {
	roleRepo interfaces.RoleRepository
	menuRepo interfaces.MenuRepository
}

// NewRoleService 创建角色服务实例
func NewRoleService(repositoryGroup *repository.Group) *RoleService {
	return &RoleService{
		roleRepo: repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		menuRepo: repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
	}
}

// ==================== 角色CRUD ====================

// GetRoleList 获取角色列表（分页，支持过滤）
func (s *RoleService) GetRoleList(
	ctx context.Context,
	filter *request.RoleListFilter,
) ([]*entity.Role, int64, error) {
	if filter == nil {
		filter = &request.RoleListFilter{}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)

	roles, total, err := s.roleRepo.GetRoleListWithFilter(ctx, filter)
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}
	return roles, total, nil
}

// GetRoleByID 根据ID获取角色详情
func (s *RoleService) GetRoleByID(ctx context.Context, id uint) (*entity.Role, error) {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return nil, errors.New(errors.CodeRoleNotFound)
	}
	return role, nil
}

// CreateRole 创建角色
func (s *RoleService) CreateRole(ctx context.Context, req *request.CreateRoleReq) error {
	name := strings.TrimSpace(req.Name)
	code := strings.TrimSpace(req.Code)

	if name == "" || code == "" {
		return errors.New(errors.CodeInvalidParams)
	}

	// 检查code唯一性
	exists, err := s.roleRepo.ExistsByCode(ctx, code)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if exists {
		return errors.New(errors.CodeRoleAlreadyExists)
	}

	role := &entity.Role{
		Name:   name,
		Code:   code,
		Desc:   strings.TrimSpace(req.Desc),
		Status: 1, // 默认启用
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// UpdateRole 更新角色（支持部分更新）
func (s *RoleService) UpdateRole(ctx context.Context, id uint, req *request.UpdateRoleReq) error {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return errors.New(errors.CodeRoleNotFound)
	}

	// 系统内置角色保护（不允许修改code）
	if consts.IsBuiltinRole(role.Code) && req.Code != nil && *req.Code != role.Code {
		return errors.NewWithMsg(errors.CodeInvalidParams, "系统内置角色不允许修改代码")
	}

	// 部分更新
	if req.Name != nil {
		role.Name = strings.TrimSpace(*req.Name)
	}
	if req.Code != nil {
		newCode := strings.TrimSpace(*req.Code)
		if newCode != role.Code {
			// 检查新code唯一性（排除自身）
			exists, err := s.roleRepo.ExistsByCodeExcludeID(ctx, newCode, id)
			if err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			if exists {
				return errors.New(errors.CodeRoleAlreadyExists)
			}
			role.Code = newCode
		}
	}
	if req.Desc != nil {
		role.Desc = strings.TrimSpace(*req.Desc)
	}
	if req.Status != nil {
		role.Status = *req.Status
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// DeleteRole 删除角色
func (s *RoleService) DeleteRole(ctx context.Context, id uint) error {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return errors.New(errors.CodeRoleNotFound)
	}

	// 系统内置角色不允许删除
	if consts.IsBuiltinRole(role.Code) {
		return errors.NewWithMsg(errors.CodeInvalidParams, "系统内置角色不允许删除")
	}

	// 检查角色是否正在被使用
	inUse, err := s.roleRepo.IsRoleInUse(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if inUse {
		return errors.NewWithMsg(errors.CodeInvalidParams, "该角色正在被使用，无法删除")
	}

	// 清空菜单关联
	if err := s.roleRepo.ClearRoleMenus(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}

	// 软删除角色
	if err := s.roleRepo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// ==================== 菜单权限分配 ====================

// AssignMenus 分配菜单权限（全量覆盖，使用分布式锁防止并发）
func (s *RoleService) AssignMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	// 1. 获取分布式锁，防止并发修改同一角色的菜单
	lockKey := redislock.LockKeyRoleMenuAssign(roleID)
	lock := redislock.NewRedisLock(ctx, lockKey, 10*time.Second)
	if err := lock.Lock(); err != nil {
		global.Log.Warn("获取角色菜单分配锁失败", zap.Uint("roleID", roleID), zap.Error(err))
		return errors.NewWithMsg(errors.CodeTooManyRequests, "操作过于频繁，请稍后重试")
	}
	defer func() {
		if releaseErr := lock.Unlock(); releaseErr != nil {
			global.Log.Warn("释放角色菜单分配锁失败", zap.Uint("roleID", roleID), zap.Error(releaseErr))
		}
	}()

	// 2. 校验角色存在
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return errors.New(errors.CodeRoleNotFound)
	}

	// 3. 过滤有效的菜单ID
	validMenuIDs := make([]uint, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		menu, err := s.menuRepo.GetByID(ctx, menuID)
		if err == nil && menu != nil && menu.ID > 0 {
			validMenuIDs = append(validMenuIDs, menuID)
		}
	}

	// 4. 全量替换菜单权限
	if err := s.roleRepo.ReplaceRoleMenus(ctx, roleID, validMenuIDs); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}

	// 5. 同步到Casbin（异步或即时）
	go s.syncRoleMenusToCasbin(role.Code, validMenuIDs)

	return nil
}

// GetRoleMenuIDs 获取角色已分配的菜单ID列表
func (s *RoleService) GetRoleMenuIDs(ctx context.Context, roleID uint) ([]uint, error) {
	// 校验角色存在
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return nil, errors.New(errors.CodeRoleNotFound)
	}

	menuIDs, err := s.roleRepo.GetRoleMenuIDs(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	return menuIDs, nil
}

// ==================== 内部方法 ====================

// syncRoleMenusToCasbin 同步角色菜单到Casbin（异步执行）
func (s *RoleService) syncRoleMenusToCasbin(roleCode string, menuIDs []uint) {
	// 检查Casbin执行器是否初始化
	if global.CasbinEnforcer == nil {
		global.Log.Warn("Casbin执行器未初始化，跳过权限同步")
		return
	}

	ctx := context.Background()

	// 获取菜单code列表
	menuCodes := make([]string, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		menu, err := s.menuRepo.GetByID(ctx, menuID)
		if err == nil && menu != nil {
			menuCodes = append(menuCodes, menu.Code)
		}
	}

	// 删除角色的旧策略
	_, err := global.CasbinEnforcer.RemoveFilteredPolicy(0, roleCode)
	if err != nil {
		global.Log.Error("Casbin删除角色策略失败",
			zap.String("roleCode", roleCode),
			zap.Error(err))
		return
	}

	// 添加新策略
	for _, menuCode := range menuCodes {
		_, err := global.CasbinEnforcer.AddPolicy(roleCode, menuCode, "access")
		if err != nil {
			global.Log.Warn("Casbin添加策略失败",
				zap.String("roleCode", roleCode),
				zap.String("menuCode", menuCode),
				zap.Error(err))
		}
	}

	global.Log.Info("角色菜单同步到Casbin完成",
		zap.String("roleCode", roleCode),
		zap.Int("menuCount", len(menuCodes)))
}

// ==================== 辅助方法 ====================

// GetAllActiveRoles 获取所有启用的角色（用于下拉选择）
func (s *RoleService) GetAllActiveRoles(ctx context.Context) ([]*entity.Role, error) {
	roles, err := s.roleRepo.GetActiveRoles(ctx)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	return roles, nil
}
