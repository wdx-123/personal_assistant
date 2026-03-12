package system

import (
	"context"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

func TestOrgServiceJoinOrgByInviteCodeAssignsConfiguredDefaultRole(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	user := createUser(t, env, "2001")
	org := createOrg(t, env, 81)
	defaultRole := createRole(t, env, "observer")

	oldDefaultRoleCode := global.Config.System.DefaultRoleCode
	global.Config.System.DefaultRoleCode = defaultRole.Code
	t.Cleanup(func() {
		global.Config.System.DefaultRoleCode = oldDefaultRoleCode
	})

	if err := env.orgService.JoinOrgByInviteCode(ctx, user.ID, org.Code); err != nil {
		t.Fatalf("JoinOrgByInviteCode() error = %v", err)
	}

	roles, err := env.userService.GetUserRoles(ctx, user.ID, org.ID)
	if err != nil {
		t.Fatalf("GetUserRoles() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("roles len = %d, want 1", len(roles))
	}
	if roles[0].Code != defaultRole.Code {
		t.Fatalf("role code = %s, want %s", roles[0].Code, defaultRole.Code)
	}

	var refreshed entity.User
	if err := env.db.First(&refreshed, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if refreshed.CurrentOrgID == nil || *refreshed.CurrentOrgID != org.ID {
		t.Fatalf("current_org_id = %v, want %d", refreshed.CurrentOrgID, org.ID)
	}
}

func TestOrgServiceRecoverMemberFallsBackToBuiltinMemberRole(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	user := createUser(t, env, "2002")
	org := createOrg(t, env, 91)
	memberRole := createRole(t, env, consts.RoleCodeMember)
	legacyRole := createRole(t, env, "legacy_role")
	seedOrgMember(t, env, org.ID, user.ID, consts.OrgMemberStatusRemoved)
	assignUserRole(t, env, user.ID, org.ID, legacyRole.ID)

	oldDefaultRoleCode := global.Config.System.DefaultRoleCode
	global.Config.System.DefaultRoleCode = "missing_default_role"
	t.Cleanup(func() {
		global.Config.System.DefaultRoleCode = oldDefaultRoleCode
	})

	if err := env.orgService.RecoverMember(ctx, org.OwnerID, org.ID, user.ID, "recover"); err != nil {
		t.Fatalf("RecoverMember() error = %v", err)
	}

	roles, err := env.userService.GetUserRoles(ctx, user.ID, org.ID)
	if err != nil {
		t.Fatalf("GetUserRoles() error = %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("roles len = %d, want 1", len(roles))
	}
	if roles[0].Code != memberRole.Code {
		t.Fatalf("role code = %s, want %s", roles[0].Code, memberRole.Code)
	}
}
