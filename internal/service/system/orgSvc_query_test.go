package system

import (
	"context"
	"testing"
	"time"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

func TestOrgServiceGetOrgListReturnsVisibleOrgsForRegularUserPagedQuery(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	requester := createUser(t, env, "00000010")
	orgA := createOrg(t, env, 11)
	orgB := createOrg(t, env, 22)
	orgHidden := createOrg(t, env, 33)

	seedOrgMember(t, env, orgA.ID, orgA.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgA.ID, requester.ID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgA.ID, 12, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgA.ID, 13, consts.OrgMemberStatusLeft)
	seedOrgMember(t, env, orgA.ID, 14, consts.OrgMemberStatusRemoved)
	seedOrgMember(t, env, orgB.ID, orgB.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgB.ID, requester.ID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgHidden.ID, orgHidden.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgHidden.ID, requester.ID, consts.OrgMemberStatusLeft)

	items, total, err := env.orgService.GetOrgList(context.Background(), requester.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("GetOrgList() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	counts := make(map[uint]int64, len(items))
	for _, item := range items {
		counts[item.ID] = item.MemberCount
	}
	if _, ok := counts[orgHidden.ID]; ok {
		t.Fatalf("hidden org %d should not be visible", orgHidden.ID)
	}
	if counts[orgA.ID] != 3 {
		t.Fatalf("orgA member_count = %d, want 3", counts[orgA.ID])
	}
	if counts[orgB.ID] != 2 {
		t.Fatalf("orgB member_count = %d, want 2", counts[orgB.ID])
	}
}

func TestOrgServiceGetOrgListReturnsVisibleOrgsForRegularUserUnpagedKeywordQuery(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	requester := createUser(t, env, "00000011")
	orgMatch := createOrg(t, env, 11)
	orgHidden := createOrg(t, env, 22)
	orgOther := createOrg(t, env, 33)

	orgMatch.Name = "Alpha Team"
	if err := env.db.Save(orgMatch).Error; err != nil {
		t.Fatalf("save matched org: %v", err)
	}
	orgHidden.Name = "Alpha Hidden"
	if err := env.db.Save(orgHidden).Error; err != nil {
		t.Fatalf("save hidden org: %v", err)
	}
	orgOther.Name = "Beta Team"
	if err := env.db.Save(orgOther).Error; err != nil {
		t.Fatalf("save other org: %v", err)
	}

	seedOrgMember(t, env, orgMatch.ID, orgMatch.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgMatch.ID, requester.ID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgHidden.ID, requester.ID, consts.OrgMemberStatusRemoved)
	seedOrgMember(t, env, orgOther.ID, requester.ID, consts.OrgMemberStatusActive)

	items, total, err := env.orgService.GetOrgList(context.Background(), requester.ID, 0, 0, "Alpha")
	if err != nil {
		t.Fatalf("GetOrgList() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("list len = %d, want 1", len(items))
	}
	if items[0].ID != orgMatch.ID {
		t.Fatalf("org id = %d, want %d", items[0].ID, orgMatch.ID)
	}
	if items[0].MemberCount != 2 {
		t.Fatalf("member_count = %d, want 2", items[0].MemberCount)
	}
}

func TestOrgServiceGetOrgListReturnsAllOrgsForSuperAdmin(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	admin := createUser(t, env, "00000012")
	grantGlobalRole(t, env, admin.ID, consts.RoleCodeSuperAdmin)
	orgA := createOrg(t, env, 11)
	orgB := createOrg(t, env, 22)
	orgC := createOrg(t, env, 33)

	seedOrgMember(t, env, orgA.ID, orgA.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgB.ID, orgB.OwnerID, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgC.ID, orgC.OwnerID, consts.OrgMemberStatusActive)

	items, total, err := env.orgService.GetOrgList(context.Background(), admin.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("GetOrgList() error = %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}
}

func TestOrgServiceGetOrgDetailReturnsMemberCountForAuthorizedUser(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)

	seedOrgMember(t, env, org.ID, 11, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, org.ID, 22, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, org.ID, 23, consts.OrgMemberStatusLeft)

	item, err := env.orgService.GetOrgDetail(context.Background(), 22, org.ID)
	if err != nil {
		t.Fatalf("GetOrgDetail() error = %v", err)
	}
	if item.MemberCount != 2 {
		t.Fatalf("member_count = %d, want 2", item.MemberCount)
	}
}

func TestOrgServiceGetOrgDetailRejectsUnauthorizedUser(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	org := createOrg(t, env, 11)
	seedOrgMember(t, env, org.ID, 11, consts.OrgMemberStatusActive)

	_, err := env.orgService.GetOrgDetail(context.Background(), 99, org.ID)
	assertBizCode(t, err, bizerrors.CodePermissionDenied)
}

func TestOrgServiceGetOrgDetailAllowsSuperAdminWithoutMembership(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	admin := createUser(t, env, "00000013")
	grantGlobalRole(t, env, admin.ID, consts.RoleCodeSuperAdmin)
	org := createOrg(t, env, 11)
	seedOrgMember(t, env, org.ID, org.OwnerID, consts.OrgMemberStatusActive)

	item, err := env.orgService.GetOrgDetail(context.Background(), admin.ID, org.ID)
	if err != nil {
		t.Fatalf("GetOrgDetail() error = %v", err)
	}
	if item.ID != org.ID {
		t.Fatalf("org id = %d, want %d", item.ID, org.ID)
	}
}

func seedOrgMember(
	t *testing.T,
	env *authorizationTestEnv,
	orgID, userID uint,
	status consts.OrgMemberStatus,
) {
	t.Helper()

	member := &entity.OrgMember{
		OrgID:        orgID,
		UserID:       userID,
		MemberStatus: status,
		JoinedAt:     time.Now(),
		JoinSource:   "test",
	}
	switch status {
	case consts.OrgMemberStatusLeft:
		now := time.Now()
		member.LeftAt = &now
	case consts.OrgMemberStatusRemoved:
		now := time.Now()
		member.RemovedAt = &now
	}
	if err := env.db.Create(member).Error; err != nil {
		t.Fatalf("create org member: %v", err)
	}
}
