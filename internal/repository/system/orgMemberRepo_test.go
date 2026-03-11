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

func TestOrgMemberRepositoryCountActiveMembersByOrgIDs(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&entity.OrgMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	repo := NewOrgMemberRepository(db)
	seedRepoOrgMember(t, db, 1, 101, consts.OrgMemberStatusActive)
	seedRepoOrgMember(t, db, 1, 102, consts.OrgMemberStatusActive)
	seedRepoOrgMember(t, db, 1, 103, consts.OrgMemberStatusLeft)
	seedRepoOrgMember(t, db, 2, 201, consts.OrgMemberStatusRemoved)
	seedRepoOrgMember(t, db, 2, 202, consts.OrgMemberStatusActive)

	counts, err := repo.CountActiveMembersByOrgIDs(context.Background(), []uint{1, 2, 3})
	if err != nil {
		t.Fatalf("CountActiveMembersByOrgIDs() error = %v", err)
	}
	if counts[1] != 2 {
		t.Fatalf("org 1 count = %d, want 2", counts[1])
	}
	if counts[2] != 1 {
		t.Fatalf("org 2 count = %d, want 1", counts[2])
	}
	if counts[3] != 0 {
		t.Fatalf("org 3 count = %d, want 0", counts[3])
	}
}

func seedRepoOrgMember(
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
