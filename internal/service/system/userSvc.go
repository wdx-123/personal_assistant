package system

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/imageops"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/util"
)

type UserService struct {
	userRepo          interfaces.UserRepository  // 依赖接口而不是具体实现
	roleRepo          interfaces.RoleRepository  // 角色仓储，用于获取默认角色
	imageRepo         interfaces.ImageRepository // 图片仓储
	permissionService *PermissionService         // 权限服务，用于RBAC角色分配
}

func NewUserService(
	repositoryGroup *repository.Group,
	permissionService *PermissionService,
) *UserService {
	return &UserService{
		userRepo:          repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		imageRepo:         repositoryGroup.SystemRepositorySupplier.GetImageRepository(),
		permissionService: permissionService,
	}
}

// Register 注册
func (u *UserService) Register(
	ctx *gin.Context,
	req *request.RegisterReq,
) (*entity.User, error) {
	//// 1. 验证图片验证码
	//if !base64Captcha.DefaultMemStore.Verify(req.CaptchaID, req.Captcha, true) {
	//	return ni  ml, errors.New("验证码错误")
	//}

	// 2. 检查手机号是否已存在
	exists, err := u.userRepo.ExistsByPhone(ctx, req.Phone)
	if err != nil {
		global.Log.Error("检查手机号是否存在时发生错误",
			zap.String("phone", req.Phone), zap.Error(err))
		return nil, errors.New("系统错误，请稍后重试")
	}
	if exists {
		global.Log.Error("手机号已被注册",
			zap.String("phone", req.Phone))
		return nil, errors.New("该手机号已被注册")
	}

	// 3. 创建用户实例（不直接设置RoleID，通过权限服务分配）
	user := &entity.User{
		Username: req.Username,
		Password: util.BcryptHash(req.Password),
		Phone:    req.Phone,
		UUID:     uuid.Must(uuid.NewV4()),
		Avatar:   "", // 默认头像为空
		Register: consts.Email,
		// 不直接设置 RoleID，将通过权限服务分配角色
	}

	// 4. 处理组织选择
	if req.OrgID <= 0 {
		return nil, errors.New("必须指定组织")
	}
	user.CurrentOrgID = &req.OrgID

	global.Log.Error(user.Password)

	err = u.userRepo.Create(ctx, user)
	if err != nil {
		global.Log.Error("创建用户失败",
			zap.String("phone", req.Phone),
			zap.String("username", req.Username),
			zap.Error(err))
		return nil, errors.New("创建用户失败，请稍后重试")
	}

	// 5. 为新用户分配默认角色（从配置获取）
	defaultRoleCode := global.Config.System.DefaultRoleCode
	if defaultRoleCode == "" {
		defaultRoleCode = "user" // 兜底默认值
	}

	// 根据角色代码查找角色
	defaultRole, err := u.roleRepo.GetByCode(ctx, defaultRoleCode)
	if err != nil {
		global.Log.Error("获取默认角色失败",
			zap.String("role_code", defaultRoleCode),
			zap.Error(err))
		return nil, fmt.Errorf("获取默认角色失败: %w", err)
	}

	// 分配角色
	err = u.permissionService.AssignRoleToUserInOrg(ctx, user.ID, req.OrgID, defaultRole.ID)
	if err != nil {
		global.Log.Error("分配默认角色失败",
			zap.Uint("user_id", user.ID),
			zap.Uint("role_id", defaultRole.ID),
			zap.String("role_code", defaultRole.Code),
			zap.Error(err))
		return nil, fmt.Errorf("分配默认角色失败: %w", err)
	}

	// 6. 重新查询用户，确保返回包含关联数据（如CurrentOrg）的完整对象
	// 这对于后续直接生成 Token 并返回完整信息至关重要
	fullUser, err := u.userRepo.GetByID(ctx, user.ID)
	if err != nil {
		global.Log.Error("注册后获取用户信息失败", zap.Error(err))
		// 如果获取失败，降级返回原始 user 对象（虽然缺少关联信息，但不影响核心流程）
		return user, nil
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
	if user.Freeze {
		return nil, errors.New("账号已被冻结")
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
	err := global.DB.Transaction(func(tx *gorm.DB) error {
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

	// 调用权限服务进行角色分配（全量替换）
	return u.permissionService.ReplaceUserRolesInOrg(ctx, req.UserID, req.OrgID, req.RoleIDs)
}
