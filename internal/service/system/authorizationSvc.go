package system

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	pkgcasbin "personal_assistant/pkg/casbin"
	bizerrors "personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

// AuthorizationService 负责业务授权判断，不负责权限投影同步。
type AuthorizationService struct {
	roleRepo   interfaces.RoleRepository
	orgRepo    interfaces.OrgRepository
	userRepo   interfaces.UserRepository
	casbinSvc  *pkgcasbin.Service
}

func NewAuthorizationService(repositoryGroup *repository.Group) *AuthorizationService {
	return &AuthorizationService{
		roleRepo:  repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		orgRepo:   repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		userRepo:  repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		casbinSvc: pkgcasbin.NewService(),
	}
}

// GetUserRoles 获取用户角色
func (s *AuthorizationService) GetUserRoles(ctx context.Context, userID uint) ([]entity.Role, error) {
	globalRoles, err := s.roleRepo.GetUserGlobalRoles(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取全局角色失败: %w", err)
	}
	for _, role := range globalRoles {
		if role != nil && role.Code == consts.RoleCodeSuperAdmin {
			return toRoleSlice(globalRoles), nil
		}
	}

	_, orgID, err := s.getUserSubject(ctx, userID)
	if err != nil || orgID == nil || *orgID == 0 {
		if len(globalRoles) > 0 {
			return toRoleSlice(globalRoles), nil
		}
		return nil, fmt.Errorf("未设置当前组织")
	}

	orgRoles, err := s.roleRepo.GetUserRolesByOrg(ctx, userID, *orgID)
	if err != nil {
		return nil, fmt.Errorf("获取组织角色失败: %w", err)
	}

	return toRoleSlice(append(globalRoles, orgRoles...)), nil
}

// CheckUserAPIPermission 检查用户是否有访问API的权限
func (s *AuthorizationService) CheckUserAPIPermission(
	ctx context.Context,
	userID uint,
	apiPath, method string,
) (bool, error) {
	subject, _, err := s.getUserSubject(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("获取用户上下文失败: %w", err)
	}
	resource := fmt.Sprintf("%s:%s", apiPath, method)
	ok, err := s.casbinSvc.HasPermission(subject, resource, pkgcasbin.ActionAccess)
	if err != nil {
		return false, fmt.Errorf("权限检查失败: %w", err)
	}
	return ok, nil
}

func (s *AuthorizationService) CheckUserCapabilityInOrg(
	ctx context.Context,
	userID, orgID uint,
	capabilityCode string,
) (bool, error) {
	_ = ctx
	subject := pkgcasbin.BuildSubject(userID, orgID)
	return s.checkSubjectCapability(subject, capabilityCode)
}

// CanOperateOrgCapability 检查用户是否有权限操作组织能力（如授权/撤销组织能力）
func (s *AuthorizationService) CanOperateOrgCapability(
	ctx context.Context,
	operatorID, orgID uint,
	capabilityCode string,
) (bool, error) {
	if s == nil {
		return false, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "授权服务未初始化")
	}
	if strings.TrimSpace(capabilityCode) == "" {
		return false, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "capability_code 不能为空")
	}

	// 首先检查用户是否是组织所有者或超级管理员
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, bizerrors.New(bizerrors.CodeOrgNotFound)
		}
		return false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if org == nil {
		return false, bizerrors.New(bizerrors.CodeOrgNotFound)
	}
	if operatorID == org.OwnerID {
		return true, nil
	}

	// 检查用户是否具有全局超级管理员角色
	globalRoles, err := s.roleRepo.GetUserGlobalRoles(ctx, operatorID)
	if err != nil {
		return false, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	for _, role := range globalRoles {
		if role != nil && role.Code == consts.RoleCodeSuperAdmin {
			return true, nil
		}
	}

	ok, err := s.CheckUserCapabilityInOrg(ctx, operatorID, orgID, capabilityCode)
	if err != nil {
		return false, bizerrors.Wrap(bizerrors.CodeInternalError, err)
	}
	return ok, nil
}

// AuthorizeOrgCapability 授权组织能力
func (s *AuthorizationService) AuthorizeOrgCapability(
	ctx context.Context,
	operatorID, orgID uint,
	capabilityCode string,
) error {
	ok, err := s.CanOperateOrgCapability(ctx, operatorID, orgID, capabilityCode)
	if err != nil {
		return err
	}
	if !ok {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	return nil
}

// checkSubjectCapability 检查权限主体是否具有指定能力
func (s *AuthorizationService) checkSubjectCapability(subject string, capabilityCode string) (bool, error) {
	subject = strings.TrimSpace(subject)
	capabilityCode = strings.TrimSpace(capabilityCode)
	if subject == "" || capabilityCode == "" {
		return false, fmt.Errorf("subject 或 capabilityCode 为空")
	}
	ok, err := s.casbinSvc.HasPermission(subject, capabilityCode, pkgcasbin.ActionOperate)
	if err != nil {
		return false, fmt.Errorf("capability 权限检查失败: %w", err)
	}
	return ok, nil
}

// getUserSubject 获取用户的权限主体字符串和当前组织ID
func (s *AuthorizationService) getUserSubject(
	ctx context.Context,
	userID uint,
) (string, *uint, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, fmt.Errorf("用户不存在")
	}
	if user.CurrentOrgID != nil {
		subject := pkgcasbin.BuildSubject(userID, *user.CurrentOrgID)
		return subject, user.CurrentOrgID, nil
	}
	return "", nil, fmt.Errorf("未设置当前组织")
}

// toRoleSlice 将 []*entity.Role 转换为 []entity.Role，过滤掉 nil 值
func toRoleSlice(roles []*entity.Role) []entity.Role {
	result := make([]entity.Role, 0, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		result = append(result, *role)
	}
	return result
}
