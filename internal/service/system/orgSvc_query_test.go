package system

import (
	"context"
	"testing"
	"time"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	bizerrors "personal_assistant/pkg/errors"
)

func TestOrgServiceGetOrgListReturnsActiveMemberCountForPagedQuery(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	orgA := createOrg(t, env, 11)
	orgB := createOrg(t, env, 22)

	seedOrgMember(t, env, orgA.ID, 11, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgA.ID, 12, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgA.ID, 13, consts.OrgMemberStatusLeft)
	seedOrgMember(t, env, orgA.ID, 14, consts.OrgMemberStatusRemoved)
	seedOrgMember(t, env, orgB.ID, 22, consts.OrgMemberStatusActive)

	items, total, err := env.orgService.GetOrgList(context.Background(), 1, 10, "")
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
	if counts[orgA.ID] != 2 {
		t.Fatalf("orgA member_count = %d, want 2", counts[orgA.ID])
	}
	if counts[orgB.ID] != 1 {
		t.Fatalf("orgB member_count = %d, want 1", counts[orgB.ID])
	}
}

func TestOrgServiceGetOrgListReturnsMemberCountForUnpagedKeywordQuery(t *testing.T) {
	env := newAuthorizationTestEnv(t)
	orgMatch := createOrg(t, env, 11)
	orgOther := createOrg(t, env, 22)

	orgMatch.Name = "Alpha Team"
	if err := env.db.Save(orgMatch).Error; err != nil {
		t.Fatalf("save matched org: %v", err)
	}
	orgOther.Name = "Beta Team"
	if err := env.db.Save(orgOther).Error; err != nil {
		t.Fatalf("save other org: %v", err)
	}

	seedOrgMember(t, env, orgMatch.ID, 11, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgMatch.ID, 12, consts.OrgMemberStatusActive)
	seedOrgMember(t, env, orgOther.ID, 22, consts.OrgMemberStatusActive)

	items, total, err := env.orgService.GetOrgList(context.Background(), 0, 0, "Alpha")
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
