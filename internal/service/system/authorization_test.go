package system

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/glebarez/sqlite"
	"github.com/gofrs/uuid"
	"gorm.io/gorm"

	"personal_assistant/global"
	cfg "personal_assistant/internal/model/config"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	repositoryadapter "personal_assistant/internal/repository/adapter"
	reposystem "personal_assistant/internal/repository/system"
	pkgcasbin "personal_assistant/pkg/casbin"
	bizerrors "personal_assistant/pkg/errors"
)

type authorizationTestEnv struct {
	db            *gorm.DB
	enforcer      *casbinlib.Enforcer
	authorization *AuthorizationService
	projection    *PermissionProjectionService
	orgService    *OrgService
	userService   *UserService
}

func newAuthorizationTestEnv(t *testing.T) *authorizationTestEnv {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.User{},
		&entity.Org{},
		&entity.OrgMember{},
		&entity.Role{},
		&entity.Menu{},
		&entity.API{},
		&entity.MenuAPI{},
		&entity.RoleAPI{},
		&entity.Capability{},
		&entity.UserOrgRole{},
		&entity.RoleCapability{},
		&entity.Image{},
		&entity.OutboxEvent{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	modelPath := projectConfigPath(t)
	casbinModel, err := model.NewModelFromFile(modelPath)
	if err != nil {
		t.Fatalf("load casbin model: %v", err)
	}
	enforcer, err := casbinlib.NewEnforcer(casbinModel)
	if err != nil {
		t.Fatalf("create casbin enforcer: %v", err)
	}

	oldEnforcer := global.CasbinEnforcer
	oldConfig := global.Config
	global.CasbinEnforcer = enforcer
	global.Config = &cfg.Config{
		System: cfg.System{
			DefaultRoleCode: consts.RoleCodeMember,
		},
		Messaging: cfg.Messaging{
			CacheProjectionTopic:          "cache_projection",
			PermissionProjectionTopic:     "permission_projection",
			PermissionPolicyReloadChannel: "permission_policy_reload",
		},
	}
	t.Cleanup(func() {
		global.CasbinEnforcer = oldEnforcer
		global.Config = oldConfig
	})

	repoGroup := &repository.Group{
		SystemRepositorySupplier: reposystem.SetUp(&repositoryadapter.FactoryConfig{
			DatabaseType: repositoryadapter.MySQL,
			Connection:   db,
		}),
	}

	authorizationService := NewAuthorizationService(repoGroup)
	authorizationService.casbinSvc = pkgcasbin.NewServiceWithEnforcer(enforcer)
	projectionService := NewPermissionProjectionService(repoGroup)
	projectionService.casbinSvc = pkgcasbin.NewServiceWithEnforcer(enforcer)

	return &authorizationTestEnv{
		db:            db,
		enforcer:      enforcer,
		authorization: authorizationService,
		projection:    projectionService,
		orgService:    NewOrgService(repoGroup, authorizationService, projectionService),
		userService:   NewUserService(repoGroup, authorizationService, projectionService),
	}
}

func projectConfigPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "configs", "model.conf")
}

func createOrg(t *testing.T, env *authorizationTestEnv, ownerID uint) *entity.Org {
	t.Helper()
	org := &entity.Org{
		Name:    fmt.Sprintf("org-%d", ownerID),
		Code:    fmt.Sprintf("ORG-%d", ownerID),
		OwnerID: ownerID,
	}
	if err := env.db.Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	return org
}

func createUser(t *testing.T, env *authorizationTestEnv, label string) *entity.User {
	t.Helper()
	userUUID := uuid.Must(uuid.NewV4())
	user := &entity.User{
		UUID:     userUUID,
		Username: fmt.Sprintf("user-%s", label),
		Phone:    fmt.Sprintf("138%08s", label),
		Password: "hashed-password",
		Status:   consts.UserStatusActive,
	}
	if err := env.db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func grantGlobalRole(t *testing.T, env *authorizationTestEnv, userID uint, roleCode string) {
	t.Helper()
	role := &entity.Role{
		Name:   roleCode,
		Code:   roleCode,
		Status: 1,
	}
	if err := env.db.Create(role).Error; err != nil {
		t.Fatalf("create global role: %v", err)
	}
	relation := &entity.UserOrgRole{
		UserID: userID,
		OrgID:  0,
		RoleID: role.ID,
	}
	if err := env.db.Create(relation).Error; err != nil {
		t.Fatalf("create global role relation: %v", err)
	}
}

func createRole(t *testing.T, env *authorizationTestEnv, code string) *entity.Role {
	t.Helper()
	role := &entity.Role{
		Name:   code,
		Code:   code,
		Status: 1,
	}
	if err := env.db.Create(role).Error; err != nil {
		t.Fatalf("create role %s: %v", code, err)
	}
	return role
}

func createCapability(t *testing.T, env *authorizationTestEnv, code string) *entity.Capability {
	t.Helper()
	capability := &entity.Capability{
		Code:      code,
		Name:      code,
		Status:    1,
		GroupCode: "test_group",
		GroupName: "测试分组",
		Domain:    "test",
	}
	if err := env.db.Create(capability).Error; err != nil {
		t.Fatalf("create capability %s: %v", code, err)
	}
	return capability
}

func assignUserRole(t *testing.T, env *authorizationTestEnv, userID, orgID, roleID uint) {
	t.Helper()
	relation := &entity.UserOrgRole{
		UserID: userID,
		OrgID:  orgID,
		RoleID: roleID,
	}
	if err := env.db.Create(relation).Error; err != nil {
		t.Fatalf("assign user role: %v", err)
	}
}

func bindRoleCapability(t *testing.T, env *authorizationTestEnv, roleID, capabilityID uint) {
	t.Helper()
	relation := &entity.RoleCapability{
		RoleID:       roleID,
		CapabilityID: capabilityID,
	}
	if err := env.db.Create(relation).Error; err != nil {
		t.Fatalf("bind role capability: %v", err)
	}
}

func grantOrgCapability(t *testing.T, env *authorizationTestEnv, userID, orgID uint, roleCode, capabilityCode string) {
	t.Helper()
	subject := pkgcasbin.BuildSubject(userID, orgID)
	if _, err := env.enforcer.AddRoleForUser(subject, roleCode); err != nil {
		t.Fatalf("add role for subject: %v", err)
	}
	if _, err := env.enforcer.AddPermissionForUser(roleCode, capabilityCode, pkgcasbin.ActionOperate); err != nil {
		t.Fatalf("add capability permission: %v", err)
	}
}

func assertBizCode(t *testing.T, err error, want bizerrors.BizCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected biz error %d, got nil", want)
	}
	bizErr := bizerrors.FromError(err)
	if bizErr == nil {
		t.Fatalf("expected biz error %d, got %T: %v", want, err, err)
	}
	if bizErr.Code != want {
		t.Fatalf("expected biz error %d, got %d (%v)", want, bizErr.Code, err)
	}
}

func TestAuthorizationServiceAuthorizeOrgCapability(t *testing.T) {
	ctx := context.Background()

	t.Run("owner bypass", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)

		if err := env.authorization.AuthorizeOrgCapability(ctx, 11, org.ID, consts.CapabilityCodeOrgManageUpdate); err != nil {
			t.Fatalf("owner should bypass capability check: %v", err)
		}
	})

	t.Run("super admin bypass", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		grantGlobalRole(t, env, 22, consts.RoleCodeSuperAdmin)

		if err := env.authorization.AuthorizeOrgCapability(ctx, 22, org.ID, consts.CapabilityCodeOrgManageDelete); err != nil {
			t.Fatalf("super admin should bypass capability check: %v", err)
		}
	})

	t.Run("org capability allows", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		grantOrgCapability(t, env, 22, org.ID, "org_operator", consts.CapabilityCodeOrgManageUpdate)

		if err := env.authorization.AuthorizeOrgCapability(ctx, 22, org.ID, consts.CapabilityCodeOrgManageUpdate); err != nil {
			t.Fatalf("capability should allow action: %v", err)
		}
	})

	t.Run("deny without capability", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)

		err := env.authorization.AuthorizeOrgCapability(ctx, 22, org.ID, consts.CapabilityCodeOrgManageUpdate)
		assertBizCode(t, err, bizerrors.CodePermissionDenied)
	})

	t.Run("org not found", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)

		err := env.authorization.AuthorizeOrgCapability(ctx, 22, 404, consts.CapabilityCodeOrgManageUpdate)
		assertBizCode(t, err, bizerrors.CodeOrgNotFound)
	})

	t.Run("casbin error wrapped", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		env.authorization.casbinSvc = pkgcasbin.NewServiceWithEnforcer(nil)

		err := env.authorization.AuthorizeOrgCapability(ctx, 22, org.ID, consts.CapabilityCodeOrgManageUpdate)
		assertBizCode(t, err, bizerrors.CodeInternalError)
	})
}

func TestOrgServiceUpdateOrgAllowsCapabilityOperator(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	grantOrgCapability(t, env, 22, org.ID, "org_manager", consts.CapabilityCodeOrgManageUpdate)

	newName := "updated-org"
	if err := env.orgService.UpdateOrg(context.Background(), 22, org.ID, &request.UpdateOrgReq{Name: &newName}); err != nil {
		t.Fatalf("update org with capability: %v", err)
	}

	var updated entity.Org
	if err := env.db.First(&updated, org.ID).Error; err != nil {
		t.Fatalf("load updated org: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected org name %q, got %q", newName, updated.Name)
	}
}

func TestOrgServiceDeleteOrgAllowsCapabilityOperator(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	grantOrgCapability(t, env, 22, org.ID, "org_manager", consts.CapabilityCodeOrgManageDelete)

	if err := env.orgService.DeleteOrg(context.Background(), 22, org.ID, false); err != nil {
		t.Fatalf("delete org with capability: %v", err)
	}

	var count int64
	if err := env.db.Model(&entity.Org{}).Where("id = ?", org.ID).Count(&count).Error; err != nil {
		t.Fatalf("count deleted org: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected org to be deleted, count=%d", count)
	}
}

func TestUserServiceAssignRoleRequiresCapability(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	target := createUser(t, env, "00000001")

	err := env.userService.AssignRole(context.Background(), 22, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{1},
	})
	assertBizCode(t, err, bizerrors.CodePermissionDenied)
}

func TestAuthorizeOrgMemberActionHonorsBypassAndCapability(t *testing.T) {
	ctx := context.Background()

	t.Run("owner bypass", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)

		if err := env.orgService.authorizeOrgMemberAction(ctx, 11, org.ID, consts.OrgMemberActionKick); err != nil {
			t.Fatalf("owner should bypass member capability check: %v", err)
		}
	})

	t.Run("super admin bypass", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		grantGlobalRole(t, env, 22, consts.RoleCodeSuperAdmin)

		if err := env.orgService.authorizeOrgMemberAction(ctx, 22, org.ID, consts.OrgMemberActionRecover); err != nil {
			t.Fatalf("super admin should bypass member capability check: %v", err)
		}
	})

	t.Run("deny without capability", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)

		err := env.orgService.authorizeOrgMemberAction(ctx, 22, org.ID, consts.OrgMemberActionKick)
		assertBizCode(t, err, bizerrors.CodePermissionDenied)
	})
}

func TestAssignRoleUpdatesSubjectProjectionImmediately(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	operator := createUser(t, env, "00000002")
	target := createUser(t, env, "00000003")
	currentOrgID := org.ID
	target.CurrentOrgID = &currentOrgID
	if err := env.db.Model(&entity.User{}).Where("id = ?", target.ID).Update("current_org_id", currentOrgID).Error; err != nil {
		t.Fatalf("set current org: %v", err)
	}
	if err := env.db.Create(&entity.OrgMember{OrgID: org.ID, UserID: target.ID, MemberStatus: consts.OrgMemberStatusActive}).Error; err != nil {
		t.Fatalf("create org member: %v", err)
	}

	role := createRole(t, env, "org_auditor")
	grantOrgCapability(t, env, operator.ID, org.ID, "org_operator", consts.CapabilityCodeOrgMemberAssignRole)
	if _, err := env.enforcer.AddPermissionForUser(role.Code, consts.CapabilityCodeOrgManageUpdate, pkgcasbin.ActionOperate); err != nil {
		t.Fatalf("grant role capability: %v", err)
	}

	if err := env.userService.AssignRole(ctx, operator.ID, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{role.ID},
	}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	ok, err := env.authorization.CheckUserCapabilityInOrg(ctx, target.ID, org.ID, consts.CapabilityCodeOrgManageUpdate)
	if err != nil {
		t.Fatalf("check capability after assign: %v", err)
	}
	if !ok {
		t.Fatalf("expected assigned role projection to take effect immediately")
	}

	var outboxCount int64
	if err := env.db.Model(&entity.OutboxEvent{}).Where("event_type = ?", "permission_projection").Count(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount == 0 {
		t.Fatalf("expected subject binding event to be written to outbox")
	}
}

func TestPermissionGraphChangedEventRebuildsProjection(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	user := createUser(t, env, "00000004")
	role := createRole(t, env, "org_manager")
	capability := createCapability(t, env, consts.CapabilityCodeOrgManageUpdate)

	assignUserRole(t, env, user.ID, org.ID, role.ID)
	bindRoleCapability(t, env, role.ID, capability.ID)

	ok, err := env.authorization.CheckUserCapabilityInOrg(ctx, user.ID, org.ID, consts.CapabilityCodeOrgManageUpdate)
	if err != nil {
		t.Fatalf("check capability before rebuild: %v", err)
	}
	if ok {
		t.Fatalf("expected capability to be unavailable before projection rebuild")
	}

	if err := env.projection.HandlePermissionProjectionEvent(ctx, &eventdto.PermissionProjectionEvent{
		Kind:          eventdto.PermissionProjectionKindPermissionGraphChanged,
		AggregateType: "role",
		AggregateID:   role.ID,
	}); err != nil {
		t.Fatalf("handle permission graph event: %v", err)
	}

	ok, err = env.authorization.CheckUserCapabilityInOrg(ctx, user.ID, org.ID, consts.CapabilityCodeOrgManageUpdate)
	if err != nil {
		t.Fatalf("check capability after rebuild: %v", err)
	}
	if !ok {
		t.Fatalf("expected capability to be available after projection rebuild")
	}
}
