package system

import (
	"context"
	"fmt"
	"strings"

	"personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	pkgcasbin "personal_assistant/pkg/casbin"
)

// PermissionProjectionService 负责 Casbin 权限投影的重建、修复和事件消费。
type PermissionProjectionService struct {
	roleRepo          interfaces.RoleRepository
	capabilityRepo    interfaces.CapabilityRepository
	menuRepo          interfaces.MenuRepository
	casbinSvc         *pkgcasbin.Service
	eventPublisher    permissionProjectionEventPublisher
	reloadBroadcaster permissionPolicyReloadBroadcaster
}

// NewPermissionProjectionService 创建权限投影服务实例
func NewPermissionProjectionService(repositoryGroup *repository.Group) *PermissionProjectionService {
	return &PermissionProjectionService{
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		capabilityRepo:    repositoryGroup.SystemRepositorySupplier.GetCapabilityRepository(),
		menuRepo:          repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
		casbinSvc:         pkgcasbin.NewService(),
		eventPublisher:    newPermissionProjectionOutboxPublisher(repositoryGroup.SystemRepositorySupplier.GetOutboxRepository()),
		reloadBroadcaster: newPermissionPolicyReloadBroadcaster(),
	}
}

// RebuildAll 重建所有权限投影
func (s *PermissionProjectionService) RebuildAll(ctx context.Context) error {
	snapshot, err := s.buildPolicySnapshot(ctx)
	if err != nil {
		return err
	}
	return s.casbinSvc.RebuildFromSnapshot(snapshot)
}

// ReloadPolicy 重新加载权限策略
func (s *PermissionProjectionService) ReloadPolicy(ctx context.Context) error {
	_ = ctx
	return s.casbinSvc.ReloadPolicy()
}

// SyncSubjectRoles 同步主体角色
func (s *PermissionProjectionService) SyncSubjectRoles(
	ctx context.Context,
	userID, orgID uint,
) error {
	if userID == 0 {
		return fmt.Errorf("user_id is empty")
	}
	// 权限主体格式为 "userID@orgID"，如果 orgID 为空或0，则格式为 "userID@"
	subject := pkgcasbin.BuildSubject(userID, orgID)
	roles, err := s.roleRepo.GetUserRolesByOrg(ctx, userID, orgID)
	if err != nil {
		return err
	}
	// 提取角色代码列表，去除空值和空格
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		roleCodes = append(roleCodes, strings.TrimSpace(role.Code))
	}
	// 替换 Casbin 中主体的角色列表，确保权限投影数据与数据库中的用户角色关系保持一致
	return s.casbinSvc.ReplaceSubjectRoles(subject, roleCodes)
}

// PublishSubjectBindingChanged 发布主体绑定变更事件（如用户角色变更）
func (s *PermissionProjectionService) PublishSubjectBindingChanged(
	ctx context.Context,
	userID, orgID uint,
) error {
	return s.eventPublisher.Publish(ctx, newSubjectBindingChangedPermissionProjectionEvent(userID, orgID))
}

// PublishSubjectBindingChangedInTx 在事务中发布主体绑定变更事件
func (s *PermissionProjectionService) PublishSubjectBindingChangedInTx(
	ctx context.Context,
	tx any,
	userID, orgID uint,
) error {
	return s.eventPublisher.PublishInTx(ctx, tx, newSubjectBindingChangedPermissionProjectionEvent(userID, orgID))
}

// PublishPermissionGraphChanged 发布权限图变更事件（如权限分配、回收等）
func (s *PermissionProjectionService) PublishPermissionGraphChanged(
	ctx context.Context,
	aggregateType string,
	aggregateID uint,
) error {
	return s.eventPublisher.Publish(ctx, newPermissionGraphChangedProjectionEvent(aggregateType, aggregateID))
}

// PublishPermissionGraphChangedInTx 在事务中发布权限图变更事件
func (s *PermissionProjectionService) PublishPermissionGraphChangedInTx(
	ctx context.Context,
	tx any,
	aggregateType string,
	aggregateID uint,
) error {
	return s.eventPublisher.PublishInTx(ctx, tx, newPermissionGraphChangedProjectionEvent(aggregateType, aggregateID))
}

// Publish 在非事务环境中发布权限投影事件
func (s *PermissionProjectionService) HandlePermissionProjectionEvent(
	ctx context.Context,
	payload *event.PermissionProjectionEvent,
) error {
	if payload == nil {
		return fmt.Errorf("nil permission projection event")
	}
	switch strings.TrimSpace(payload.Kind) {
	case event.PermissionProjectionKindSubjectBindingChanged:
		// 主体绑定变更事件通常是用户角色变更，需要同步用户角色到 Casbin
		if err := s.SyncSubjectRoles(ctx, payload.UserID, payload.OrgID); err != nil {
			return err
		}
	case event.PermissionProjectionKindPermissionGraphChanged:
		// 权限图变更事件可能涉及权限分配、回收等，需要重建整个权限投影以确保数据一致性
		if err := s.RebuildAll(ctx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported permission projection kind: %s", payload.Kind)
	}
	return s.reloadBroadcaster.Broadcast(ctx)
}

// buildPolicySnapshot 构建当前权限数据的快照，用于重建 Casbin 权限投影
func (s *PermissionProjectionService) buildPolicySnapshot(ctx context.Context) (*pkgcasbin.PolicySnapshot, error) {
	// 构建权限主体与角色的映射，以及权限列表
	subjectRoles, err := s.buildSubjectRoles(ctx)
	if err != nil {
		return nil, err
	}
	// 构建权限列表，包括角色-菜单、菜单-API、角色-API和角色-capability等关系
	permissions, err := s.buildPermissions(ctx)
	if err != nil {
		return nil, err
	}
	return &pkgcasbin.PolicySnapshot{
		SubjectRoles: subjectRoles,
		Permissions:  permissions,
	}, nil
}

// buildSubjectRoles 构建权限主体（用户@组织）与角色代码列表的映射
func (s *PermissionProjectionService) buildSubjectRoles(ctx context.Context) (map[string][]string, error) {
	// 获取所有用户-组织-角色关系，构建权限主体与角色的映射
	relations, err := s.roleRepo.GetAllUserOrgRoleRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取用户角色关系失败: %w", err)
	}
	result := make(map[string][]string, len(relations))
	for _, relation := range relations {
		userID := fmt.Sprintf("%v", relation["user_id"])
		orgID := fmt.Sprintf("%v", relation["org_id"])
		roleCode := strings.TrimSpace(fmt.Sprintf("%v", relation["role_code"]))
		if userID == "" || orgID == "" || roleCode == "" {
			continue
		}
		subject := fmt.Sprintf("%s@%s", userID, orgID)
		result[subject] = append(result[subject], roleCode)
	}
	return result, nil
}

// buildPermissions 构建权限列表，包括角色-菜单、菜单-API、角色-API和角色-capability等关系
func (s *PermissionProjectionService) buildPermissions(
	ctx context.Context,
) ([]pkgcasbin.Permission, error) {
	permissions := make([]pkgcasbin.Permission, 0)

	// 获取角色-菜单关系，构建角色对菜单的访问权限
	roleMenuRelations, err := s.roleRepo.GetAllRoleMenuRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取角色菜单关系失败: %w", err)
	}
	for _, relation := range roleMenuRelations {
		permissions = append(permissions, pkgcasbin.Permission{
			Subject: strings.TrimSpace(fmt.Sprintf("%v", relation["role_code"])),
			Object:  strings.TrimSpace(fmt.Sprintf("%v", relation["menu_code"])),
			Action:  pkgcasbin.ActionRead,
		})
	}

	// 获取菜单-API关系，构建菜单对API的访问权限
	menuAPIRelations, err := s.menuRepo.GetAllMenuAPIRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取菜单 API 关系失败: %w", err)
	}
	for _, relation := range menuAPIRelations {
		menuCode := strings.TrimSpace(fmt.Sprintf("%v", relation["menu_code"]))
		apiPath := strings.TrimSpace(fmt.Sprintf("%v", relation["path"]))
		apiMethod := strings.TrimSpace(fmt.Sprintf("%v", relation["method"]))
		if menuCode == "" || apiPath == "" || apiMethod == "" {
			continue
		}
		permissions = append(permissions, pkgcasbin.Permission{
			Subject: menuCode,
			Object:  fmt.Sprintf("%s:%s", apiPath, apiMethod),
			Action:  pkgcasbin.ActionAccess,
		})
	}

	// 获取角色-API关系，构建角色对API的访问权限
	roleAPIRelations, err := s.roleRepo.GetAllRoleAPIRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取角色 API 关系失败: %w", err)
	}
	for _, relation := range roleAPIRelations {
		roleCode := strings.TrimSpace(fmt.Sprintf("%v", relation["role_code"]))
		apiPath := strings.TrimSpace(fmt.Sprintf("%v", relation["path"]))
		apiMethod := strings.TrimSpace(fmt.Sprintf("%v", relation["method"]))
		if roleCode == "" || apiPath == "" || apiMethod == "" {
			continue
		}
		permissions = append(permissions, pkgcasbin.Permission{
			Subject: roleCode,
			Object:  fmt.Sprintf("%s:%s", apiPath, apiMethod),
			Action:  pkgcasbin.ActionAccess,
		})
	}

	// 获取角色-capability关系，构建角色对能力的操作权限
	roleCapabilityRelations, err := s.capabilityRepo.GetAllRoleCapabilityRelations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取角色 capability 关系失败: %w", err)
	}
	for _, relation := range roleCapabilityRelations {
		permissions = append(permissions, pkgcasbin.Permission{
			Subject: strings.TrimSpace(fmt.Sprintf("%v", relation["role_code"])),
			Object:  strings.TrimSpace(fmt.Sprintf("%v", relation["capability_code"])),
			Action:  pkgcasbin.ActionOperate,
		})
	}

	return permissions, nil
}

// newSubjectBindingChangedPermissionProjectionEvent 创建主体绑定变更事件
func newSubjectBindingChangedPermissionProjectionEvent(
	userID, orgID uint,
) *event.PermissionProjectionEvent {
	if userID == 0 {
		return nil
	}
	return &event.PermissionProjectionEvent{
		Kind:          event.PermissionProjectionKindSubjectBindingChanged,
		UserID:        userID,
		OrgID:         orgID,
		AggregateType: "permission_subject",
		AggregateID:   userID,
	}
}

// newPermissionGraphChangedProjectionEvent 创建权限图变更事件
func newPermissionGraphChangedProjectionEvent(
	aggregateType string,
	aggregateID uint,
) *event.PermissionProjectionEvent {
	return &event.PermissionProjectionEvent{
		Kind:          event.PermissionProjectionKindPermissionGraphChanged,
		AggregateType: strings.TrimSpace(aggregateType),
		AggregateID:   aggregateID,
	}
}
