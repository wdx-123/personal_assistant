package system

import (
	"context"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"

	"gorm.io/gorm"
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
// 权限控制：
// 1. 超级管理员：可查看任何组织
// 2. 组织成员（包括 Owner）：仅可查看自己所在的组织
func (s *OrgService) GetOrgDetail(
	ctx context.Context,
	userID uint,
	orgID uint,
) (*entity.Org, error) {
	// 1. 检查是否为超级管理员
	isSuperAdmin := false
	globalRoles, _ := s.roleRepo.GetUserGlobalRoles(ctx, userID)
	for _, role := range globalRoles {
		if role.Code == consts.RoleCodeSuperAdmin {
			isSuperAdmin = true
			break
		}
	}

	// 2. 如果不是超管，检查是否为组织成员
	if !isSuperAdmin {
		isMember, err := s.orgRepo.IsUserInOrg(ctx, userID, orgID)
		if err != nil {
			return nil, errors.Wrap(errors.CodeDBError, err)
		}
		if !isMember {
			return nil, errors.NewWithMsg(errors.CodePermissionDenied, "无权查看该组织信息")
		}
	}

	// 3. 获取详情
	org, err := s.orgRepo.GetByID(ctx, orgID)
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

	// 开启事务
	return global.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 创建组织
		org := &entity.Org{
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			Code:        strings.TrimSpace(req.Code),
			OwnerID:     userID,
		}
		// 使用 Repository 的事务方法
		if err := s.orgRepo.WithTx(tx).Create(ctx, org); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		// 2. 自动将创建者设为组织管理员
		orgAdminRole, roleErr := s.roleRepo.GetByCode(ctx, consts.RoleCodeOrgAdmin)
		if roleErr == nil && orgAdminRole != nil && orgAdminRole.ID > 0 {
			// 使用 Repository 的事务方法
			if err := s.roleRepo.WithTx(tx).AssignRoleToUserInOrg(ctx, userID, org.ID, orgAdminRole.ID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		// 3. 设置为用户的当前组织
		if err := s.userRepo.WithTx(tx).UpdateCurrentOrgID(ctx, userID, &org.ID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		return nil
	})
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

	// 非强制删除时，检查组织下是否还有成员（不包括 Owner 自己）
	if !force {
		memberCount, err := s.orgRepo.CountMembersByOrgID(ctx, orgID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		// 如果成员数 > 1（除了 Owner 还有别人），则不允许删除
		if memberCount > 1 {
			return errors.New(errors.CodeOrgHasMembers)
		}
	}

	// 开启事务
	return global.DB.Transaction(func(tx *gorm.DB) error {
		// 1. 删除所有成员关联
		if err := tx.WithContext(ctx).Exec("DELETE FROM user_org_roles WHERE org_id = ?", orgID).Error; err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		// 2. 执行删除组织
		if err := tx.WithContext(ctx).Delete(&entity.Org{}, orgID).Error; err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	})
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
