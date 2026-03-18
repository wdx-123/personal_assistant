package system

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/imageops"

	"github.com/gofrs/uuid"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	svccontract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/util"
)

type UserService struct {
	txRunner                 repository.TxRunner
	userRepo                 interfaces.UserRepository // 依赖接口而不是具体实现
	roleRepo                 interfaces.RoleRepository // 角色仓储，用于获取默认角色
	orgRepo                  interfaces.OrgRepository
	orgMemberRepo            interfaces.OrgMemberRepository
	imageRepo                interfaces.ImageRepository // 图片仓储
	authorizationService     svccontract.AuthorizationServiceContract
	permissionProjectionSvc  svccontract.PermissionProjectionServiceContract
	cacheProjectionPublisher cacheProjectionEventPublisher
}

func NewUserService(
	repositoryGroup *repository.Group,
	authorizationService svccontract.AuthorizationServiceContract,
	permissionProjectionSvc svccontract.PermissionProjectionServiceContract,
) *UserService {
	return &UserService{
		txRunner:                repositoryGroup,
		userRepo:                repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:                repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		orgRepo:                 repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		orgMemberRepo:           repositoryGroup.SystemRepositorySupplier.GetOrgMemberRepository(),
		imageRepo:               repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
		authorizationService:    authorizationService,
		permissionProjectionSvc: permissionProjectionSvc,
		cacheProjectionPublisher: newCacheProjectionOutboxPublisher(
			repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		),
	}
}

type userRoleMatrixLevel string

const (
	userRoleMatrixLevelSuperAdmin userRoleMatrixLevel = "super_admin"
	userRoleMatrixLevelOrgAdmin   userRoleMatrixLevel = "org_admin"
	userRoleMatrixLevelMember     userRoleMatrixLevel = "member"
)

const (
	userRoleMatrixDisabledReasonGlobalRoleOnly    = "global_role_only"    // 全局角色专用，无法分配组织角色
	userRoleMatrixDisabledReasonHigherMatrixLevel = "higher_matrix_level" // 存在更高矩阵级别角色
)

type userRoleMatrixBuildResult struct {
	response      *resp.UserRoleMatrixItem
	roleItemsByID map[uint]resp.UserRoleMatrixRoleItem
}

// Register 注册
func (u *UserService) Register(
	ctx context.Context,
	req *request.RegisterReq,
) (*entity.User, error) {
	//// 1. 验证图片验证码
	//if !base64Captcha.DefaultMemStore.Verify(req.CaptchaID, req.Captcha, true) {
	//	return ni  ml, errors.New("验证码错误")
	//}

	// 2. 检查手机号是否已存在
	exists, err := u.userRepo.ExistsByPhone(ctx, req.Phone)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if exists {
		return nil, bizerrors.New(bizerrors.CodePhoneAlreadyUsed)
	}
	// 邀请码必须有效且对应一个组织，且该组织必须是启用状态。
	inviteCode := strings.TrimSpace(req.InviteCode)
	if inviteCode == "" {
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}
	// 查询邀请码对应的组织，验证组织状态。
	targetOrg, err := u.orgRepo.GetByCode(ctx, inviteCode)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if targetOrg == nil {
		return nil, bizerrors.New(bizerrors.CodeInviteCodeInvalid)
	}

	// 获取系统内置的全体成员组织，确保用户注册后能自动加入（如果邀请码组织不是全体成员组织）。
	allMembersOrg, err := u.orgRepo.GetByBuiltinKey(ctx, consts.OrgBuiltinKeyAllMembers)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if allMembersOrg == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeOrgNotFound, "系统内置组织不存在")
	}

	// 获取默认角色：优先使用系统配置的默认角色 code，未命中时回退到内置 member 角色。
	defaultRole, err := resolveDefaultOrgRole(ctx, u.roleRepo)
	if err != nil {
		return nil, err
	}

	// 3. 创建用户实例（不直接设置RoleID，通过权限服务分配）
	user := &entity.User{
		Username: req.Username,
		Password: util.BcryptHash(req.Password),
		Phone:    req.Phone,
		UUID:     uuid.Must(uuid.NewV4()),
		Avatar:   "", // 默认头像为空
		Register: consts.Email,
		Status:   consts.UserStatusActive,
		// 不直接设置 RoleID，将通过权限服务分配角色
	}
	targetOrgID := targetOrg.ID
	user.CurrentOrgID = &targetOrgID

	// 4. 开启事务，创建用户、成员记录，并分配默认角色
	err = u.txRunner.InTx(ctx, func(tx any) error {
		txUserRepo := u.userRepo.WithTx(tx)
		txOrgMemberRepo := u.orgMemberRepo.WithTx(tx)
		txRoleRepo := u.roleRepo.WithTx(tx)

		// 创建用户
		if err := txUserRepo.Create(ctx, user); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		now := time.Now()
		// 创建成员记录，加入邀请码组织和全体成员组织（如果不同）。默认状态为 active，来源分别标记为 Register 和 SystemBackfill。
		if err := txOrgMemberRepo.Create(ctx, &entity.OrgMember{
			OrgID:        targetOrg.ID,
			UserID:       user.ID,
			MemberStatus: consts.OrgMemberStatusActive,
			JoinedAt:     now,
			JoinSource:   string(consts.OrgMemberJoinSourceRegister),
		}); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		// 如果邀请码组织不是全体成员组织，则也加入全体成员组织（如默认切换到全体成员组织，获取公告等）。
		if allMembersOrg.ID != targetOrg.ID {
			if err := txOrgMemberRepo.Create(ctx, &entity.OrgMember{
				OrgID:        allMembersOrg.ID,
				UserID:       user.ID,
				MemberStatus: consts.OrgMemberStatusActive,
				JoinedAt:     now,
				JoinSource:   string(consts.OrgMemberJoinSourceSystemBackfill),
			}); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}

		// 邀请码组织 + 全体成员组织均授予默认角色。
		if err := txRoleRepo.AssignRoleToUserInOrg(ctx, user.ID, targetOrg.ID, defaultRole.ID); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		// 如果邀请码组织不是全体成员组织，则全体成员组织也授予默认角色。
		if allMembersOrg.ID != targetOrg.ID {
			if err := txRoleRepo.AssignRoleToUserInOrg(ctx, user.ID, allMembersOrg.ID, defaultRole.ID); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		if u.permissionProjectionSvc != nil {
			if err := u.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, user.ID, targetOrg.ID); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
			if allMembersOrg.ID != targetOrg.ID {
				if err := u.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, user.ID, allMembersOrg.ID); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
			}
		}

		// 发布用户快照变更事件，触发缓存投影更新。包含用户ID和受影响的组织ID列表（邀请码组织和全体成员组织）。
		if err := u.publishCacheProjectionInTx(ctx, tx, newUserSnapshotProjectionEvent(user.ID, nil)); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if u.permissionProjectionSvc != nil {
		if err := u.permissionProjectionSvc.SyncSubjectRoles(ctx, user.ID, targetOrg.ID); err != nil {
			global.Log.Error("注册后同步目标组织角色投影失败", zap.Error(err))
		}
		if allMembersOrg.ID != targetOrg.ID {
			if err := u.permissionProjectionSvc.SyncSubjectRoles(ctx, user.ID, allMembersOrg.ID); err != nil {
				global.Log.Error("注册后同步全体成员角色投影失败", zap.Error(err))
			}
		}
	}

	// 6. 重新查询用户，确保返回包含关联数据（如CurrentOrg）的完整对象
	// 这对于后续直接生成 Token 并返回完整信息至关重要
	fullUser, err := u.userRepo.GetByID(ctx, user.ID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return fullUser, nil
}

// PhoneLogin 手机号登录
func (u *UserService) PhoneLogin(
	ctx context.Context,
	req *request.LoginReq,
) (*entity.User, error) {
	// 1. 验证图片验证码
	if !base64Captcha.DefaultMemStore.Verify(req.CaptchaID, req.Captcha, true) {
		return nil, errors.New("验证码错误")
	}

	// 2. 根据手机号获取用户
	user, err := u.userRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		global.Log.Error("根据手机号查询用户失败",
			zap.String("phone", req.Phone),
			zap.Error(err))
		return nil, errors.New("账号或密码错误") // 模糊提示
	}
	if user == nil {
		return nil, errors.New("账号或密码错误")
	}

	// 3. 验证密码
	if !util.BcryptCheck(req.Password, user.Password) {
		return nil, errors.New("账号或密码错误")
	}

	// 4. 检查是否冻结
	if user.Freeze || user.Status != consts.UserStatusActive {
		return nil, bizerrors.New(bizerrors.CodeUserDisabled)
	}

	return user, nil
}

// VerifyCode 校验验证码是否正确
func (u *UserService) VerifyCode(
	store base64Captcha.Store,
	req request.LoginReq,
) bool {
	return store.Verify(req.CaptchaID, req.Captcha, true)
}

// UpdateProfile 更新个人资料
func (u *UserService) UpdateProfile(
	ctx context.Context,
	userID uint,
	req *request.UpdateProfileReq,
) (*entity.User, error) {
	var user *entity.User
	err := u.txRunner.InTx(ctx, func(tx any) error {
		txUserRepo := u.userRepo.WithTx(tx)
		txImageRepo := u.imageRepo.WithTx(tx)

		// 1. 获取用户
		var getErr error
		user, getErr = txUserRepo.GetByID(ctx, userID)
		if getErr != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, getErr)
		}
		if user == nil {
			return bizerrors.New(bizerrors.CodeUserNotFound)
		}

		avatarPatch, parseErr := parseUserAvatarPatch(req.Avatar, req.AvatarID)
		if parseErr != nil {
			return parseErr
		}

		oldAvatarID := uint(0)
		if user.AvatarID != nil {
			oldAvatarID = *user.AvatarID
		}

		updated := false
		projectionChanged := false
		// 更新用户名
		if req.Username != nil && *req.Username != "" {
			user.Username = *req.Username
			updated = true
			projectionChanged = true
		}
		// 更新签名
		if req.Signature != nil {
			user.Signature = *req.Signature
			updated = true
		}
		// 更新头像（avatar 与 avatar_id 成对变更）
		if avatarPatch.Provided {
			user.Avatar = avatarPatch.Avatar
			user.AvatarID = avatarPatch.AvatarID
			updated = true
			projectionChanged = true
		}
		if !updated {
			return nil
		}

		// 2. 保存用户信息
		if err := txUserRepo.Update(ctx, user); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		if !avatarPatch.Provided {
			return nil
		}

		newAvatarID := uint(0)
		if user.AvatarID != nil {
			newAvatarID = *user.AvatarID
			if err := txImageRepo.UpdateCategoryByID(ctx, newAvatarID, consts.CatAvatar); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		if oldAvatarID > 0 && oldAvatarID != newAvatarID {
			if _, err := imageops.SoftDeleteByIDs(ctx, txImageRepo, []uint{oldAvatarID}); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		if projectionChanged {
			if err := u.publishCacheProjectionInTx(ctx, tx, newUserSnapshotProjectionEvent(userID, nil)); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return user, nil
}

type userAvatarPatch struct {
	Provided bool
	Avatar   string
	AvatarID *uint
}

func parseUserAvatarPatch(avatar *string, avatarID *uint) (userAvatarPatch, error) {
	if avatar == nil && avatarID == nil {
		return userAvatarPatch{Provided: false}, nil
	}
	if avatar == nil || avatarID == nil {
		return userAvatarPatch{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "avatar 与 avatar_id 必须同时传入")
	}

	trimmedAvatar := strings.TrimSpace(*avatar)
	switch {
	case trimmedAvatar == "":
		if *avatarID != 0 {
			return userAvatarPatch{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "清空头像时 avatar_id 必须为 0")
		}
		return userAvatarPatch{
			Provided: true,
			Avatar:   "",
			AvatarID: nil,
		}, nil
	default:
		if *avatarID == 0 {
			return userAvatarPatch{}, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "设置头像时 avatar_id 必须大于 0")
		}
		if err := validateUserAvatarURL(trimmedAvatar); err != nil {
			return userAvatarPatch{}, err
		}
		id := *avatarID
		return userAvatarPatch{
			Provided: true,
			Avatar:   trimmedAvatar,
			AvatarID: &id,
		}, nil
	}
}

func validateUserAvatarURL(rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "头像URL格式不合法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "头像URL仅支持http或https")
	}
	if parsed.Host == "" {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "头像URL格式不合法")
	}
	return nil
}

// ChangePhone 换绑手机号
func (u *UserService) ChangePhone(
	ctx context.Context,
	userID uint,
	req *request.ChangePhoneReq,
) (*entity.User, error) {
	// 1. 验证验证码
	if !base64Captcha.DefaultMemStore.Verify(req.CaptchaID, req.Captcha, true) {
		return nil, errors.New("验证码错误")
	}

	// 2. 获取用户
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("用户不存在")
	}

	// 3. 验证当前密码
	if !util.BcryptCheck(req.Password, user.Password) {
		return nil, errors.New("密码错误")
	}

	// 4. 检查新手机号是否已存在
	exists, err := u.userRepo.ExistsByPhone(ctx, req.NewPhone)
	if err != nil {
		return nil, errors.New("系统错误")
	}
	if exists {
		return nil, errors.New("该手机号已被注册")
	}

	// 5. 更新手机号
	user.Phone = req.NewPhone
	if err := u.userRepo.Update(ctx, user); err != nil {
		global.Log.Error("更新手机号失败", zap.Uint("userID", userID), zap.Error(err))
		return nil, errors.New("更新失败")
	}

	return user, nil
}

// ChangePassword 修改密码
func (u *UserService) ChangePassword(
	ctx context.Context,
	userID uint,
	req *request.ChangePasswordReq,
) error {
	// 1. 获取用户
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("用户不存在")
	}

	// 2. 验证旧密码
	if !util.BcryptCheck(req.OldPassword, user.Password) {
		return errors.New("旧密码错误")
	}

	// 3. 更新密码
	user.Password = util.BcryptHash(req.NewPassword)
	if err := u.userRepo.Update(ctx, user); err != nil {
		global.Log.Error("修改密码失败", zap.Uint("userID", userID), zap.Error(err))
		return errors.New("修改失败")
	}

	return nil
}

// GetUserList 获取用户列表
func (u *UserService) GetUserList(
	ctx context.Context,
	req *request.UserListReq,
) (*resp.PageDataUser, error) {
	// 1. 查询用户列表
	users, total, err := u.userRepo.GetUserListWithFilter(ctx, req)
	if err != nil {
		global.Log.Error("获取用户列表失败", zap.Error(err))
		return nil, errors.New("获取用户列表失败")
	}

	// 2. 组装数据（填充角色信息）
	list := make([]*resp.UserListItem, 0, len(users))
	for _, user := range users {
		item := &resp.UserListItem{
			ID:           user.ID,
			Username:     user.Username,
			Phone:        util.DesensitizePhone(user.Phone),
			CurrentOrgID: user.CurrentOrgID,
		}
		if user.CurrentOrg != nil {
			item.CurrentOrg = &resp.OrgSimpleItem{
				ID:   user.CurrentOrg.ID,
				Name: user.CurrentOrg.Name,
			}
		}

		// 获取用户在该上下文下的角色
		// 如果请求指定了 OrgID，则查询该 Org 下的角色
		// 否则查询用户 CurrentOrg 下的角色
		var targetOrgID uint
		if req.OrgID > 0 {
			targetOrgID = req.OrgID
		} else if user.CurrentOrgID != nil {
			targetOrgID = *user.CurrentOrgID
		}

		if targetOrgID > 0 {
			roles, err := u.roleRepo.GetUserRolesByOrg(ctx, user.ID, targetOrgID)
			if err == nil {
				item.Roles = make([]struct {
					ID   uint   `json:"id"`
					Name string `json:"name"`
				}, len(roles))
				for i, r := range roles {
					item.Roles[i].ID = r.ID
					item.Roles[i].Name = r.Name
				}
			}
		}

		list = append(list, item)
	}

	return &resp.PageDataUser{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetUserDetail 获取用户详情
func (u *UserService) GetUserDetail(
	ctx context.Context,
	id uint,
) (*entity.User, error) {
	user, err := u.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserRoles 获取用户在指定组织下的角色
func (u *UserService) GetUserRoles(
	ctx context.Context,
	userID, orgID uint,
) ([]*entity.Role, error) {
	return u.roleRepo.GetUserRolesByOrg(ctx, userID, orgID)
}

// GetUserRoleMatrix 获取用户角色分配矩阵
func (u *UserService) GetUserRoleMatrix(
	ctx context.Context,
	operatorID, targetUserID, orgID uint,
) (*resp.UserRoleMatrixItem, error) {
	result, err := u.buildUserRoleMatrix(ctx, operatorID, targetUserID, orgID)
	if err != nil {
		return nil, err
	}
	return result.response, nil
}

// AssignRole 分配角色
func (u *UserService) AssignRole(
	ctx context.Context,
	operatorID uint,
	req *request.AssignUserRoleReq,
) error {
	if req == nil || req.UserID == 0 || req.OrgID == 0 {
		return bizerrors.New(bizerrors.CodeInvalidParams)
	}

	matrix, err := u.buildUserRoleMatrix(ctx, operatorID, req.UserID, req.OrgID)
	if err != nil {
		return err
	}

	validRoleIDs := make([]uint, 0, len(req.RoleIDs))
	for _, roleID := range req.RoleIDs {
		roleItem, ok := matrix.roleItemsByID[roleID]
		if !ok {
			return bizerrors.New(bizerrors.CodeRoleNotFound)
		}
		if !roleItem.Assignable {
			return bizerrors.NewWithMsg(bizerrors.CodePermissionDenied, "无权分配所选角色")
		}
		validRoleIDs = append(validRoleIDs, roleID)
	}
	validRoleIDs = normalizeUserRoleIDs(validRoleIDs)

	if err := u.txRunner.InTx(ctx, func(tx any) error {
		txRoleRepo := u.roleRepo.WithTx(tx)
		if err := txRoleRepo.ReplaceUserOrgRoles(ctx, req.UserID, req.OrgID, validRoleIDs); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if u.permissionProjectionSvc != nil {
			if err := u.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, req.UserID, req.OrgID); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if u.permissionProjectionSvc != nil {
		if err := u.permissionProjectionSvc.SyncSubjectRoles(ctx, req.UserID, req.OrgID); err != nil {
			return bizerrors.Wrap(bizerrors.CodeInternalError, err)
		}
	}
	return nil
}

// buildUserRoleMatrix 构建用户角色矩阵，包含以下步骤：
// 1. 验证输入参数有效性
// 2. 获取目标用户信息，验证用户存在
// 3. 验证操作者对目标组织具有分配角色的权限
// 4. 获取目标用户在组织中的成员状态，验证为 active
// 5. 决定操作者在用户角色矩阵中的级别（超级管理员 > 组织管理员 > 成员）
// 6. 获取目标用户在组织中已分配的角色列表
// 7. 获取系统中所有启用的角色列表，并根据预定义规则排序（如超级管理员角色始终靠前）
// 8. 构建角色矩阵项列表，标记每个角色是否已分配给目标用户，以及是否可分配（基于操作者级别和角色级别的比较）
// 9. 返回构建结果，包括角色矩阵数据和辅助映射（如角色ID到矩阵项的映射）以供后续使用
func (u *UserService) buildUserRoleMatrix(
	ctx context.Context,
	operatorID, targetUserID, orgID uint,
) (*userRoleMatrixBuildResult, error) {
	if operatorID == 0 || targetUserID == 0 || orgID == 0 {
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}

	targetUser, err := u.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if targetUser == nil {
		return nil, bizerrors.New(bizerrors.CodeUserNotFound)
	}

	if u.authorizationService == nil {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "授权服务未初始化")
	}
	if err := u.authorizationService.AuthorizeOrgCapability(
		ctx,
		operatorID,
		orgID,
		consts.CapabilityCodeOrgMemberAssignRole,
	); err != nil {
		return nil, err
	}

	active, err := u.orgMemberRepo.IsUserActiveInOrg(ctx, targetUserID, orgID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if !active {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeOrgMemberStatusConflict, "成员非 active 状态，禁止改角色")
	}

	operatorLevel, err := u.resolveOperatorRoleMatrixLevel(ctx, operatorID, orgID)
	if err != nil {
		return nil, err
	}

	assignedRoles, err := u.roleRepo.GetUserRolesByOrg(ctx, targetUserID, orgID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	activeRoles, err := u.roleRepo.GetActiveRoles(ctx)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	sort.SliceStable(activeRoles, func(i, j int) bool {
		left := activeRoles[i]
		right := activeRoles[j]
		if left == nil || right == nil {
			return left != nil
		}
		leftWeight := userRoleMatrixSortWeight(left.Code)
		rightWeight := userRoleMatrixSortWeight(right.Code)
		if leftWeight != rightWeight {
			return leftWeight < rightWeight
		}
		return left.ID < right.ID
	})

	roleItems := make([]resp.UserRoleMatrixRoleItem, 0, len(activeRoles))
	roleItemsByID := make(map[uint]resp.UserRoleMatrixRoleItem, len(activeRoles))
	for _, role := range activeRoles {
		if role == nil || role.ID == 0 {
			continue
		}

		matrixLevel := userRoleMatrixLevelForRole(role.Code)
		assignable, disabledReason := userRoleMatrixAssignable(operatorLevel, role.Code)
		roleItem := resp.UserRoleMatrixRoleItem{
			ID:          role.ID,
			Name:        role.Name,
			Code:        role.Code,
			IsBuiltin:   consts.IsBuiltinRole(role.Code),
			MatrixLevel: string(matrixLevel),
			Assignable:  assignable,
		}
		if !assignable {
			roleItem.DisabledReason = disabledReason
		}

		roleItems = append(roleItems, roleItem)
		roleItemsByID[role.ID] = roleItem
	}

	return &userRoleMatrixBuildResult{
		response: &resp.UserRoleMatrixItem{
			AssignedRoleIDs:     collectUserRoleIDs(assignedRoles),
			OperatorMatrixLevel: string(operatorLevel),
			Roles:               roleItems,
		},
		roleItemsByID: roleItemsByID,
	}, nil
}

// resolveOperatorRoleMatrixLevel 决定操作者在用户角色矩阵中的级别，优先级为：超级管理员 > 组织管理员 > 成员
func (u *UserService) resolveOperatorRoleMatrixLevel(
	ctx context.Context,
	operatorID, orgID uint,
) (userRoleMatrixLevel, error) {
	// 1. 获取操作者的全局角色，判断是否包含超级管理员角色
	globalRoles, err := u.roleRepo.GetUserGlobalRoles(ctx, operatorID)
	if err != nil {
		return "", bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	for _, role := range globalRoles {
		if role != nil && role.Code == consts.RoleCodeSuperAdmin {
			return userRoleMatrixLevelSuperAdmin, nil
		}
	}

	// 2. 获取操作者在当前组织的角色，判断是否包含组织管理员角色
	org, err := u.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if org == nil {
		return "", bizerrors.New(bizerrors.CodeOrgNotFound)
	}
	if org.OwnerID == operatorID {
		return userRoleMatrixLevelOrgAdmin, nil
	}

	// 3. 获取操作者在当前组织的角色，判断是否包含成员角色
	orgRoles, err := u.roleRepo.GetUserRolesByOrg(ctx, operatorID, orgID)
	if err != nil {
		return "", bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	for _, role := range orgRoles {
		if role != nil && role.Code == consts.RoleCodeOrgAdmin {
			return userRoleMatrixLevelOrgAdmin, nil
		}
	}
	return userRoleMatrixLevelMember, nil
}

// userRoleMatrixLevelForRole 根据角色 code 映射到矩阵级别
func userRoleMatrixLevelForRole(roleCode string) userRoleMatrixLevel {
	switch roleCode {
	case consts.RoleCodeSuperAdmin:
		return userRoleMatrixLevelSuperAdmin
	case consts.RoleCodeOrgAdmin:
		return userRoleMatrixLevelOrgAdmin
	default:
		return userRoleMatrixLevelMember
	}
}

// userRoleMatrixAssignable 根据操作者矩阵级别和目标角色，判断是否可分配，并返回不可分配的原因
func userRoleMatrixAssignable(
	operatorLevel userRoleMatrixLevel,
	roleCode string,
) (bool, string) {
	// 超级管理员角色不可分配，只能由系统自动赋予，且不受矩阵规则限制
	if roleCode == consts.RoleCodeSuperAdmin {
		return false, userRoleMatrixDisabledReasonGlobalRoleOnly
	}
	// 组织管理员角色不可分配，只能由系统自动赋予，且不受矩阵规则限制
	roleLevel := userRoleMatrixLevelForRole(roleCode)
	if operatorLevel == userRoleMatrixLevelMember && roleLevel == userRoleMatrixLevelOrgAdmin {
		return false, userRoleMatrixDisabledReasonHigherMatrixLevel
	}
	return true, ""
}

func userRoleMatrixSortWeight(roleCode string) int {
	switch roleCode {
	case consts.RoleCodeSuperAdmin:
		return 0
	case consts.RoleCodeOrgAdmin:
		return 1
	case consts.RoleCodeMember:
		return 2
	default:
		return 3
	}
}

func collectUserRoleIDs(roles []*entity.Role) []uint {
	ids := make([]uint, 0, len(roles))
	for _, role := range roles {
		if role == nil || role.ID == 0 {
			continue
		}
		ids = append(ids, role.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func normalizeUserRoleIDs(roleIDs []uint) []uint {
	if len(roleIDs) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(roleIDs))
	result := make([]uint, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		if roleID == 0 {
			continue
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		result = append(result, roleID)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// DeactivateAccount 主动注销账号（等同禁用）
func (u *UserService) DeactivateAccount(
	ctx context.Context,
	userID uint,
	req *request.DeactivateAccountReq,
) error {
	reason := ""
	if req != nil {
		reason = req.Reason
	}
	return u.applyUserStatus(ctx, userID, userID, consts.UserStatusDisabled, reason)
}

// UpdateUserStatus 管理员启用/禁用账号
func (u *UserService) UpdateUserStatus(
	ctx context.Context,
	operatorID, targetUserID uint,
	req *request.AdminUpdateUserStatusReq,
) error {
	if req == nil {
		return bizerrors.New(bizerrors.CodeInvalidParams)
	}

	var status consts.UserStatus
	switch strings.ToLower(strings.TrimSpace(req.Status)) {
	case "active":
		status = consts.UserStatusActive
	case "disabled":
		status = consts.UserStatusDisabled
	default:
		return bizerrors.New(bizerrors.CodeInvalidParams)
	}
	return u.applyUserStatus(ctx, operatorID, targetUserID, status, req.Reason)
}

// CleanupDisabledUsers 清理超过保留期的禁用账号（软删+匿名化）
func (u *UserService) CleanupDisabledUsers(ctx context.Context) (int, error) {
	if global.Config == nil || !global.Config.Task.DisabledUserCleanupEnabled {
		return 0, nil
	}
	retentionDays := global.Config.Task.DisabledUserRetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}
	before := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	users, err := u.userRepo.ListDisabledUsersBefore(ctx, before, 200)
	if err != nil {
		return 0, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if len(users) == 0 {
		return 0, nil
	}

	cleaned := 0
	for _, item := range users {
		if item == nil || item.ID == 0 {
			continue
		}
		orgIDs, err := u.orgMemberRepo.ListActiveOrgIDsByUser(ctx, item.ID)
		if err != nil {
			return cleaned, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := u.txRunner.InTx(ctx, func(tx any) error {
			txOrgMemberRepo := u.orgMemberRepo.WithTx(tx)
			txRoleRepo := u.roleRepo.WithTx(tx)
			txUserRepo := u.userRepo.WithTx(tx)

			for _, orgID := range orgIDs {
				if err := txOrgMemberRepo.SetStatus(
					ctx,
					orgID,
					item.ID,
					consts.OrgMemberStatusRemoved,
					nil,
					"disabled_cleanup",
					"",
				); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
				if err := txRoleRepo.DeleteUserOrgRoles(ctx, item.ID, orgID); err != nil {
					return bizerrors.Wrap(bizerrors.CodeDBError, err)
				}
				if u.permissionProjectionSvc != nil {
					if err := u.permissionProjectionSvc.PublishSubjectBindingChangedInTx(ctx, tx, item.ID, orgID); err != nil {
						return bizerrors.Wrap(bizerrors.CodeDBError, err)
					}
				}
			}
			if err := txUserRepo.SoftDeleteAndAnonymize(ctx, item.ID); err != nil {
				return err
			}
			if err := u.publishCacheProjectionInTx(
				ctx,
				tx,
				newUserDeletedProjectionEvent(item.ID, item.CurrentOrgID, orgIDs),
			); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
			return nil
		}); err != nil {
			return cleaned, err
		}
		if u.permissionProjectionSvc != nil {
			for _, orgID := range orgIDs {
				if err := u.permissionProjectionSvc.SyncSubjectRoles(ctx, item.ID, orgID); err != nil {
					return cleaned, bizerrors.Wrap(bizerrors.CodeInternalError, err)
				}
			}
		}
		cleaned++
	}
	return cleaned, nil
}

// applyUserStatus 内部方法：修改用户状态（启用/禁用），并处理相关逻辑（如清理会话、移除排行榜等）。
func (u *UserService) applyUserStatus(
	ctx context.Context,
	operatorID, targetUserID uint,
	status consts.UserStatus,
	reason string,
) error {
	// 获取目标用户，验证用户存在
	target, err := u.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if target == nil {
		return bizerrors.New(bizerrors.CodeUserNotFound)
	}

	if target.Status == status {
		return u.publishCacheProjection(ctx, newUserSnapshotProjectionEvent(targetUserID, nil))
	}

	var operatorPtr *uint
	if operatorID > 0 {
		op := operatorID
		operatorPtr = &op
	}

	// 禁用账号时，必须提供理由；启用账号时，理由可选但不允许过长。
	if err := u.txRunner.InTx(ctx, func(tx any) error {
		txUserRepo := u.userRepo.WithTx(tx)
		if err := txUserRepo.UpdateUserStatus(ctx, targetUserID, status, operatorPtr, strings.TrimSpace(reason)); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		if err := u.publishCacheProjectionInTx(ctx, tx, newUserSnapshotProjectionEvent(targetUserID, nil)); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
		return nil
	}); err != nil {
		return err
	}

	// 禁用后立即移除 refresh 会话（access 由 ActiveUserMW 即时拦截）
	if global.Redis != nil && target.UUID != uuid.Nil {
		if err := global.Redis.Del(ctx, target.UUID.String()).Err(); err != nil {
			return bizerrors.Wrap(bizerrors.CodeRedisError, err)
		}
	}
	return nil
}

func (u *UserService) publishCacheProjection(
	ctx context.Context,
	event *eventdto.CacheProjectionEvent,
) error {
	if event == nil || u.cacheProjectionPublisher == nil {
		return nil
	}
	if err := u.cacheProjectionPublisher.Publish(ctx, event); err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return nil
}

func (u *UserService) publishCacheProjectionInTx(
	ctx context.Context,
	tx any,
	event *eventdto.CacheProjectionEvent,
) error {
	if event == nil || u.cacheProjectionPublisher == nil {
		return nil
	}
	if err := u.cacheProjectionPublisher.PublishInTx(ctx, tx, event); err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return nil
}
