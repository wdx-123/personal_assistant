package contract

import (
	"context"
	"mime/multipart"

	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
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

type AuthorizationServiceContract interface {
	// GetUserRoles 获取用户角色
	GetUserRoles(ctx context.Context, userID uint) ([]entity.Role, error)
	// IsSuperAdmin 判断用户是否具备全局超级管理员角色
	IsSuperAdmin(ctx context.Context, userID uint) (bool, error)
	// CheckUserAPIPermission 检查用户API权限
	CheckUserAPIPermission(ctx context.Context, userID uint, apiPath, method string) (bool, error)
	// CheckUserCapabilityInOrg 检查用户在组织中的能力
	CheckUserCapabilityInOrg(ctx context.Context, userID, orgID uint, capabilityCode string) (bool, error)
	// AuthorizeOrgCapability 授权组织能力
	AuthorizeOrgCapability(ctx context.Context, operatorID, orgID uint, capabilityCode string) error
}

type PermissionProjectionServiceContract interface {
	// RebuildAll 重建所有权限投影
	RebuildAll(ctx context.Context) error
	// ReloadPolicy 重新加载策略
	ReloadPolicy(ctx context.Context) error
	// SyncSubjectRoles 同步主体角色
	SyncSubjectRoles(ctx context.Context, userID, orgID uint) error
	// PublishSubjectBindingChanged 发布主体绑定变更事件
	PublishSubjectBindingChanged(ctx context.Context, userID, orgID uint) error
	// PublishSubjectBindingChangedInTx 发布主体绑定变更事件（事务中）
	PublishSubjectBindingChangedInTx(ctx context.Context, tx any, userID, orgID uint) error
	// PublishPermissionGraphChanged 发布权限图变更事件
	PublishPermissionGraphChanged(ctx context.Context, aggregateType string, aggregateID uint) error
	// PublishPermissionGraphChangedInTx 发布权限图变更事件（事务中）
	PublishPermissionGraphChangedInTx(ctx context.Context, tx any, aggregateType string, aggregateID uint) error
	// HandlePermissionProjectionEvent 处理权限投影事件
	HandlePermissionProjectionEvent(ctx context.Context, event *eventdto.PermissionProjectionEvent) error
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
	AssignRole(ctx context.Context, operatorID uint, req *request.AssignUserRoleReq) error

	// DeactivateAccount 注销账号
	DeactivateAccount(ctx context.Context, userID uint, req *request.DeactivateAccountReq) error

	// UpdateUserStatus 更新账号状态（禁用/启用）
	UpdateUserStatus(ctx context.Context, operatorID, targetUserID uint, req *request.AdminUpdateUserStatusReq) error

	// CleanupDisabledUsers 清理过期的禁用用户，返回清理的用户数量
	CleanupDisabledUsers(ctx context.Context) (int, error)
}

type OrgServiceContract interface {
	GetOrgList(ctx context.Context, userID uint, page, pageSize int, keyword string) ([]*readmodel.OrgWithMemberCount, int64, error)
	GetOrgDetail(ctx context.Context, userID uint, orgID uint) (*readmodel.OrgWithMemberCount, error)
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
	BindLanqiaoAccount(ctx context.Context, userID uint, req *request.BindLanqiaoAccountReq) (*resp.BindOJAccountResp, error)
	GetRankingList(ctx context.Context, userID uint, req *request.OJRankingListReq) (*resp.OJRankingListResp, error)
	GetUserStats(ctx context.Context, userID uint, req *request.OJStatsReq) (*resp.OJStatsResp, error)
	GetCurve(ctx context.Context, userID uint, req *request.OJCurveReq) (*resp.OJCurveResp, error)
	SyncAllLuoguUsers(ctx context.Context) error
	SyncAllLeetcodeUsers(ctx context.Context) error
	SyncAllLanqiaoUsers(ctx context.Context) error
	RefreshAllLanqiaoSubmissionStats(ctx context.Context) error
	RebuildRankingCaches(ctx context.Context) error
	HandleLuoguBindPayload(ctx context.Context, userID uint, payload *eventdto.LuoguBindPayload) error
	HandleLeetcodeBindSignal(ctx context.Context, userID uint) error
}

type OJDailyStatsProjectionServiceContract interface {
	PublishOJDailyStatsProjectionEvent(ctx context.Context, event *eventdto.OJDailyStatsProjectionEvent) error
	HandleOJDailyStatsProjectionEvent(ctx context.Context, event *eventdto.OJDailyStatsProjectionEvent) error
	RebuildRecentWindow(ctx context.Context, userID uint, platform string, reset bool) error
	RepairRecentWindow(ctx context.Context) error
}

type CacheProjectionServiceContract interface {
	// HandleCacheProjectionEvent 处理缓存投影事件，根据事件类型和数据更新对应的缓存状态，确保系统内的缓存数据与底层数据源保持一致。
	HandleCacheProjectionEvent(ctx context.Context, event *eventdto.CacheProjectionEvent) error

	// RebuildAll 全量重建所有缓存数据，通常在系统启动或重大数据变更后调用，以确保缓存与数据库完全同步。该方法会依次重建各个模块的缓存，并在过程中记录重建结果和可能的错误。
	RebuildAll(ctx context.Context) error
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
	GetAuthorizationSvc() AuthorizationServiceContract
	GetPermissionProjectionSvc() PermissionProjectionServiceContract
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
	GetCacheProjectionSvc() CacheProjectionServiceContract
	GetOJDailyStatsProjectionSvc() OJDailyStatsProjectionServiceContract
}
