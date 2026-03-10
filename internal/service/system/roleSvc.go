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

const (
	rolePermissionAssignLockTTL           = 10 * time.Second
	rolePermissionAssignLockRetryCount    = 50
	rolePermissionAssignLockRetryInterval = 100 * time.Millisecond
	rolePermissionAssignUnlockTimeout     = 2 * time.Second
)

// RoleService 角色管理服务
type RoleService struct {
	txRunner          repository.TxRunner
	roleRepo          interfaces.RoleRepository
	capabilityRepo    interfaces.CapabilityRepository
	menuRepo          interfaces.MenuRepository
	apiRepo           interfaces.APIRepository
	permissionService *PermissionService
}

// NewRoleService 创建角色服务实例
func NewRoleService(repositoryGroup *repository.Group, permissionService *PermissionService) *RoleService {
	return &RoleService{
		txRunner:          repositoryGroup,
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		capabilityRepo:    repositoryGroup.SystemRepositorySupplier.GetCapabilityRepository(),
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

// AssignPermissions 分配角色菜单和直绑API权限（全量覆盖，单次刷新）
func (s *RoleService) AssignPermissions(
	ctx context.Context,
	roleID uint,
	menuIDs []uint,
	directAPIIDs []uint,
	capabilityCodes []string,
) error {
	if roleID == 0 {
		return errors.New(errors.CodeInvalidParams)
	}

	lock, err := s.acquireRolePermissionAssignLock(ctx, roleID, "权限")
	if err != nil {
		return err
	}
	defer s.releaseRolePermissionAssignLock(roleID, "权限", lock)

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if role == nil || role.ID == 0 {
		return errors.New(errors.CodeRoleNotFound)
	}

	validMenuIDs, err := s.filterValidMenuIDs(ctx, menuIDs)
	if err != nil {
		return err
	}
	validAPIIDs, err := s.filterValidAPIIDs(ctx, directAPIIDs)
	if err != nil {
		return err
	}
	// 过滤有效的 capability code 列表，并获取对应的 capability 实体列表（包含 ID）
	validCapabilities, err := s.filterValidCapabilityCodes(ctx, capabilityCodes)
	if err != nil {
		return err
	}
	capabilityIDs := make([]uint, 0, len(validCapabilities))
	for _, capability := range validCapabilities {
		capabilityIDs = append(capabilityIDs, capability.ID)
	}

	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txRoleRepo := s.roleRepo.WithTx(tx)
		txCapabilityRepo := s.capabilityRepo.WithTx(tx)
		// 全量替换角色菜单关联、直绑API关联和 capability 关联
		if err := txRoleRepo.ReplaceRolePermissions(ctx, roleID, validMenuIDs, validAPIIDs); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		// 替换角色 capability 关联
		if err := txCapabilityRepo.ReplaceRoleCapabilities(ctx, roleID, capabilityIDs); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := s.permissionService.RefreshAllPermissions(ctx); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
	}
	return nil
}

// GetRoleMenuAPIMap 获取角色菜单/API映射（一次性渲染大对象）
func (s *RoleService) GetRoleMenuAPIMap(
	ctx context.Context,
	roleID uint,
	maxLevel *int,
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

	// 获取角色已分配的菜单ID列表
	assignedMenuIDs, err := s.roleRepo.GetRoleMenuIDs(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	// 获取角色通过菜单链路分配的API ID列表
	menuAPIIDs, err := s.menuRepo.GetAPIIDsByMenuIDs(ctx, assignedMenuIDs)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	// 获取角色直绑API ID列表
	directAPIIDs, err := s.roleRepo.GetRoleAPIIDs(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	assignedAPIIDs := normalizeIDs(append(append([]uint{}, menuAPIIDs...), directAPIIDs...))

	assignedMenuIDs = normalizeIDs(assignedMenuIDs)
	directAPIIDs = normalizeIDs(directAPIIDs)
	sort.Slice(assignedMenuIDs, func(i, j int) bool { return assignedMenuIDs[i] < assignedMenuIDs[j] })
	sort.Slice(directAPIIDs, func(i, j int) bool { return directAPIIDs[i] < directAPIIDs[j] })
	sort.Slice(assignedAPIIDs, func(i, j int) bool { return assignedAPIIDs[i] < assignedAPIIDs[j] })
	assignedCapabilityCodes, err := s.permissionService.GetRoleCapabilityCodes(ctx, roleID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	capabilityGroups, err := s.permissionService.GetAllCapabilityGroups(ctx)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}

	return &response.RoleMenuAPIMappingItem{
		MenuTree:                s.buildRoleMenuTree(menus, 0, 1, maxLevel),
		AssignedMenuIDs:         assignedMenuIDs,
		DirectAPIIDs:            directAPIIDs,
		AssignedAPIIDs:          assignedAPIIDs,
		CapabilityGroups:        capabilityGroups,
		AssignedCapabilityCodes: assignedCapabilityCodes,
	}, nil
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

func normalizeCapabilityCodes(codes []string) []string {
	if len(codes) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *RoleService) acquireRolePermissionAssignLock(
	ctx context.Context,
	roleID uint,
	scene string,
) (*redislock.RedisLock, error) {
	lockKey := redislock.LockKeyRolePermissionAssign(roleID)
	lock := redislock.NewRedisLock(ctx, lockKey, rolePermissionAssignLockTTL)
	if err := lock.LockWithRetry(rolePermissionAssignLockRetryCount, rolePermissionAssignLockRetryInterval); err != nil {
		if stderrors.Is(err, redislock.ErrLockFailed) {
			global.Log.Warn("获取角色"+scene+"分配锁失败", zap.Uint("roleID", roleID), zap.Error(err))
			return nil, errors.NewWithMsg(errors.CodeTooManyRequests, "操作过于频繁，请稍后重试")
		}
		global.Log.Error("获取角色"+scene+"分配锁异常", zap.Uint("roleID", roleID), zap.Error(err))
		return nil, errors.Wrap(errors.CodeRedisError, err)
	}
	return lock, nil
}

func (s *RoleService) releaseRolePermissionAssignLock(
	roleID uint,
	scene string,
	lock *redislock.RedisLock,
) {
	if lock == nil {
		return
	}
	unlockCtx, cancel := context.WithTimeout(context.Background(), rolePermissionAssignUnlockTimeout)
	defer cancel()
	if releaseErr := lock.UnlockWithContext(unlockCtx); releaseErr != nil {
		global.Log.Warn("释放角色"+scene+"分配锁失败", zap.Uint("roleID", roleID), zap.Error(releaseErr))
	}
}

func (s *RoleService) filterValidMenuIDs(ctx context.Context, menuIDs []uint) ([]uint, error) {
	validMenuIDs := make([]uint, 0, len(menuIDs))
	for _, menuID := range normalizeIDs(menuIDs) {
		menu, err := s.menuRepo.GetByID(ctx, menuID)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, errors.Wrap(errors.CodeDBError, err)
		}
		if menu != nil && menu.ID > 0 {
			validMenuIDs = append(validMenuIDs, menuID)
		}
	}
	return validMenuIDs, nil
}

func (s *RoleService) filterValidAPIIDs(ctx context.Context, apiIDs []uint) ([]uint, error) {
	validAPIIDs := make([]uint, 0, len(apiIDs))
	for _, apiID := range normalizeIDs(apiIDs) {
		api, err := s.apiRepo.GetByID(ctx, apiID)
		if err != nil {
			if stderrors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, errors.Wrap(errors.CodeDBError, err)
		}
		if api != nil && api.ID > 0 {
			validAPIIDs = append(validAPIIDs, apiID)
		}
	}
	return validAPIIDs, nil
}

func (s *RoleService) filterValidCapabilityCodes(
	ctx context.Context,
	codes []string,
) ([]*entity.Capability, error) {
	normalized := normalizeCapabilityCodes(codes)
	if len(normalized) == 0 {
		return nil, nil
	}

	capabilities, err := s.capabilityRepo.GetByCodes(ctx, normalized)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if len(capabilities) != len(normalized) {
		return nil, errors.NewWithMsg(errors.CodeInvalidParams, "存在无效 capability_codes")
	}
	return capabilities, nil
}

func (s *RoleService) buildRoleMenuTree(
	menus []*entity.Menu,
	parentID uint,
	depth int,
	maxLevel *int,
) []*response.MenuItem {
	result := make([]*response.MenuItem, 0)
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}
		item := s.roleMenuEntityToItem(m)
		if maxLevel == nil || depth < *maxLevel {
			item.Children = s.buildRoleMenuTree(menus, m.ID, depth+1, maxLevel)
		}
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
