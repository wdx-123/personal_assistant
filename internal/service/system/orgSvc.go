package system

import (
	"context"
	"net/url"
	"strings"
	"time"

	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	svccontract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/imageops"

	"github.com/gofrs/uuid"
)

// OrgService 组织管理服务
type OrgService struct {
	txRunner                 repository.TxRunner
	orgRepo                  interfaces.OrgRepository
	orgMemberRepo            interfaces.OrgMemberRepository
	userRepo                 interfaces.UserRepository
	roleRepo                 interfaces.RoleRepository
	imageRepo                interfaces.ImageRepository
	authorizationService     svccontract.AuthorizationServiceContract
	permissionProjectionSvc  svccontract.PermissionProjectionServiceContract
	cacheProjectionPublisher cacheProjectionEventPublisher
}

// NewOrgService 创建组织服务实例
func NewOrgService(
	repositoryGroup *repository.Group,
	authorizationService svccontract.AuthorizationServiceContract,
	permissionProjectionSvc svccontract.PermissionProjectionServiceContract,
) *OrgService {
	return &OrgService{
		txRunner:                repositoryGroup,
		orgRepo:                 repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		orgMemberRepo:           repositoryGroup.SystemRepositorySupplier.GetOrgMemberRepository(),
		userRepo:                repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:                repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		imageRepo:               repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
		authorizationService:    authorizationService,
		permissionProjectionSvc: permissionProjectionSvc,
		cacheProjectionPublisher: newCacheProjectionOutboxPublisher(
			repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		),
	}
}

// ==================== 查询 ====================

// GetOrgList 获取组织列表（按当前用户可见范围过滤，支持分页与不分页、关键词搜索）。
// 超级管理员可查看全部组织，普通用户仅可查看自己 active 成员的组织。
func (s *OrgService) GetOrgList(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]*readmodel.OrgWithMemberCount, int64, error) {
	var orgs []*entity.Org
	var total int64

	if s.authorizationService == nil {
		return nil, 0, errors.NewWithMsg(errors.CodeInternalError, "授权服务未初始化")
	}
	isSuperAdmin, err := s.authorizationService.IsSuperAdmin(ctx, userID)
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}

	if isSuperAdmin {
		orgs, total, err = s.orgRepo.GetOrgListWithKeyword(ctx, page, pageSize, keyword)
	} else {
		orgs, total, err = s.orgRepo.GetVisibleOrgListByUserIDWithKeyword(ctx, userID, page, pageSize, keyword)
	}
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}

	items, err := s.buildOrgReadModels(ctx, orgs)
	if err != nil {
		return nil, 0, errors.Wrap(errors.CodeDBError, err)
	}
	return items, total, nil
}

// GetOrgDetail 获取组织详情
// 权限控制：
// 1. 超级管理员：可查看任何组织
// 2. 组织成员（包括 Owner）：仅可查看自己所在的组织
func (s *OrgService) GetOrgDetail(
	ctx context.Context,
	userID uint,
	orgID uint,
) (*readmodel.OrgWithMemberCount, error) {
	if s.authorizationService == nil {
		return nil, errors.NewWithMsg(errors.CodeInternalError, "授权服务未初始化")
	}

	// 1. 检查是否为超级管理员
	isSuperAdmin, err := s.authorizationService.IsSuperAdmin(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
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
	items, err := s.buildOrgReadModels(ctx, []*entity.Org{org})
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if len(items) == 0 {
		return nil, errors.New(errors.CodeOrgNotFound)
	}
	return items[0], nil
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
	var createdOrgID uint
	var assignedOrgAdmin bool
	creator, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if creator == nil {
		return errors.New(errors.CodeUserNotFound)
	}
	oldCurrentOrgID := cloneUintPtr(creator.CurrentOrgID)

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
	// 生成邀请码
	code := strings.TrimSpace(req.Code)
	if code == "" {
		var err error
		code, err = s.generateInviteCode(ctx)
		if err != nil {
			return err
		}
	} else {
		existing, err := s.orgRepo.GetByCode(ctx, code)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if existing != nil {
			return errors.New(errors.CodeOrgAlreadyExists)
		}
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
	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txOrgRepo := s.orgRepo.WithTx(tx)
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txUserRepo := s.userRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		txImageRepo := s.imageRepo.WithTx(tx)

		// 1. 创建组织
		org := &entity.Org{
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			Code:        code,
			Avatar:      avatar,
			AvatarID:    avatarID,
			OwnerID:     userID,
		}
		// 使用 Repository 的事务方法
		if err := txOrgRepo.Create(ctx, org); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		createdOrgID = org.ID

		// 2. 建立组织成员关系（owner 也走成员状态机）
		if err := txOrgMemberRepo.Create(ctx, &entity.OrgMember{
			OrgID:        org.ID,
			UserID:       userID,
			MemberStatus: consts.OrgMemberStatusActive,
			JoinedAt:     time.Now(),
			JoinSource:   string(consts.OrgMemberJoinSourceOrgCreate),
		}); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		// 3. 自动将创建者设为组织管理员
		orgAdminRole, roleErr := txRoleRepo.GetByCode(ctx, consts.RoleCodeOrgAdmin)
		if roleErr == nil && orgAdminRole != nil && orgAdminRole.ID > 0 {
			if err := txRoleRepo.AssignRoleToUserInOrg(ctx, userID, org.ID, orgAdminRole.ID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			assignedOrgAdmin = true
			if s.permissionProjectionSvc != nil {
				if err := s.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, userID, org.ID); err != nil {
					return errors.Wrap(errors.CodeDBError, err)
				}
			}
		}

		// 4. 设置为用户的当前组织
		if err := txUserRepo.UpdateCurrentOrgID(ctx, userID, &org.ID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if avatarID != nil {
			if err := txImageRepo.UpdateCategoryByID(ctx, *avatarID, consts.CatAvatar); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		if err := s.publishCacheProjectionInTx(
			ctx,
			tx,
			newCurrentOrgChangedProjectionEvent(userID, oldCurrentOrgID, &org.ID, []uint{org.ID}),
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		return nil
	}); err != nil {
		return err
	}
	if assignedOrgAdmin && s.permissionProjectionSvc != nil {
		if err := s.permissionProjectionSvc.SyncSubjectRoles(ctx, userID, createdOrgID); err != nil {
			return errors.Wrap(errors.CodeInternalError, err)
		}
	}
	return nil
}

// UpdateOrg 更新组织信息（owner / super_admin / 具备组织管理能力的角色可操作）
// 设计要点：使用事务保证“组织更新 + 头像分类更新 + 旧头像软删除”要么全部成功，要么全部回滚。
func (s *OrgService) UpdateOrg(
	ctx context.Context, // 请求上下文（用于超时/取消/链路追踪）
	userID, orgID uint, // 当前用户ID、目标组织ID
	req *request.UpdateOrgReq, // 更新参数（支持部分更新/可选字段）
) error {
	if err := s.authorizeOrgAction(ctx, userID, orgID, consts.OrgActionUpdate); err != nil {
		return err
	}

	// 开启事务：回调返回 nil -> commit；返回 error -> rollback。
	return s.txRunner.InTx(ctx, func(tx any) error {
		txOrgRepo := s.orgRepo.WithTx(tx)
		txImageRepo := s.imageRepo.WithTx(tx)

		// 读取组织信息（不存在则返回“组织不存在”）。
		org, err := txOrgRepo.GetByID(ctx, orgID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if org == nil {
			return errors.New(errors.CodeOrgNotFound)
		}
		if isAllMembersBuiltinOrg(org) {
			return errors.New(errors.CodeOrgBuiltinProtected)
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
			code := strings.TrimSpace(*req.Code)
			if code == "" {
				return errors.NewWithMsg(errors.CodeInvalidParams, "邀请码不能为空")
			}
			if code != org.Code {
				existing, checkErr := txOrgRepo.GetByCode(ctx, code)
				if checkErr != nil {
					return errors.Wrap(errors.CodeDBError, checkErr)
				}
				if existing != nil && existing.ID != org.ID {
					return errors.New(errors.CodeOrgAlreadyExists)
				}
				org.Code = code
			}
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

// DeleteOrg 删除组织（owner / super_admin / 具备组织管理能力的角色可操作）
// force 为 true 时跳过成员数检查
func (s *OrgService) DeleteOrg(ctx context.Context, userID, orgID uint, force bool) error {
	if err := s.authorizeOrgAction(ctx, userID, orgID, consts.OrgActionDelete); err != nil {
		return err
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if org == nil {
		return errors.New(errors.CodeOrgNotFound)
	}
	if isAllMembersBuiltinOrg(org) {
		return errors.New(errors.CodeOrgBuiltinProtected)
	}
	affectedUserIDs, err := s.userRepo.ListIDsByCurrentOrgID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
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
	return s.txRunner.InTx(ctx, func(tx any) error {
		txOrgRepo := s.orgRepo.WithTx(tx)
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txUserRepo := s.userRepo.WithTx(tx)

		// 1. 删除所有成员关联
		if err := txOrgRepo.RemoveAllMembers(ctx, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		operator := userID
		if err := txOrgMemberRepo.SetAllRemovedByOrg(ctx, orgID, &operator, "组织删除"); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := txUserRepo.ClearCurrentOrgByOrgID(ctx, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		for _, affectedUserID := range affectedUserIDs {
			if err := s.publishCacheProjectionInTx(
				ctx,
				tx,
				newCurrentOrgChangedProjectionEvent(affectedUserID, &orgID, nil, []uint{orgID}),
			); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		// 2. 执行删除组织
		if err := txOrgRepo.Delete(ctx, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	})
}

// SetCurrentOrg 切换用户当前组织
func (s *OrgService) SetCurrentOrg(ctx context.Context, userID, orgID uint) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if user == nil {
		return errors.New(errors.CodeUserNotFound)
	}
	oldCurrentOrgID := cloneUintPtr(user.CurrentOrgID)
	if oldCurrentOrgID != nil && *oldCurrentOrgID == orgID {
		return nil
	}

	// 校验组织存在
	_, err = s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}

	// 校验用户是组织成员
	isMember, err := s.orgMemberRepo.IsUserActiveInOrg(ctx, userID, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if !isMember {
		return errors.New(errors.CodeNotOrgMember)
	}

	// 更新 current_org_id
	return s.txRunner.InTx(ctx, func(tx any) error {
		txUserRepo := s.userRepo.WithTx(tx)
		if err := txUserRepo.UpdateCurrentOrgID(ctx, userID, &orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := s.publishCacheProjectionInTx(
			ctx,
			tx,
			newCurrentOrgChangedProjectionEvent(userID, oldCurrentOrgID, &orgID, []uint{orgID}),
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	})
}

// JoinOrgByInviteCode 用户通过邀请码加入组织。
// 规则：left/removed 可恢复为 active；重新加入仅授予默认角色，不恢复历史角色。
func (s *OrgService) JoinOrgByInviteCode(ctx context.Context, userID uint, inviteCode string) error {
	inviteCode = strings.TrimSpace(inviteCode)
	if inviteCode == "" {
		return errors.New(errors.CodeInvalidParams)
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if user == nil {
		return errors.New(errors.CodeUserNotFound)
	}
	oldCurrentOrgID := cloneUintPtr(user.CurrentOrgID)

	org, err := s.orgRepo.GetByCode(ctx, inviteCode)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if org == nil {
		return errors.New(errors.CodeInviteCodeInvalid)
	}

	shouldSyncSubject := false
	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		txUserRepo := s.userRepo.WithTx(tx)
		shouldResetDefaultRole := false

		member, err := txOrgMemberRepo.GetByOrgAndUserForUpdate(ctx, org.ID, userID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}

		switch {
		case member == nil:
			if err := txOrgMemberRepo.Create(ctx, &entity.OrgMember{
				OrgID:        org.ID,
				UserID:       userID,
				MemberStatus: consts.OrgMemberStatusActive,
				JoinedAt:     time.Now(),
				JoinSource:   string(consts.OrgMemberJoinSourceInvite),
			}); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			shouldResetDefaultRole = true
		case member.MemberStatus == consts.OrgMemberStatusActive:
			// 幂等：已加入
		case member.MemberStatus == consts.OrgMemberStatusLeft || member.MemberStatus == consts.OrgMemberStatusRemoved:
			if err := txOrgMemberRepo.SetStatus(
				ctx,
				org.ID,
				userID,
				consts.OrgMemberStatusActive,
				nil,
				"",
				string(consts.OrgMemberJoinSourceInvite),
			); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			shouldResetDefaultRole = true
		default:
			return errors.New(errors.CodeInvalidParams)
		}

		// left/removed->active 或首次加入时仅保留默认角色，不恢复历史角色。
		if shouldResetDefaultRole {
			if err := txRoleRepo.DeleteUserOrgRoles(ctx, userID, org.ID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			if err := s.assignDefaultOrgRole(ctx, txRoleRepo, userID, org.ID); err != nil {
				return err
			}
			shouldSyncSubject = true
			if s.permissionProjectionSvc != nil {
				if err := s.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, userID, org.ID); err != nil {
					return errors.Wrap(errors.CodeDBError, err)
				}
			}
		}

		if err := txUserRepo.UpdateCurrentOrgID(ctx, userID, &org.ID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := s.publishCacheProjectionInTx(
			ctx,
			tx,
			newCurrentOrgChangedProjectionEvent(userID, oldCurrentOrgID, &org.ID, []uint{org.ID}),
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}
	if shouldSyncSubject && s.permissionProjectionSvc != nil {
		if err := s.permissionProjectionSvc.SyncSubjectRoles(ctx, userID, org.ID); err != nil {
			return errors.Wrap(errors.CodeInternalError, err)
		}
	}
	return nil
}

// LeaveOrg 用户主动退出组织。
func (s *OrgService) LeaveOrg(ctx context.Context, userID, orgID uint, reason string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}
	if org == nil {
		return errors.New(errors.CodeOrgNotFound)
	}
	if isAllMembersBuiltinOrg(org) {
		return errors.New(errors.CodeOrgCannotLeaveBuiltin)
	}
	// Owner 无法退出，必须先转移组织所有权或删除组织。
	if org.OwnerID == userID {
		return errors.New(errors.CodeOrgOwnerTransferRequired)
	}

	// 权限校验：仅组织成员可操作（Owner 也算成员）。
	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		txUserRepo := s.userRepo.WithTx(tx)
		var event *eventdto.CacheProjectionEvent

		member, err := txOrgMemberRepo.GetByOrgAndUserForUpdate(ctx, orgID, userID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if member == nil {
			return errors.New(errors.CodeNotOrgMember)
		}
		if member.MemberStatus == consts.OrgMemberStatusLeft {
			return nil
		}
		if member.MemberStatus == consts.OrgMemberStatusRemoved {
			return errors.New(errors.CodeOrgMemberRemoved)
		}

		if err := txOrgMemberRepo.SetStatus(
			ctx,
			orgID,
			userID,
			consts.OrgMemberStatusLeft,
			nil,
			reason,
			"",
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := txRoleRepo.DeleteUserOrgRoles(ctx, userID, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, userID, orgID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		user, err := txUserRepo.GetByID(ctx, userID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if user != nil && user.CurrentOrgID != nil && *user.CurrentOrgID == orgID {
			oldCurrentOrgID := cloneUintPtr(user.CurrentOrgID)
			nextOrgID, fixErr := s.pickAnotherActiveOrgID(ctx, txOrgMemberRepo, userID, orgID)
			if fixErr != nil {
				return errors.Wrap(errors.CodeDBError, fixErr)
			}
			if err := txUserRepo.UpdateCurrentOrgID(ctx, userID, nextOrgID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			event = newCurrentOrgChangedProjectionEvent(userID, oldCurrentOrgID, nextOrgID, []uint{orgID})
		} else {
			event = newUserSnapshotProjectionEvent(userID, []uint{orgID})
		}

		if err := s.publishCacheProjectionInTx(ctx, tx, event); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}
	if s.permissionProjectionSvc != nil {
		if err := s.permissionProjectionSvc.SyncSubjectRoles(ctx, userID, orgID); err != nil {
			return errors.Wrap(errors.CodeInternalError, err)
		}
	}
	return nil
}

// KickMember 管理员踢出组织成员。
func (s *OrgService) KickMember(ctx context.Context, operatorID, orgID, targetUserID uint, reason string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}
	if org == nil {
		return errors.New(errors.CodeOrgNotFound)
	}
	if err := s.authorizeOrgMemberAction(ctx, operatorID, orgID, consts.OrgMemberActionKick); err != nil {
		return err
	}
	if isAllMembersBuiltinOrg(org) {
		return errors.New(errors.CodeOrgBuiltinProtected)
	}
	if targetUserID == org.OwnerID {
		return errors.New(errors.CodeOrgOwnerTransferRequired)
	}

	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		txUserRepo := s.userRepo.WithTx(tx)
		var event *eventdto.CacheProjectionEvent

		member, err := txOrgMemberRepo.GetByOrgAndUserForUpdate(ctx, orgID, targetUserID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if member == nil {
			return errors.New(errors.CodeNotOrgMember)
		}
		if member.MemberStatus == consts.OrgMemberStatusRemoved {
			return nil
		}

		if err := txOrgMemberRepo.SetStatus(
			ctx,
			orgID,
			targetUserID,
			consts.OrgMemberStatusRemoved,
			&operatorID,
			reason,
			"",
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := txRoleRepo.DeleteUserOrgRoles(ctx, targetUserID, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, targetUserID, orgID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}

		user, err := txUserRepo.GetByID(ctx, targetUserID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if user != nil && user.CurrentOrgID != nil && *user.CurrentOrgID == orgID {
			oldCurrentOrgID := cloneUintPtr(user.CurrentOrgID)
			nextOrgID, fixErr := s.pickAnotherActiveOrgID(ctx, txOrgMemberRepo, targetUserID, orgID)
			if fixErr != nil {
				return errors.Wrap(errors.CodeDBError, fixErr)
			}
			if err := txUserRepo.UpdateCurrentOrgID(ctx, targetUserID, nextOrgID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			event = newCurrentOrgChangedProjectionEvent(targetUserID, oldCurrentOrgID, nextOrgID, []uint{orgID})
		} else {
			event = newUserSnapshotProjectionEvent(targetUserID, []uint{orgID})
		}

		if err := s.publishCacheProjectionInTx(ctx, tx, event); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}
	if s.permissionProjectionSvc != nil {
		if err := s.permissionProjectionSvc.SyncSubjectRoles(ctx, targetUserID, orgID); err != nil {
			return errors.Wrap(errors.CodeInternalError, err)
		}
	}
	return nil
}

// RecoverMember 管理员恢复成员（removed/left -> active），恢复后仅授予默认角色。
func (s *OrgService) RecoverMember(
	ctx context.Context,
	operatorID, orgID, targetUserID uint,
	reason string,
) error {
	// 权限校验：仅 owner 或 org admin 可操作
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return errors.Wrap(errors.CodeOrgNotFound, err)
	}
	if org == nil {
		return errors.New(errors.CodeOrgNotFound)
	}
	if err := s.authorizeOrgMemberAction(ctx, operatorID, orgID, consts.OrgMemberActionRecover); err != nil {
		return err
	}
	// 内置组织禁止修改成员状态（如踢人/恢复成员），
	if isAllMembersBuiltinOrg(org) {
		return errors.New(errors.CodeOrgBuiltinProtected)
	}

	// 开启事务，恢复成员状态，并重置为默认角色（删除原有角色）。
	shouldSyncSubject := false
	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txOrgMemberRepo := s.orgMemberRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		shouldResetDefaultRole := false
		// 获取成员记录（行锁），判断状态并执行相应操作。
		member, err := txOrgMemberRepo.GetByOrgAndUserForUpdate(ctx, orgID, targetUserID)
		if err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		// 如果成员不存在，则创建新的成员记录。
		if member == nil {
			if err := txOrgMemberRepo.Create(ctx, &entity.OrgMember{
				OrgID:        orgID,
				UserID:       targetUserID,
				MemberStatus: consts.OrgMemberStatusActive,
				JoinedAt:     time.Now(),
				JoinSource:   string(consts.OrgMemberJoinSourceAdminRecover),
			}); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			shouldResetDefaultRole = true
		} else if member.MemberStatus != consts.OrgMemberStatusActive {
			if err := txOrgMemberRepo.SetStatus(
				ctx,
				orgID,
				targetUserID,
				consts.OrgMemberStatusActive,
				&operatorID,
				reason,
				string(consts.OrgMemberJoinSourceAdminRecover),
			); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			shouldResetDefaultRole = true
		}
		if !shouldResetDefaultRole {
			// 已是 active，按幂等成功处理，不重置现有角色。
			return nil
		}

		// 恢复成员后重置为默认角色：删除原有角色并分配默认角色（不恢复历史角色）。
		if err := txRoleRepo.DeleteUserOrgRoles(ctx, targetUserID, orgID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := s.assignDefaultOrgRole(ctx, txRoleRepo, targetUserID, orgID); err != nil {
			return err
		}
		shouldSyncSubject = true
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, targetUserID, orgID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		if err := s.publishCacheProjectionInTx(
			ctx,
			tx,
			newUserSnapshotProjectionEvent(targetUserID, []uint{orgID}),
		); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}
	if shouldSyncSubject && s.permissionProjectionSvc != nil {
		if err := s.permissionProjectionSvc.SyncSubjectRoles(ctx, targetUserID, orgID); err != nil {
			return errors.Wrap(errors.CodeInternalError, err)
		}
	}
	return nil
}

// authorizeOrgMemberAction 校验操作者在目标组织下是否具备指定成员动作 capability。
func (s *OrgService) authorizeOrgMemberAction(
	ctx context.Context,
	operatorID, orgID uint,
	action string,
) error {
	capabilityCode, err := capabilityForOrgMemberAction(action)
	if err != nil {
		return err
	}
	if s.authorizationService == nil {
		return errors.NewWithMsg(errors.CodeInternalError, "授权服务未初始化")
	}
	return s.authorizationService.AuthorizeOrgCapability(ctx, operatorID, orgID, capabilityCode)
}

// authorizeOrgAction 校验操作者在目标组织下是否具备指定组织动作 capability。
func (s *OrgService) authorizeOrgAction(
	ctx context.Context,
	operatorID, orgID uint,
	action string,
) error {
	capabilityCode, err := capabilityForOrgAction(action)
	if err != nil {
		return err
	}
	if s.authorizationService == nil {
		return errors.NewWithMsg(errors.CodeInternalError, "授权服务未初始化")
	}
	return s.authorizationService.AuthorizeOrgCapability(ctx, operatorID, orgID, capabilityCode)
}

func capabilityForOrgMemberAction(action string) (string, error) {
	switch action {
	case consts.OrgMemberActionKick:
		return consts.CapabilityCodeOrgMemberKick, nil
	case consts.OrgMemberActionRecover:
		return consts.CapabilityCodeOrgMemberRecover, nil
	case consts.OrgMemberActionFreeze:
		return consts.CapabilityCodeOrgMemberFreeze, nil
	case consts.OrgMemberActionDelete:
		return consts.CapabilityCodeOrgMemberDelete, nil
	case consts.OrgMemberActionInvite:
		return consts.CapabilityCodeOrgMemberInvite, nil
	case consts.OrgMemberActionAssignRole:
		return consts.CapabilityCodeOrgMemberAssignRole, nil
	default:
		return "", errors.NewWithMsg(errors.CodeInvalidParams, "不支持的成员操作动作")
	}
}

func capabilityForOrgAction(action string) (string, error) {
	switch action {
	case consts.OrgActionUpdate:
		return consts.CapabilityCodeOrgManageUpdate, nil
	case consts.OrgActionDelete:
		return consts.CapabilityCodeOrgManageDelete, nil
	default:
		return "", errors.NewWithMsg(errors.CodeInvalidParams, "不支持的组织操作动作")
	}
}

// pickAnotherActiveOrgID 在用户离开/被踢出组织后，
// 尝试为用户切换到另一个活跃的组织（如果有），返回新组织ID；
// 如果没有其他活跃组织，则返回 nil。
func (s *OrgService) pickAnotherActiveOrgID(
	ctx context.Context,
	repo interfaces.OrgMemberRepository,
	userID uint,
	excludeOrgID uint,
) (*uint, error) {
	orgIDs, err := repo.ListActiveOrgIDsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, id := range orgIDs {
		if id != excludeOrgID {
			next := id
			return &next, nil
		}
	}
	return nil, nil
}

// assignDefaultOrgRole 给用户分配组织内的默认角色（如首次加入/恢复组织时）。
func (s *OrgService) assignDefaultOrgRole(
	ctx context.Context,
	roleRepo interfaces.RoleRepository,
	userID, orgID uint,
) error {
	defaultRole, err := resolveDefaultOrgRole(ctx, roleRepo)
	if err != nil {
		return err
	}
	if err := roleRepo.AssignRoleToUserInOrg(ctx, userID, orgID, defaultRole.ID); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

func (s *OrgService) publishCacheProjectionInTx(
	ctx context.Context,
	tx any,
	event *eventdto.CacheProjectionEvent,
) error {
	if event == nil || s.cacheProjectionPublisher == nil {
		return nil
	}
	return s.cacheProjectionPublisher.PublishInTx(ctx, tx, event)
}

// generateInviteCode 生成唯一的邀请码，格式为 "ORG-" + 10位随机大写字母数字组合。
func (s *OrgService) generateInviteCode(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		raw := strings.ToUpper(strings.ReplaceAll(uuid.Must(uuid.NewV4()).String(), "-", ""))
		code := "ORG-" + raw[:10]
		exists, err := s.orgRepo.GetByCode(ctx, code)
		if err != nil {
			return "", errors.Wrap(errors.CodeDBError, err)
		}
		if exists == nil {
			return code, nil
		}
	}
	return "", errors.NewWithMsg(errors.CodeInternalError, "生成邀请码失败")
}

// isAllMembersBuiltinOrg 判断组织是否为“全员组织”，
// 即系统内置、不可修改、所有用户默认加入的特殊组织。
func isAllMembersBuiltinOrg(org *entity.Org) bool {
	if org == nil {
		return false
	}
	return org.IsBuiltin && org.BuiltinKey != nil && *org.BuiltinKey == consts.OrgBuiltinKeyAllMembers
}

func (s *OrgService) buildOrgReadModels(
	ctx context.Context,
	orgs []*entity.Org,
) ([]*readmodel.OrgWithMemberCount, error) {
	if len(orgs) == 0 {
		return make([]*readmodel.OrgWithMemberCount, 0), nil
	}

	orgIDs := make([]uint, 0, len(orgs))
	for _, org := range orgs {
		if org == nil {
			continue
		}
		orgIDs = append(orgIDs, org.ID)
	}

	counts, err := s.orgMemberRepo.CountActiveMembersByOrgIDs(ctx, orgIDs)
	if err != nil {
		return nil, err
	}

	items := make([]*readmodel.OrgWithMemberCount, 0, len(orgs))
	for _, org := range orgs {
		if org == nil {
			continue
		}
		items = append(items, &readmodel.OrgWithMemberCount{
			ID:          org.ID,
			Name:        org.Name,
			Description: org.Description,
			Code:        org.Code,
			Avatar:      org.Avatar,
			AvatarID:    org.AvatarID,
			OwnerID:     org.OwnerID,
			MemberCount: counts[org.ID],
			CreatedAt:   org.CreatedAt,
			UpdatedAt:   org.UpdatedAt,
		})
	}
	return items, nil
}
