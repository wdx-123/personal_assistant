package system

import (
	"context"
	"testing"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

func TestOrgServiceGetOrgQueriesIncludeBuiltinMetadata(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	owner := createUser(t, env, "3101")
	org := createOrg(t, env, owner.ID)
	builtinKey := consts.OrgBuiltinKeyAllMembers
	org.IsBuiltin = true
	org.BuiltinKey = &builtinKey
	if err := env.db.Save(org).Error; err != nil {
		t.Fatalf("save builtin org: %v", err)
	}
	seedOrgMember(t, env, org.ID, owner.ID, consts.OrgMemberStatusActive)

	items, total, err := env.orgService.GetOrgList(ctx, owner.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("GetOrgList() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("GetOrgList() returned total=%d len=%d, want 1/1", total, len(items))
	}
	if !items[0].IsBuiltin {
		t.Fatal("GetOrgList() is_builtin = false, want true")
	}
	if items[0].BuiltinKey == nil || *items[0].BuiltinKey != builtinKey {
		t.Fatalf("GetOrgList() builtin_key = %v, want %q", items[0].BuiltinKey, builtinKey)
	}

	detail, err := env.orgService.GetOrgDetail(ctx, owner.ID, org.ID)
	if err != nil {
		t.Fatalf("GetOrgDetail() error = %v", err)
	}
	if !detail.IsBuiltin {
		t.Fatal("GetOrgDetail() is_builtin = false, want true")
	}
	if detail.BuiltinKey == nil || *detail.BuiltinKey != builtinKey {
		t.Fatalf("GetOrgDetail() builtin_key = %v, want %q", detail.BuiltinKey, builtinKey)
	}
}

func TestOrgServiceUpdateOrgAllowsOwnerForRegularOrg(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	owner := createUser(t, env, "3102")
	org := createOrg(t, env, owner.ID)

	name := "更新后的组织"
	description := "新的组织描述"
	code := "ORG-3102-NEW"
	req := &request.UpdateOrgReq{
		Name:        &name,
		Description: &description,
		Code:        &code,
	}

	if err := env.orgService.UpdateOrg(ctx, owner.ID, org.ID, req); err != nil {
		t.Fatalf("UpdateOrg() error = %v", err)
	}

	var refreshed entity.Org
	if err := env.db.First(&refreshed, org.ID).Error; err != nil {
		t.Fatalf("reload org: %v", err)
	}
	if refreshed.Name != name || refreshed.Description != description || refreshed.Code != code {
		t.Fatalf("updated org = %+v, want name=%q description=%q code=%q", refreshed, name, description, code)
	}
}

func TestOrgServiceUpdateOrgRejectsBuiltinAllMembersForNonSuperAdmin(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	owner := createUser(t, env, "3103")
	org := createBuiltinAllMembersOrg(t, env, owner.ID)
	name := "禁止修改"

	err := env.orgService.UpdateOrg(ctx, owner.ID, org.ID, &request.UpdateOrgReq{Name: &name})
	assertBizCode(t, err, bizerrors.CodeOrgBuiltinProtected)
}

func TestOrgServiceUpdateOrgAllowsBuiltinAllMembersForSuperAdmin(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	owner := createUser(t, env, "3104")
	admin := createUser(t, env, "3105")
	grantGlobalRole(t, env, admin.ID, consts.RoleCodeSuperAdmin)
	org := createBuiltinAllMembersOrg(t, env, owner.ID)

	description := "超级管理员已更新"
	code := "ORG-3105-SUPER"
	req := &request.UpdateOrgReq{
		Description: &description,
		Code:        &code,
	}

	if err := env.orgService.UpdateOrg(ctx, admin.ID, org.ID, req); err != nil {
		t.Fatalf("UpdateOrg() error = %v", err)
	}

	var refreshed entity.Org
	if err := env.db.First(&refreshed, org.ID).Error; err != nil {
		t.Fatalf("reload org: %v", err)
	}
	if refreshed.Description != description || refreshed.Code != code {
		t.Fatalf("updated builtin org = %+v, want description=%q code=%q", refreshed, description, code)
	}
}

func TestOrgServiceBuiltinOrgProtectionStillBlocksDeleteAndMemberMutationForSuperAdmin(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)

	owner := createUser(t, env, "3106")
	admin := createUser(t, env, "3107")
	target := createUser(t, env, "3108")
	grantGlobalRole(t, env, admin.ID, consts.RoleCodeSuperAdmin)
	org := createBuiltinAllMembersOrg(t, env, owner.ID)
	seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

	err := env.orgService.DeleteOrg(ctx, admin.ID, org.ID, true)
	assertBizCode(t, err, bizerrors.CodeOrgBuiltinProtected)

	err = env.orgService.KickMember(ctx, admin.ID, org.ID, target.ID, "kick")
	assertBizCode(t, err, bizerrors.CodeOrgBuiltinProtected)
}

func createBuiltinAllMembersOrg(t *testing.T, env *authorizationTestEnv, ownerID uint) *entity.Org {
	t.Helper()

	org := createOrg(t, env, ownerID)
	builtinKey := consts.OrgBuiltinKeyAllMembers
	org.IsBuiltin = true
	org.BuiltinKey = &builtinKey
	if err := env.db.Save(org).Error; err != nil {
		t.Fatalf("save builtin org: %v", err)
	}
	return org
}
