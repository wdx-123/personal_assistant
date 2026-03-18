package system

import (
	"context"
	"testing"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

func TestUserServiceGetUserRoleMatrixLevels(t *testing.T) {
	ctx := context.Background()

	t.Run("super admin operator", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		operator := createUser(t, env, "3001")
		target := createUser(t, env, "3002")
		seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

		superAdminRole, orgAdminRole, memberRole, customRole := seedMatrixRoles(t, env)
		assignGlobalRoleByID(t, env, operator.ID, superAdminRole.ID)

		matrix, err := env.userService.GetUserRoleMatrix(ctx, operator.ID, target.ID, org.ID)
		if err != nil {
			t.Fatalf("GetUserRoleMatrix() error = %v", err)
		}
		if matrix.OperatorMatrixLevel != string(userRoleMatrixLevelSuperAdmin) {
			t.Fatalf("operator matrix level = %s, want %s", matrix.OperatorMatrixLevel, userRoleMatrixLevelSuperAdmin)
		}
		assertRoleMatrixOrder(t, matrix, []string{superAdminRole.Code, orgAdminRole.Code, memberRole.Code, customRole.Code})
		assertRoleMatrixItem(t, matrix, superAdminRole.Code, false, userRoleMatrixDisabledReasonGlobalRoleOnly)
		assertRoleMatrixItem(t, matrix, orgAdminRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, memberRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, customRole.Code, true, "")
	})

	t.Run("owner without explicit org admin role", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		target := createUser(t, env, "3003")
		seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

		_, orgAdminRole, memberRole, customRole := seedMatrixRoles(t, env)

		matrix, err := env.userService.GetUserRoleMatrix(ctx, org.OwnerID, target.ID, org.ID)
		if err != nil {
			t.Fatalf("GetUserRoleMatrix() error = %v", err)
		}
		if matrix.OperatorMatrixLevel != string(userRoleMatrixLevelOrgAdmin) {
			t.Fatalf("operator matrix level = %s, want %s", matrix.OperatorMatrixLevel, userRoleMatrixLevelOrgAdmin)
		}
		assertRoleMatrixItem(t, matrix, orgAdminRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, memberRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, customRole.Code, true, "")
	})

	t.Run("org admin operator", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		operator := createUser(t, env, "3004")
		target := createUser(t, env, "3005")
		seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

		superAdminRole, orgAdminRole, memberRole, customRole := seedMatrixRoles(t, env)
		assignUserRole(t, env, operator.ID, org.ID, orgAdminRole.ID)
		grantOrgCapability(t, env, operator.ID, org.ID, "org_admin_operator", consts.CapabilityCodeOrgMemberAssignRole)

		matrix, err := env.userService.GetUserRoleMatrix(ctx, operator.ID, target.ID, org.ID)
		if err != nil {
			t.Fatalf("GetUserRoleMatrix() error = %v", err)
		}
		if matrix.OperatorMatrixLevel != string(userRoleMatrixLevelOrgAdmin) {
			t.Fatalf("operator matrix level = %s, want %s", matrix.OperatorMatrixLevel, userRoleMatrixLevelOrgAdmin)
		}
		assertRoleMatrixItem(t, matrix, superAdminRole.Code, false, userRoleMatrixDisabledReasonGlobalRoleOnly)
		assertRoleMatrixItem(t, matrix, orgAdminRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, memberRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, customRole.Code, true, "")
	})

	t.Run("member tier operator", func(t *testing.T) {
		env := newAuthorizationTestEnv(t)
		org := createOrg(t, env, 11)
		operator := createUser(t, env, "3006")
		target := createUser(t, env, "3007")
		seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

		superAdminRole, orgAdminRole, memberRole, customRole := seedMatrixRoles(t, env)
		assignUserRole(t, env, operator.ID, org.ID, customRole.ID)
		grantOrgCapability(t, env, operator.ID, org.ID, "custom_operator", consts.CapabilityCodeOrgMemberAssignRole)

		matrix, err := env.userService.GetUserRoleMatrix(ctx, operator.ID, target.ID, org.ID)
		if err != nil {
			t.Fatalf("GetUserRoleMatrix() error = %v", err)
		}
		if matrix.OperatorMatrixLevel != string(userRoleMatrixLevelMember) {
			t.Fatalf("operator matrix level = %s, want %s", matrix.OperatorMatrixLevel, userRoleMatrixLevelMember)
		}
		assertRoleMatrixItem(t, matrix, superAdminRole.Code, false, userRoleMatrixDisabledReasonGlobalRoleOnly)
		assertRoleMatrixItem(t, matrix, orgAdminRole.Code, false, userRoleMatrixDisabledReasonHigherMatrixLevel)
		assertRoleMatrixItem(t, matrix, memberRole.Code, true, "")
		assertRoleMatrixItem(t, matrix, customRole.Code, true, "")
	})
}

func TestUserServiceRoleMatrixRequiresCapabilityForNonPrivilegedOperator(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	operator := createUser(t, env, "3010")
	target := createUser(t, env, "3011")
	seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)
	_, _, memberRole, _ := seedMatrixRoles(t, env)

	_, err := env.userService.GetUserRoleMatrix(ctx, operator.ID, target.ID, org.ID)
	assertBizCode(t, err, bizerrors.CodePermissionDenied)

	err = env.userService.AssignRole(ctx, operator.ID, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{memberRole.ID},
	})
	assertBizCode(t, err, bizerrors.CodePermissionDenied)
}

func TestUserServiceRoleMatrixRejectsInactiveTarget(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	target := createUser(t, env, "3012")
	seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusRemoved)
	_, _, memberRole, _ := seedMatrixRoles(t, env)

	_, err := env.userService.GetUserRoleMatrix(ctx, org.OwnerID, target.ID, org.ID)
	assertBizCode(t, err, bizerrors.CodeOrgMemberStatusConflict)

	err = env.userService.AssignRole(ctx, org.OwnerID, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{memberRole.ID},
	})
	assertBizCode(t, err, bizerrors.CodeOrgMemberStatusConflict)
}

func TestUserServiceAssignRoleRejectsHigherMatrixLevelAndPreservesBindings(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	operator := createUser(t, env, "3013")
	target := createUser(t, env, "3014")
	seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

	_, orgAdminRole, memberRole, customRole := seedMatrixRoles(t, env)
	assignUserRole(t, env, operator.ID, org.ID, customRole.ID)
	assignUserRole(t, env, target.ID, org.ID, memberRole.ID)
	grantOrgCapability(t, env, operator.ID, org.ID, "custom_operator", consts.CapabilityCodeOrgMemberAssignRole)

	err := env.userService.AssignRole(ctx, operator.ID, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{memberRole.ID, orgAdminRole.ID},
	})
	assertBizCode(t, err, bizerrors.CodePermissionDenied)

	roles, getErr := env.userService.GetUserRoles(ctx, target.ID, org.ID)
	if getErr != nil {
		t.Fatalf("GetUserRoles() error = %v", getErr)
	}
	if len(roles) != 1 || roles[0].Code != memberRole.Code {
		t.Fatalf("target roles changed unexpectedly: %+v", roles)
	}
}

func TestUserServiceAssignRoleRejectsUnavailableRoleIDs(t *testing.T) {
	ctx := context.Background()
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	target := createUser(t, env, "3015")
	seedOrgMember(t, env, org.ID, target.ID, consts.OrgMemberStatusActive)

	superAdminRole, _, memberRole, _ := seedMatrixRoles(t, env)
	disabledRole := createRole(t, env, "disabled_operator_role")
	deletedRole := createRole(t, env, "deleted_operator_role")
	if err := env.db.Model(&entity.Role{}).Where("id = ?", disabledRole.ID).Update("status", 0).Error; err != nil {
		t.Fatalf("disable role: %v", err)
	}
	if err := env.db.Delete(&entity.Role{}, deletedRole.ID).Error; err != nil {
		t.Fatalf("delete role: %v", err)
	}

	cases := []struct {
		name    string
		roleIDs []uint
		want    bizerrors.BizCode
	}{
		{name: "super admin role", roleIDs: []uint{superAdminRole.ID}, want: bizerrors.CodePermissionDenied},
		{name: "disabled role", roleIDs: []uint{disabledRole.ID}, want: bizerrors.CodeRoleNotFound},
		{name: "deleted role", roleIDs: []uint{deletedRole.ID}, want: bizerrors.CodeRoleNotFound},
		{name: "invalid role id", roleIDs: []uint{999999}, want: bizerrors.CodeRoleNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := env.userService.AssignRole(ctx, org.OwnerID, &request.AssignUserRoleReq{
				UserID:  target.ID,
				OrgID:   org.ID,
				RoleIDs: tc.roleIDs,
			})
			assertBizCode(t, err, tc.want)
		})
	}

	if err := env.userService.AssignRole(ctx, org.OwnerID, &request.AssignUserRoleReq{
		UserID:  target.ID,
		OrgID:   org.ID,
		RoleIDs: []uint{memberRole.ID},
	}); err != nil {
		t.Fatalf("AssignRole() with valid member role error = %v", err)
	}
}

func seedMatrixRoles(
	t *testing.T,
	env *authorizationTestEnv,
) (*entity.Role, *entity.Role, *entity.Role, *entity.Role) {
	t.Helper()
	return createRole(t, env, consts.RoleCodeSuperAdmin),
		createRole(t, env, consts.RoleCodeOrgAdmin),
		createRole(t, env, consts.RoleCodeMember),
		createRole(t, env, "custom_matrix_role")
}

func assignGlobalRoleByID(t *testing.T, env *authorizationTestEnv, userID, roleID uint) {
	t.Helper()
	if err := env.db.Create(&entity.UserOrgRole{UserID: userID, OrgID: 0, RoleID: roleID}).Error; err != nil {
		t.Fatalf("assign global role: %v", err)
	}
}

func assertRoleMatrixOrder(t *testing.T, matrix *resp.UserRoleMatrixItem, wantCodes []string) {
	t.Helper()
	if len(matrix.Roles) < len(wantCodes) {
		t.Fatalf("matrix roles len = %d, want at least %d", len(matrix.Roles), len(wantCodes))
	}
	for idx, wantCode := range wantCodes {
		if matrix.Roles[idx].Code != wantCode {
			t.Fatalf("matrix role[%d] code = %s, want %s", idx, matrix.Roles[idx].Code, wantCode)
		}
	}
}

func assertRoleMatrixItem(
	t *testing.T,
	matrix *resp.UserRoleMatrixItem,
	roleCode string,
	assignable bool,
	disabledReason string,
) {
	t.Helper()
	for _, item := range matrix.Roles {
		if item.Code != roleCode {
			continue
		}
		if item.Assignable != assignable {
			t.Fatalf("role %s assignable = %v, want %v", roleCode, item.Assignable, assignable)
		}
		if item.DisabledReason != disabledReason {
			t.Fatalf("role %s disabled_reason = %s, want %s", roleCode, item.DisabledReason, disabledReason)
		}
		return
	}
	t.Fatalf("role %s not found in matrix", roleCode)
}
