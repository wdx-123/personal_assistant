package system

import (
	"context"
	"strings"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
)

// OrgService 组织管理服务
type OrgService struct {
	orgRepo  interfaces.OrgRepository
	userRepo interfaces.UserRepository
	roleRepo interfaces.RoleRepository
}

// NewOrgService 创建组织服务实例
func NewOrgService(repositoryGroup *repository.Group) *OrgService {
	return &OrgService{
		orgRepo:  repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		userRepo: repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo: repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
	}
}

// ==================== 查询 ====================

// GetOrgList 获取组织列表（支持分页与不分页、关键词搜索）
// 如果 page <= 0，则返回所有数据
func (s *OrgService) GetOrgList(
	ctx context.Context,
	page, pageSize int,
	keyword string,
) ([]*entity.Org, int64, error) {
	keyword = strings.TrimSpace(keyword)

	if page <= 0 {
		// 不分页，获取所有
		list, err := s.orgRepo.GetAllOrgs(ctx)
		if err != nil {
			return nil, 0, errors.Wrap(errors.CodeDBError, err)
		}
		// 简单内存过滤关键词（全量查询场景数据量小）
		if keyword != "" {
			filtered := make([]*entity.Org, 0, len(list))
			for _, org := range list {
				if strings.Contains(org.Name, keyword) {
					filtered = append(filtered, org)
				}
			}
			return filtered, int64(len(filtered)), nil
		}
		return list, int64(len(list)), nil
	}

	// 分页查询（支持关键词搜索）
	orgs, total, err := s.orgRepo.GetOrgListWithKeyword(ctx, page, pageSize, keyword)
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}
	return orgs, total, nil
}

// GetOrgDetail 获取组织详情
func (s *OrgService) GetOrgDetail(
	ctx context.Context,
	id uint,
) (*entity.Org, error) {
	org, err := s.orgRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(errors.CodeOrgNotFound, err)
	}
	return org, nil
}

// GetMyOrgs 获取当前用户所属的组织列表
func (s *OrgService) GetMyOrgs(ctx context.Context, userID uint) ([]*response.MyOrgItem, error) {
	orgs, err := s.orgRepo.GetOrgsByUserID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}

	items := make([]*response.MyOrgItem, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, &response.MyOrgItem{
			ID:      org.ID,
			Name:    org.Name,
			IsOwner: org.OwnerID == userID,
		})
	}
	return items, nil
}

// ==================== 写操作 ====================

// CreateOrg 创建组织
// 创建者自动成为 owner，并自动加入组织（user_org_roles）
func (s *OrgService) CreateOrg(
	ctx context.Context,
	userID uint,
	req *request.CreateOrgReq,
) error {
	// 校验参数
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errors.New(errors.CodeInvalidParams)
	}

	// 校验名称唯一性
	exists, err := s.orgRepo.ExistsByName(ctx, name)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if exists {
		return errors.New(errors.CodeOrgNameDuplicate)
	}

	// 创建组织
	org := &entity.Org{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Code:        strings.TrimSpace(req.Code),
		OwnerID:     userID,
	}
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}

	// 自动将创建者设为组织管理员（org_admin），拥有该组织的最高管理权限
	orgAdminRole, roleErr := s.roleRepo.GetByCode(ctx, consts.RoleCodeOrgAdmin)
	if roleErr == nil && orgAdminRole != nil && orgAdminRole.ID > 0 {
		_ = s.roleRepo.AssignRoleToUserInOrg(ctx, userID, org.ID, orgAdminRole.ID)
	}

	// 设置为用户的当前组织
	_ = s.userRepo.UpdateCurrentOrgID(ctx, userID, &org.ID)

	return nil
}

// UpdateOrg 更新组织信息（仅组织所有者可操作）
func (s *OrgService) UpdateOrg(ctx context.Context, userID, orgID uint, req *request.UpdateOrgReq) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}

	// 校验权限：仅 owner 可操作
	if org.OwnerID != userID {
		return errors.New(errors.CodeOrgOwnerOnly)
	}

	// 部分更新
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name != "" && name != org.Name {
			// 校验新名称唯一性
			exists, err := s.orgRepo.ExistsByName(ctx, name)
			if err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			if exists {
				return errors.New(errors.CodeOrgNameDuplicate)
			}
			org.Name = name
		}
	}
	if req.Description != nil {
		org.Description = strings.TrimSpace(*req.Description)
	}
	if req.Code != nil {
		org.Code = strings.TrimSpace(*req.Code)
	}

	if err := s.orgRepo.Update(ctx, org); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// DeleteOrg 删除组织（仅组织所有者可操作）
// force 为 true 时跳过成员数检查
func (s *OrgService) DeleteOrg(ctx context.Context, userID, orgID uint, force bool) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}

	// 校验权限：仅 owner 可操作
	if org.OwnerID != userID {
		return errors.New(errors.CodeOrgOwnerOnly)
	}

	// 非强制删除时，检查组织下是否还有成员
	if !force {
		memberCount, err := s.orgRepo.CountMembersByOrgID(ctx, orgID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if memberCount > 0 {
			return errors.New(errors.CodeOrgHasMembers)
		}
	}

	// 执行删除
	if err := s.orgRepo.Delete(ctx, orgID); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// SetCurrentOrg 切换用户当前组织
func (s *OrgService) SetCurrentOrg(ctx context.Context, userID, orgID uint) error {
	// 校验组织存在
	_, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}

	// 校验用户是组织成员
	isMember, err := s.orgRepo.IsUserInOrg(ctx, userID, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if !isMember {
		return errors.New(errors.CodeNotOrgMember)
	}

	// 更新 current_org_id
	if err := s.userRepo.UpdateCurrentOrgID(ctx, userID, &orgID); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}
