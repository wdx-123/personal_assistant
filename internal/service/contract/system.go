package contract

import (
	"context"
	"mime/multipart"

	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	erro "personal_assistant/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)

type JWTServiceContract interface {
	IssueLoginTokens(ctx context.Context, user entity.User) (*resp.LoginResponse, string, int64, *erro.JWTError)
	IsInBlacklist(jwt string) bool
	GetAccessToken(ctx context.Context, token string) (*resp.RefreshTokenResponse, *erro.JWTError)
	JoinInBlacklist(ctx context.Context, jwtList entity.JwtBlacklist) error
}

type PermissionServiceContract interface {
	SyncAllPermissionsToCasbin(ctx context.Context) error
	GetUserRoles(ctx context.Context, userID uint) ([]entity.Role, error)
	CheckUserAPIPermission(userID uint, apiPath, method string) (bool, error)

	// CheckUserMenuPermission 检查用户是否有访问菜单的权限（基于菜单绑定的 API 权限）
	CheckUserCapabilityInOrg(ctx context.Context, userID, orgID uint, capabilityCode string) (bool, error)

	// GetUserCapabilitiesInOrg 获取用户在特定组织内的 capability 列表
	GetAllCapabilityGroups(ctx context.Context) ([]resp.CapabilityGroupItem, error)

	// GetUserCapabilitiesInOrg 获取用户在特定组织内的 capability 列表
	GetRoleCapabilityCodes(ctx context.Context, roleID uint) ([]string, error)
}

type BaseServiceContract interface {
	GetCaptcha(store base64Captcha.Store) (string, string, error)
	VerifyAndSendEmailCode(ctx *gin.Context, store base64Captcha.Store, req *request.SendEmailVerificationCodeReq) error
}

type HealthServiceContract interface {
	Health(ctx context.Context) (*resp.HealthResponse, error)
	Ping(ctx context.Context) (*resp.PingResponse, error)
}

type UserServiceContract interface {
	Register(ctx context.Context, req *request.RegisterReq) (*entity.User, error)
	PhoneLogin(ctx context.Context, req *request.LoginReq) (*entity.User, error)
	UpdateProfile(ctx context.Context, userID uint, req *request.UpdateProfileReq) (*entity.User, error)
	ChangePhone(ctx context.Context, userID uint, req *request.ChangePhoneReq) (*entity.User, error)
	ChangePassword(ctx context.Context, userID uint, req *request.ChangePasswordReq) error
	GetUserList(ctx context.Context, req *request.UserListReq) (*resp.PageDataUser, error)
	GetUserDetail(ctx context.Context, id uint) (*entity.User, error)
	GetUserRoles(ctx context.Context, userID, orgID uint) ([]*entity.Role, error)
	AssignRole(ctx context.Context, req *request.AssignUserRoleReq) error

	// DeactivateAccount 注销账号
	DeactivateAccount(ctx context.Context, userID uint, req *request.DeactivateAccountReq) error

	// UpdateUserStatus 更新账号状态（禁用/启用）
	UpdateUserStatus(ctx context.Context, operatorID, targetUserID uint, req *request.AdminUpdateUserStatusReq) error

	// CleanupDisabledUsers 清理过期的禁用用户，返回清理的用户数量
	CleanupDisabledUsers(ctx context.Context) (int, error)
}

type OrgServiceContract interface {
	GetOrgList(ctx context.Context, page, pageSize int, keyword string) ([]*entity.Org, int64, error)
	GetOrgDetail(ctx context.Context, userID uint, orgID uint) (*entity.Org, error)
	CreateOrg(ctx context.Context, userID uint, req *request.CreateOrgReq) error
	UpdateOrg(ctx context.Context, userID, orgID uint, req *request.UpdateOrgReq) error
	DeleteOrg(ctx context.Context, userID, orgID uint, force bool) error
	SetCurrentOrg(ctx context.Context, userID, orgID uint) error
	GetMyOrgs(ctx context.Context, userID uint) ([]*resp.MyOrgItem, error)

	// JoinOrgByInviteCode 加入组织
	JoinOrgByInviteCode(ctx context.Context, userID uint, inviteCode string) error

	// LeaveOrg 退出组织
	LeaveOrg(ctx context.Context, userID, orgID uint, reason string) error

	// KickMember 踢出成员
	KickMember(ctx context.Context, operatorID, orgID, targetUserID uint, reason string) error

	// RecoverMember 恢复成员（撤销踢出/移除）
	RecoverMember(ctx context.Context, operatorID, orgID, targetUserID uint, reason string) error
}

type OJServiceContract interface {
	BindOJAccount(ctx context.Context, userID uint, req *request.BindOJAccountReq) (*resp.BindOJAccountResp, error)
	GetRankingList(ctx context.Context, userID uint, req *request.OJRankingListReq) (*resp.OJRankingListResp, error)
	GetUserStats(ctx context.Context, userID uint, req *request.OJStatsReq) (*resp.BindOJAccountResp, error)
	SyncAllLuoguUsers(ctx context.Context) error
	SyncAllLeetcodeUsers(ctx context.Context) error
	RebuildRankingCaches(ctx context.Context) error
	HandleLuoguBindPayload(ctx context.Context, userID uint, payload *eventdto.LuoguBindPayload) error
	HandleLeetcodeBindSignal(ctx context.Context, userID uint) error
}

type ApiServiceContract interface {
	GetAPIList(ctx context.Context, filter *request.ApiListFilter) ([]*entity.API, map[uint]*entity.Menu, int64, error)
	GetAPIByID(ctx context.Context, id uint) (*entity.API, *entity.Menu, error)
	CreateAPI(ctx context.Context, req *request.CreateApiReq) error
	UpdateAPI(ctx context.Context, id uint, req *request.UpdateApiReq) error
	DeleteAPI(ctx context.Context, id uint) error
	SyncAPI(ctx context.Context, deleteRemoved bool) (added, updated, disabled int, total int, err error)
}

type MenuServiceContract interface {
	GetMenuTree(ctx context.Context) ([]*resp.MenuItem, error)
	GetMyMenus(ctx context.Context, userID uint, orgID *uint) ([]*resp.MenuItem, error)
	GetMenuList(ctx context.Context, filter *request.MenuListFilter) ([]*entity.Menu, int64, error)
	GetMenuByID(ctx context.Context, id uint) (*entity.Menu, error)
	CreateMenu(ctx context.Context, req *request.CreateMenuReq) error
	UpdateMenu(ctx context.Context, id uint, req *request.UpdateMenuReq) error
	DeleteMenu(ctx context.Context, id uint) error
	BindAPIs(ctx context.Context, menuID uint, apiIDs []uint) error
}

type RoleServiceContract interface {
	GetRoleList(ctx context.Context, filter *request.RoleListFilter) ([]*entity.Role, int64, error)
	CreateRole(ctx context.Context, req *request.CreateRoleReq) error
	UpdateRole(ctx context.Context, id uint, req *request.UpdateRoleReq) error
	DeleteRole(ctx context.Context, id uint) error
	AssignPermissions(ctx context.Context, roleID uint, menuIDs []uint, directAPIIDs []uint, capabilityCodes []string) error
	GetRoleMenuAPIMap(ctx context.Context, roleID uint, maxLevel *int) (*resp.RoleMenuAPIMappingItem, error)
}

type ImageServiceContract interface {
	Upload(ctx context.Context, files []*multipart.FileHeader, req *request.UploadImageReq, uploaderID uint) ([]resp.ImageItem, error)
	Delete(ctx context.Context, ids []uint) error
	List(ctx context.Context, req *request.ListImageReq) ([]resp.ImageItem, int64, error)
	CleanOrphanFiles(ctx context.Context) error
}

type ObservabilityServiceContract interface {
	QueryMetrics(ctx context.Context, req *request.ObservabilityMetricsQueryReq) (*resp.ObservabilityMetricsQueryResp, error)
	QueryRuntimeMetrics(ctx context.Context, req *request.ObservabilityRuntimeMetricQueryReq) (*resp.ObservabilityRuntimeMetricQueryResp, error)
	QueryTraceDetail(
		ctx context.Context,
		id string,
		idType string,
		limit int,
		offset int,
		includePayload bool,
		includeErrorDetail bool,
	) (*resp.ObservabilityTraceQueryResp, error)
	QueryTrace(ctx context.Context, req *request.ObservabilityTraceQueryReq) (*resp.ObservabilityTraceSummaryQueryResp, error)
}

type Supplier interface {
	GetJWTSvc() JWTServiceContract
	GetPermissionSvc() PermissionServiceContract
	GetBaseSvc() BaseServiceContract
	GetHealthSvc() HealthServiceContract
	GetUserSvc() UserServiceContract
	GetOrgSvc() OrgServiceContract
	GetOJSvc() OJServiceContract
	GetApiSvc() ApiServiceContract
	GetMenuSvc() MenuServiceContract
	GetRoleSvc() RoleServiceContract
	GetImageSvc() ImageServiceContract
	GetObservabilitySvc() ObservabilityServiceContract
}
