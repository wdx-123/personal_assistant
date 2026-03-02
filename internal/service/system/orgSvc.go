package system

import (
	"context"
	"net/url"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/imageops"

	"gorm.io/gorm"
)

// OrgService 组织管理服务
type OrgService struct {
	orgRepo   interfaces.OrgRepository
	userRepo  interfaces.UserRepository
	roleRepo  interfaces.RoleRepository
	imageRepo interfaces.ImageRepository
}

// NewOrgService 创建组织服务实例
func NewOrgService(repositoryGroup *repository.Group) *OrgService {
	return &OrgService{
		orgRepo:   repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		userRepo:  repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:  repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		imageRepo: repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
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
	avatar := strings.TrimSpace(req.Avatar)
	if avatar != "" {
		if err := validateAvatarURL(avatar); err != nil {
			return err
		}
	}
	var avatarID *uint
	if avatar != "" && req.AvatarID != nil && *req.AvatarID > 0 {
		id := *req.AvatarID
		avatarID = &id
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
			Avatar:      avatar,
			AvatarID:    avatarID,
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
		if avatarID != nil {
			if err := s.imageRepo.WithTx(tx).UpdateCategoryByID(ctx, *avatarID, consts.CatAvatar); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		return nil
	})
}

// UpdateOrg 更新组织信息（仅组织所有者可操作）
// 设计要点：使用事务保证“组织更新 + 头像分类更新 + 旧头像软删除”要么全部成功，要么全部回滚。
func (s *OrgService) UpdateOrg(
	ctx context.Context, // 请求上下文（用于超时/取消/链路追踪）
	userID, orgID uint, // 当前用户ID、目标组织ID
	req *request.UpdateOrgReq, // 更新参数（支持部分更新/可选字段）
) error {
	// 开启事务：回调返回 nil -> commit；返回 error -> rollback。
	return global.DB.Transaction(func(tx *gorm.DB) error {
		// 使用同一事务句柄创建仓储实例，确保所有写操作落在同一事务中。
		txOrgRepo := s.orgRepo.WithTx(tx)
		txImageRepo := s.imageRepo.WithTx(tx)

		// 读取组织信息（不存在则返回“组织不存在”）。
		org, err := txOrgRepo.GetByID(ctx, orgID)
		if err != nil {
			return errors.Wrap(errors.CodeOrgNotFound, err)
		}

		// 权限校验：仅组织 owner 可更新。
		if org.OwnerID != userID {
			return errors.New(errors.CodeOrgOwnerOnly)
		}

		// 记录旧头像ID（后续若更换头像，用于软删除旧头像资源）。
		oldAvatarID := uint(0)
		if org.AvatarID != nil {
			oldAvatarID = *org.AvatarID
		}

		// 解析并校验头像参数对（avatar 与 avatar_id 必须同时传入；支持清空/设置两种语义）。
		avatarPair, err := parseOrgAvatarPair(req.Avatar, req.AvatarID)
		if err != nil {
			return err
		}

		// ---- 部分更新（仅更新客户端明确传入的字段） ----

		// 更新 Name：去空格；非空且变更时校验唯一性。
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name != "" && name != org.Name {
				// 校验新名称是否已存在（避免重名）。
				exists, checkErr := txOrgRepo.ExistsByName(ctx, name)
				if checkErr != nil {
					return errors.Wrap(errors.CodeDBError, checkErr)
				}
				if exists {
					return errors.New(errors.CodeOrgNameDuplicate)
				}
				org.Name = name
			}
		}

		// 更新 Description：允许为空字符串（表示清空）；仅在字段被提供时才更新。
		if req.Description != nil {
			org.Description = strings.TrimSpace(*req.Description)
		}

		// 更新 Code：允许为空字符串（表示清空）；仅在字段被提供时才更新。
		if req.Code != nil {
			org.Code = strings.TrimSpace(*req.Code)
		}

		// 更新 Avatar/AvatarID：仅在客户端“提供了头像字段对”时才更新（避免误清空）。
		if avatarPair.Provided {
			org.Avatar = avatarPair.Avatar
			org.AvatarID = avatarPair.AvatarID
		}

		// 落库更新组织信息（事务内）。
		if err := txOrgRepo.Update(ctx, org); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		// 若本次请求未包含头像字段对，则不触发头像分类/旧头像清理逻辑，直接提交事务。
		if !avatarPair.Provided {
			return nil
		}

		// 计算新头像ID（可能为 0：表示清空头像）。
		newAvatarID := uint(0)
		if org.AvatarID != nil {
			newAvatarID = *org.AvatarID
			// 将新头像图片标记为头像分类（用于后续管理/策略）。
			if err := txImageRepo.UpdateCategoryByID(ctx, newAvatarID, consts.CatAvatar); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		// 若旧头像存在且与新头像不同，则软删除旧头像图片记录（避免垃圾资源堆积）。
		if oldAvatarID > 0 && oldAvatarID != newAvatarID {
			if _, err := imageops.SoftDeleteByIDs(ctx, txImageRepo, []uint{oldAvatarID}); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		// 回调返回 nil 表示事务提交。
		return nil
	})
}

// orgAvatarPair 表达一次头像参数解析后的语义结果：是否提供、最终 avatar 字符串、最终 avatarID（可空）。
type orgAvatarPair struct {
	Provided bool   // 是否显式提供了头像字段对（用于区分“未修改头像”和“清空头像/设置头像”）
	Avatar   string // 头像URL（空串表示清空）
	AvatarID *uint  // 头像图片ID（nil 表示无头像）
}

// parseOrgAvatarPair 解析并校验 avatar 与 avatar_id 的组合语义：
// - 两者都不传：Provided=false（不修改头像）
// - 两者必须同时传：否则参数错误
// - avatar 为空串：表示清空头像，此时 avatar_id 必须为 0
// - avatar 非空：表示设置头像，此时 avatar_id 必须 > 0，且 avatar 必须是合法 http(s) URL
func parseOrgAvatarPair(avatar *string, avatarID *uint) (orgAvatarPair, error) {
	// 两者都未提供：不修改头像。
	if avatar == nil && avatarID == nil {
		return orgAvatarPair{Provided: false}, nil
	}

	// 只提供其一：拒绝（避免头像URL与图片ID失配）。
	if avatar == nil || avatarID == nil {
		return orgAvatarPair{}, errors.NewWithMsg(errors.CodeInvalidParams, "avatar 与 avatar_id 必须同时传入")
	}

	// 清洗 avatar（去掉两端空格）。
	trimmedAvatar := strings.TrimSpace(*avatar)

	switch {
	// avatar 为空：语义为“清空头像”。
	case trimmedAvatar == "":
		// 清空头像时 avatar_id 必须为 0（避免传入无效/脏ID）。
		if *avatarID != 0 {
			return orgAvatarPair{}, errors.NewWithMsg(errors.CodeInvalidParams, "清空头像时 avatar_id 必须为 0")
		}
		// 返回清空结果：avatar 置空，avatarID 置 nil。
		return orgAvatarPair{
			Provided: true,
			Avatar:   "",
			AvatarID: nil,
		}, nil

	// avatar 非空：语义为“设置头像”。
	default:
		// 设置头像时 avatar_id 必须 > 0（需要绑定到有效图片记录）。
		if *avatarID == 0 {
			return orgAvatarPair{}, errors.NewWithMsg(errors.CodeInvalidParams, "设置头像时 avatar_id 必须大于 0")
		}
		// 校验头像URL必须为合法 http(s) URL。
		if err := validateAvatarURL(trimmedAvatar); err != nil {
			return orgAvatarPair{}, err
		}
		// 复制一份ID到局部变量，返回其地址（避免直接复用外部指针带来的可变性/歧义）。
		id := *avatarID
		return orgAvatarPair{
			Provided: true,
			Avatar:   trimmedAvatar,
			AvatarID: &id,
		}, nil
	}
}

// validateAvatarURL 校验头像URL：必须是合法URI、协议为 http/https、且 Host 非空。
func validateAvatarURL(rawURL string) error {
	// 解析并校验 URI 格式（更严格，拒绝相对路径等不符合请求URI的形式）。
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.NewWithMsg(errors.CodeInvalidParams, "头像URL格式不合法")
	}
	// 限制协议，禁止 file:// 等潜在风险协议。
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.NewWithMsg(errors.CodeInvalidParams, "头像URL仅支持http或https")
	}
	// Host 必须存在（保证是完整的网络地址）。
	if parsed.Host == "" {
		return errors.NewWithMsg(errors.CodeInvalidParams, "头像URL格式不合法")
	}
	return nil // 校验通过
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
