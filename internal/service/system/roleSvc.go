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
	stderrors "errors"
	"sort"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/redislock"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RoleService 角色管理服务
type RoleService struct {
	roleRepo          interfaces.RoleRepository
	menuRepo          interfaces.MenuRepository
	apiRepo           interfaces.APIRepository
	permissionService *PermissionService
}

// NewRoleService 创建角色服务实例
func NewRoleService(repositoryGroup *repository.Group, permissionService *PermissionService) *RoleService {
	return &RoleService{
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		menuRepo:          repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
		apiRepo:           repositoryGroup.SystemRepositorySupplier.GetAPIRepository(),
		permissionService: permissionService,
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
	// 清空角色直绑API关联
	if err := s.roleRepo.ClearRoleAPIs(ctx, id); err != nil {
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
func (s *RoleService) AssignMenus(
	ctx context.Context,
	roleID uint,
	menuIDs []uint,
) error {
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

	// 5. 同步刷新权限（立即生效）
	if err := s.permissionService.RefreshAllPermissions(ctx); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
	}

	return nil
}

// AssignAPIs 分配角色直绑API权限（全量覆盖，使用分布式锁防止并发）
func (s *RoleService) AssignAPIs(
	ctx context.Context,
	roleID uint,
	apiIDs []uint,
) error {
	if roleID == 0 {
		return errors.New(errors.CodeInvalidParams)
	}

	lockKey := redislock.LockKeyRoleMenuAssign(roleID)
	lock := redislock.NewRedisLock(ctx, lockKey, 10*time.Second)
	if err := lock.Lock(); err != nil {
		global.Log.Warn("获取角色API分配锁失败", zap.Uint("roleID", roleID), zap.Error(err))
		return errors.NewWithMsg(errors.CodeTooManyRequests, "操作过于频繁，请稍后重试")
	}
	defer func() {
		if releaseErr := lock.Unlock(); releaseErr != nil {
			global.Log.Warn("释放角色API分配锁失败", zap.Uint("roleID", roleID), zap.Error(releaseErr))
		}
	}()
	// 校验角色存在
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return errors.New(errors.CodeRoleNotFound)
	}
	// 过滤有效的API ID
	validAPIIDs := make([]uint, 0, len(apiIDs))
	for _, apiID := range normalizeIDs(apiIDs) {
		api, getErr := s.apiRepo.GetByID(ctx, apiID)
		if getErr != nil {
			if stderrors.Is(getErr, gorm.ErrRecordNotFound) {
				continue
			}
			return errors.Wrap(errors.CodeDBError, getErr)
		}
		if api != nil && api.ID > 0 {
			validAPIIDs = append(validAPIIDs, apiID)
		}
	}

	// 全量替换角色直绑API关联
	if err := s.roleRepo.ReplaceRoleAPIs(ctx, roleID, validAPIIDs); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}

	// 同步刷新权限（立即生效）
	if err := s.permissionService.RefreshAllPermissions(ctx); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
	}
	return nil
}

// GetRoleMenuAPIMap 获取角色菜单/API映射（一次性渲染大对象）
func (s *RoleService) GetRoleMenuAPIMap(
	ctx context.Context,
	roleID uint,
) (*response.RoleMenuAPIMappingItem, error) {
	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return nil, errors.New(errors.CodeRoleNotFound)
	}

	menus, err := s.menuRepo.GetAllMenusWithAPIs(ctx)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}

	return &response.RoleMenuAPIMappingItem{
		MenuTree: s.buildRoleMenuTree(menus, 0),
	}, nil
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

func normalizeIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *RoleService) buildRoleMenuTree(menus []*entity.Menu, parentID uint) []*response.MenuItem {
	result := make([]*response.MenuItem, 0)
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}
		item := s.roleMenuEntityToItem(m)
		item.Children = s.buildRoleMenuTree(menus, m.ID)
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Sort == result[j].Sort {
			return result[i].ID < result[j].ID
		}
		return result[i].Sort < result[j].Sort
	})
	return result
}

func (s *RoleService) roleMenuEntityToItem(m *entity.Menu) *response.MenuItem {
	item := &response.MenuItem{
		ID:            m.ID,
		ParentID:      m.ParentID,
		Name:          m.Name,
		Code:          m.Code,
		Type:          m.Type,
		Icon:          m.Icon,
		RouteName:     m.RouteName,
		RoutePath:     m.RoutePath,
		RouteParam:    m.RouteParam,
		ComponentPath: m.ComponentPath,
		Status:        m.Status,
		Sort:          m.Sort,
		Desc:          m.Desc,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	if len(m.APIs) > 0 {
		apis := make([]*response.APIItemSimple, 0, len(m.APIs))
		for _, api := range m.APIs {
			apis = append(apis, &response.APIItemSimple{
				ID:     api.ID,
				Path:   api.Path,
				Method: api.Method,
			})
		}
		sort.Slice(apis, func(i, j int) bool {
			return apis[i].ID < apis[j].ID
		})
		item.APIs = apis
	}
	return item
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
