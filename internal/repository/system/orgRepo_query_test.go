package system

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

func TestOrgRepositoryGetVisibleOrgListByUserIDWithKeywordFiltersActiveMembership(t *testing.T) {
	db := newOrgRepositoryTestDB(t)
	repo := NewOrgRepository(db)
	userID := uint(100)

	visible := seedRepoOrg(t, db, 11, "Alpha Team")
	left := seedRepoOrg(t, db, 22, "Alpha Former")
	other := seedRepoOrg(t, db, 33, "Alpha External")

	seedRepoVisibleOrgMember(t, db, visible.ID, userID, consts.OrgMemberStatusActive)
	seedRepoVisibleOrgMember(t, db, left.ID, userID, consts.OrgMemberStatusLeft)
	seedRepoVisibleOrgMember(t, db, other.ID, 200, consts.OrgMemberStatusActive)

	items, total, err := repo.GetVisibleOrgListByUserIDWithKeyword(context.Background(), userID, 0, 0, "Alpha")
	if err != nil {
		t.Fatalf("GetVisibleOrgListByUserIDWithKeyword() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ID != visible.ID {
		t.Fatalf("org id = %d, want %d", items[0].ID, visible.ID)
	}
}

func TestOrgRepositoryGetVisibleOrgListByUserIDWithKeywordPaginates(t *testing.T) {
	db := newOrgRepositoryTestDB(t)
	repo := NewOrgRepository(db)
	userID := uint(101)

	orgA := seedRepoOrg(t, db, 11, "Team A")
	orgB := seedRepoOrg(t, db, 22, "Team B")
	orgC := seedRepoOrg(t, db, 33, "Team C")

	seedRepoVisibleOrgMember(t, db, orgA.ID, userID, consts.OrgMemberStatusActive)
	seedRepoVisibleOrgMember(t, db, orgB.ID, userID, consts.OrgMemberStatusActive)
	seedRepoVisibleOrgMember(t, db, orgC.ID, userID, consts.OrgMemberStatusActive)

	items, total, err := repo.GetVisibleOrgListByUserIDWithKeyword(context.Background(), userID, 1, 2, "")
	if err != nil {
		t.Fatalf("GetVisibleOrgListByUserIDWithKeyword() error = %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != orgC.ID || items[1].ID != orgB.ID {
		t.Fatalf("paged order = [%d %d], want [%d %d]", items[0].ID, items[1].ID, orgC.ID, orgB.ID)
	}
}

func newOrgRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.Org{}, &entity.OrgMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedRepoOrg(t *testing.T, db *gorm.DB, ownerID uint, name string) *entity.Org {
	t.Helper()

	org := &entity.Org{
		Name:    name,
		Code:    fmt.Sprintf("ORG-%d-%d", ownerID, time.Now().UnixNano()),
		OwnerID: ownerID,
	}
	if err := db.Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	return org
}

func seedRepoVisibleOrgMember(
	t *testing.T,
	db *gorm.DB,
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
	if err := db.Create(member).Error; err != nil {
		t.Fatalf("create org member: %v", err)
	}
}
