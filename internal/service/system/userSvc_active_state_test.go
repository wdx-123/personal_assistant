package system

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"

	"go.uber.org/zap"
)

func TestUpdateUserStatusPublishesInactiveProjectionEvent(t *testing.T) {
	userRepo := &stubUserRepository{
		user: &entity.User{MODEL: entity.MODEL{ID: 7}, Status: consts.UserStatusActive},
	}
	publisher := &stubCacheProjectionPublisher{}
	svc := &UserService{
		txRunner:                 &stubTxRunner{},
		userRepo:                 userRepo,
		orgMemberRepo:            &stubOrgMemberRepository{orgIDs: []uint{}},
		cacheProjectionPublisher: publisher,
	}

	err := svc.UpdateUserStatus(context.Background(), 99, 7, &request.AdminUpdateUserStatusReq{
		Status: "disabled",
		Reason: "manual disable",
	})
	if err != nil {
		t.Fatalf("UpdateUserStatus() error = %v", err)
	}
	if userRepo.updatedStatus != consts.UserStatusDisabled {
		t.Fatalf("updated status = %v, want %v", userRepo.updatedStatus, consts.UserStatusDisabled)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].Kind != eventdto.CacheProjectionKindUserSnapshotChanged {
		t.Fatalf("event kind = %q, want %q", publisher.events[0].Kind, eventdto.CacheProjectionKindUserSnapshotChanged)
	}
	if publisher.events[0].UserID != 7 {
		t.Fatalf("event user id = %d, want 7", publisher.events[0].UserID)
	}
}

func TestUpdateUserStatusPublishesActiveProjectionEvent(t *testing.T) {
	userRepo := &stubUserRepository{
		user: &entity.User{MODEL: entity.MODEL{ID: 8}, Status: consts.UserStatusDisabled},
	}
	publisher := &stubCacheProjectionPublisher{}
	svc := &UserService{
		txRunner:                 &stubTxRunner{},
		userRepo:                 userRepo,
		cacheProjectionPublisher: publisher,
	}

	err := svc.UpdateUserStatus(context.Background(), 99, 8, &request.AdminUpdateUserStatusReq{
		Status: "active",
	})
	if err != nil {
		t.Fatalf("UpdateUserStatus() error = %v", err)
	}
	if userRepo.updatedStatus != consts.UserStatusActive {
		t.Fatalf("updated status = %v, want %v", userRepo.updatedStatus, consts.UserStatusActive)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].Kind != eventdto.CacheProjectionKindUserSnapshotChanged {
		t.Fatalf("event kind = %q, want %q", publisher.events[0].Kind, eventdto.CacheProjectionKindUserSnapshotChanged)
	}
}

func TestUpdateUserStatusReturnsBizErrorWhenProjectionPublishFails(t *testing.T) {
	userRepo := &stubUserRepository{
		user: &entity.User{MODEL: entity.MODEL{ID: 9}, Status: consts.UserStatusActive},
	}
	publisher := &stubCacheProjectionPublisher{publishErr: stderrors.New("outbox down")}
	svc := &UserService{
		txRunner:                 &stubTxRunner{},
		userRepo:                 userRepo,
		orgMemberRepo:            &stubOrgMemberRepository{orgIDs: []uint{}},
		cacheProjectionPublisher: publisher,
	}
	oldLog := global.Log
	global.Log = zap.NewNop()
	t.Cleanup(func() {
		global.Log = oldLog
	})

	err := svc.UpdateUserStatus(context.Background(), 99, 9, &request.AdminUpdateUserStatusReq{
		Status: "disabled",
		Reason: "manual disable",
	})
	if err == nil {
		t.Fatal("UpdateUserStatus() error = nil, want redis biz error")
	}

	bizErr := bizerrors.FromError(err)
	if bizErr == nil || bizErr.Code != bizerrors.CodeDBError {
		t.Fatalf("UpdateUserStatus() bizErr = %#v, want code %v", bizErr, bizerrors.CodeDBError)
	}
}

func TestCleanupDisabledUsersPublishesDeletedProjectionEvent(t *testing.T) {
	now := time.Now().Add(-48 * time.Hour)
	userRepo := &stubUserRepository{
		listDisabledUsers: []*entity.User{
			{
				MODEL:      entity.MODEL{ID: 10},
				Status:     consts.UserStatusDisabled,
				DisabledAt: &now,
			},
		},
	}
	orgMemberRepo := &stubOrgMemberRepository{orgIDs: []uint{}}
	roleRepo := &stubRoleRepository{}
	txRunner := &stubTxRunner{}
	publisher := &stubCacheProjectionPublisher{}
	svc := &UserService{
		txRunner:                 txRunner,
		userRepo:                 userRepo,
		orgMemberRepo:            orgMemberRepo,
		roleRepo:                 roleRepo,
		cacheProjectionPublisher: publisher,
	}

	oldConfig := global.Config
	oldLog := global.Log
	global.Config = &config.Config{
		Task: config.Task{
			DisabledUserCleanupEnabled: true,
			DisabledUserRetentionDays:  1,
		},
	}
	global.Log = zap.NewNop()
	t.Cleanup(func() {
		global.Config = oldConfig
		global.Log = oldLog
	})

	cleaned, err := svc.CleanupDisabledUsers(context.Background())
	if err != nil {
		t.Fatalf("CleanupDisabledUsers() error = %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("CleanupDisabledUsers() cleaned = %d, want 1", cleaned)
	}
	if len(userRepo.softDeleted) != 1 || userRepo.softDeleted[0] != 10 {
		t.Fatalf("softDeleted = %v, want [10]", userRepo.softDeleted)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("published events = %d, want 1", len(publisher.events))
	}
	if publisher.events[0].Kind != eventdto.CacheProjectionKindUserDeleted {
		t.Fatalf("event kind = %q, want %q", publisher.events[0].Kind, eventdto.CacheProjectionKindUserDeleted)
	}
	if publisher.events[0].UserID != 10 {
		t.Fatalf("event user id = %d, want 10", publisher.events[0].UserID)
	}
	if txRunner.calls != 1 {
		t.Fatalf("txRunner calls = %d, want 1", txRunner.calls)
	}
}

type stubTxRunner struct {
	calls int
	err   error
}

func (r *stubTxRunner) InTx(_ context.Context, fn func(tx any) error) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	return fn(nil)
}

type stubUserRepository struct {
	interfaces.UserRepository
	user              *entity.User
	getByIDErr        error
	updateErr         error
	cacheErr          error
	listDisabledUsers []*entity.User
	listDisabledErr   error
	updatedStatus     consts.UserStatus
	updatedDisabledBy *uint
	updatedReason     string
	softDeleted       []uint
}

func (r *stubUserRepository) GetByID(_ context.Context, _ uint) (*entity.User, error) {
	return r.user, r.getByIDErr
}

func (r *stubUserRepository) UpdateUserStatus(
	_ context.Context,
	_ uint,
	status consts.UserStatus,
	disabledBy *uint,
	disabledReason string,
) error {
	r.updatedStatus = status
	r.updatedDisabledBy = disabledBy
	r.updatedReason = disabledReason
	return r.updateErr
}

func (r *stubUserRepository) CacheActiveState(_ context.Context, _ uint, active bool) error {
	return r.cacheErr
}

func (r *stubUserRepository) ListDisabledUsersBefore(_ context.Context, _ time.Time, _ int) ([]*entity.User, error) {
	return r.listDisabledUsers, r.listDisabledErr
}

func (r *stubUserRepository) SoftDeleteAndAnonymize(_ context.Context, userID uint) error {
	r.softDeleted = append(r.softDeleted, userID)
	return nil
}

func (r *stubUserRepository) WithTx(_ any) interfaces.UserRepository {
	return r
}

type stubOrgMemberRepository struct {
	interfaces.OrgMemberRepository
	orgIDs []uint
}

func (r *stubOrgMemberRepository) ListActiveOrgIDsByUser(_ context.Context, _ uint) ([]uint, error) {
	return append([]uint(nil), r.orgIDs...), nil
}

func (r *stubOrgMemberRepository) WithTx(_ any) interfaces.OrgMemberRepository {
	return r
}

type stubRoleRepository struct {
	interfaces.RoleRepository
	deleteCalls int
}

func (r *stubRoleRepository) DeleteUserOrgRoles(_ context.Context, _, _ uint) error {
	r.deleteCalls++
	return nil
}

func (r *stubRoleRepository) WithTx(_ any) interfaces.RoleRepository {
	return r
}

type stubCacheProjectionPublisher struct {
	events     []*eventdto.CacheProjectionEvent
	publishErr error
}

func (p *stubCacheProjectionPublisher) Publish(_ context.Context, event *eventdto.CacheProjectionEvent) error {
	if p.publishErr != nil {
		return p.publishErr
	}
	p.events = append(p.events, event)
	return nil
}

func (p *stubCacheProjectionPublisher) PublishInTx(_ context.Context, _ any, event *eventdto.CacheProjectionEvent) error {
	if p.publishErr != nil {
		return p.publishErr
	}
	p.events = append(p.events, event)
	return nil
}

var _ repository.TxRunner = (*stubTxRunner)(nil)
