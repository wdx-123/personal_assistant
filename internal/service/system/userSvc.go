package system

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/imageops"
	"personal_assistant/pkg/rediskey"

	"github.com/gofrs/uuid"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/util"
)

type UserService struct {
	txRunner          repository.TxRunner
	userRepo          interfaces.UserRepository // 依赖接口而不是具体实现
	roleRepo          interfaces.RoleRepository // 角色仓储，用于获取默认角色
	orgRepo           interfaces.OrgRepository
	orgMemberRepo     interfaces.OrgMemberRepository
	imageRepo         interfaces.ImageRepository // 图片仓储
	permissionService *PermissionService         // 权限服务，用于RBAC角色分配
}

func NewUserService(
	repositoryGroup *repository.Group,
	permissionService *PermissionService,
) *UserService {
	return &UserService{
		txRunner:          repositoryGroup,
		userRepo:          repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		orgRepo:           repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		orgMemberRepo:     repositoryGroup.SystemRepositorySupplier.GetOrgMemberRepository(),
		imageRepo:         repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
		permissionService: permissionService,
	}
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

	// 获取默认角色（优先使用系统配置的默认角色code，如果配置无效则回退到 member 角色）。如果默认角色不存在，则注册失败。
	defaultRoleCode := strings.TrimSpace(global.Config.System.DefaultRoleCode)
	if defaultRoleCode == "" {
		defaultRoleCode = consts.RoleCodeMember
	}
	// 获取默认角色，如果不存在则注册失败。
	defaultRole, err := u.roleRepo.GetByCode(ctx, defaultRoleCode)
	if err != nil || defaultRole == nil {
		defaultRole, err = u.roleRepo.GetByCode(ctx, consts.RoleCodeMember)
		if err != nil || defaultRole == nil {
			if err != nil {
				return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
			return nil, bizerrors.New(bizerrors.CodeRoleNotFound)
		}
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

		// 邀请码组织 + 全体成员组织均授予默认 member 角色。
		if err := txRoleRepo.AssignRoleToUserInOrg(ctx, user.ID, targetOrg.ID, defaultRole.ID); err != nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}

		// 如果邀请码组织不是全体成员组织，则全体成员组织也授予默认 member 角色。
		if allMembersOrg.ID != targetOrg.ID {
			if err := txRoleRepo.AssignRoleToUserInOrg(ctx, user.ID, allMembersOrg.ID, defaultRole.ID); err != nil {
				return bizerrors.Wrap(bizerrors.CodeDBError, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 同步 Casbin 角色关系（失败仅记录日志，不影响注册成功）。
	if u.permissionService != nil && u.permissionService.casbinSvc != nil && u.permissionService.casbinSvc.Enforcer != nil {
		subjectTarget := fmt.Sprintf("%d@%d", user.ID, targetOrg.ID)
		if _, err := u.permissionService.casbinSvc.Enforcer.AddRoleForUser(subjectTarget, defaultRole.Code); err != nil {
			global.Log.Error("注册后同步目标组织角色到Casbin失败", zap.Error(err))
		}
		if allMembersOrg.ID != targetOrg.ID {
			subjectAll := fmt.Sprintf("%d@%d", user.ID, allMembersOrg.ID)
			if _, err := u.permissionService.casbinSvc.Enforcer.AddRoleForUser(subjectAll, defaultRole.Code); err != nil {
				global.Log.Error("注册后同步全体成员角色到Casbin失败", zap.Error(err))
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
		// 更新用户名
		if req.Username != nil && *req.Username != "" {
			user.Username = *req.Username
			updated = true
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
			ID:       user.ID,
			Username: user.Username,
			Phone:    util.DesensitizePhone(user.Phone),
		}
		if user.CurrentOrg != nil {
			item.CurrentOrg.ID = user.CurrentOrg.ID
			item.CurrentOrg.Name = user.CurrentOrg.Name
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

// AssignRole 分配角色
func (u *UserService) AssignRole(
	ctx context.Context,
	req *request.AssignUserRoleReq,
) error {
	// 检查用户是否存在
	user, err := u.userRepo.GetByID(ctx, req.UserID)
	if err != nil || user == nil {
		return errors.New("用户不存在")
	}

	active, err := u.orgMemberRepo.IsUserActiveInOrg(ctx, req.UserID, req.OrgID)
	if err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if !active {
		return bizerrors.NewWithMsg(bizerrors.CodeOrgMemberStatusConflict, "成员非 active 状态，禁止改角色")
	}

	// 调用权限服务进行角色分配（全量替换）
	return u.permissionService.ReplaceUserRolesInOrg(ctx, req.UserID, req.OrgID, req.RoleIDs)
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
			}
			return txUserRepo.SoftDeleteAndAnonymize(ctx, item.ID)
		}); err != nil {
			return cleaned, err
		}

		if err := u.removeUserFromRankingByOrgIDs(ctx, item.ID, orgIDs); err != nil {
			return cleaned, bizerrors.Wrap(bizerrors.CodeRedisError, err)
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
		return nil
	}

	var operatorPtr *uint
	if operatorID > 0 {
		op := operatorID
		operatorPtr = &op
	}

	// 禁用账号时，必须提供理由；启用账号时，理由可选但不允许过长。
	if err := u.userRepo.UpdateUserStatus(ctx, targetUserID, status, operatorPtr, strings.TrimSpace(reason)); err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	// 禁用后立即移除 refresh 会话（access 由 ActiveUserMW 即时拦截）
	if target.UUID.String() != "" {
		if err := global.Redis.Del(ctx, target.UUID.String()).Err(); err != nil {
			return bizerrors.Wrap(bizerrors.CodeRedisError, err)
		}
	}

	// 如果是禁用账号，则将用户从所有组织的排行榜中移除
	if status == consts.UserStatusDisabled {
		if err := u.removeUserFromRankingByOrgIDs(ctx, targetUserID, nil); err != nil {
			return bizerrors.Wrap(bizerrors.CodeRedisError, err)
		}
	}
	return nil
}

// removeUserFromRankingByOrgIDs 将用户从指定组织ID列表的排行榜中移除。如果orgIDs为nil或空，则自动查询用户当前活跃的组织列表进行移除。
func (u *UserService) removeUserFromRankingByOrgIDs(ctx context.Context, userID uint, orgIDs []uint) error {
	if len(orgIDs) == 0 {
		var err error
		orgIDs, err = u.orgMemberRepo.ListActiveOrgIDsByUser(ctx, userID)
		if err != nil {
			return err
		}
	}

	member := strconv.FormatUint(uint64(userID), 10)
	for _, orgID := range orgIDs {
		if err := global.Redis.ZRem(ctx, rediskey.RankingZSetKey(orgID, "luogu"), member).Err(); err != nil {
			return err
		}
		if err := global.Redis.ZRem(ctx, rediskey.RankingZSetKey(orgID, "leetcode"), member).Err(); err != nil {
			return err
		}
	}
	return nil
}
