package system

import (
	"context"
	"errors"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/repository/interfaces"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/mojocn/base64Captcha"
	"go.uber.org/zap"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/pkg/util"
)

type UserService struct {
	userRepo          interfaces.UserRepository // 依赖接口而不是具体实现
	roleRepo          interfaces.RoleRepository // 角色仓储，用于获取默认角色
	permissionService *PermissionService        // 权限服务，用于RBAC角色分配
}

func NewUserService(
	repositoryGroup *repository.Group,
	permissionService *PermissionService,
) *UserService {
	return &UserService{
		userRepo:          repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
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
