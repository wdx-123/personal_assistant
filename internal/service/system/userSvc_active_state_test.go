package system

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

func TestUpdateUserStatusCachesInactiveState(t *testing.T) {
	setupServiceRedis(t)

	userRepo := &stubUserRepository{
		user: &entity.User{MODEL: entity.MODEL{ID: 7}, Status: consts.UserStatusActive},
	}
	svc := &UserService{
		userRepo:      userRepo,
		orgMemberRepo: &stubOrgMemberRepository{orgIDs: []uint{}},
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
	if len(userRepo.cacheWrites) != 1 || userRepo.cacheWrites[0] {
		t.Fatalf("cacheWrites = %v, want [false]", userRepo.cacheWrites)
	}
}

func TestUpdateUserStatusCachesActiveState(t *testing.T) {
	setupServiceRedis(t)

	userRepo := &stubUserRepository{
		user: &entity.User{MODEL: entity.MODEL{ID: 8}, Status: consts.UserStatusDisabled},
	}
	svc := &UserService{userRepo: userRepo}

	err := svc.UpdateUserStatus(context.Background(), 99, 8, &request.AdminUpdateUserStatusReq{
		Status: "active",
	})
	if err != nil {
		t.Fatalf("UpdateUserStatus() error = %v", err)
	}
	if userRepo.updatedStatus != consts.UserStatusActive {
		t.Fatalf("updated status = %v, want %v", userRepo.updatedStatus, consts.UserStatusActive)
	}
	if len(userRepo.cacheWrites) != 1 || !userRepo.cacheWrites[0] {
		t.Fatalf("cacheWrites = %v, want [true]", userRepo.cacheWrites)
	}
}

func TestUpdateUserStatusReturnsBizErrorWhenCacheWriteFails(t *testing.T) {
	setupServiceRedis(t)

	userRepo := &stubUserRepository{
		user:     &entity.User{MODEL: entity.MODEL{ID: 9}, Status: consts.UserStatusActive},
		cacheErr: stderrors.New("redis down"),
	}
	svc := &UserService{
		userRepo:      userRepo,
		orgMemberRepo: &stubOrgMemberRepository{orgIDs: []uint{}},
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
	if bizErr == nil || bizErr.Code != bizerrors.CodeRedisError {
		t.Fatalf("UpdateUserStatus() bizErr = %#v, want code %v", bizErr, bizerrors.CodeRedisError)
	}
}

func TestCleanupDisabledUsersCachesInactiveState(t *testing.T) {
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
	svc := &UserService{
		txRunner:      txRunner,
		userRepo:      userRepo,
		orgMemberRepo: orgMemberRepo,
		roleRepo:      roleRepo,
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
	if len(userRepo.cacheWrites) != 1 || userRepo.cacheWrites[0] {
		t.Fatalf("cacheWrites = %v, want [false]", userRepo.cacheWrites)
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
	cacheWrites       []bool
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
	r.cacheWrites = append(r.cacheWrites, active)
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

var _ repository.TxRunner = (*stubTxRunner)(nil)

func setupServiceRedis(t *testing.T) {
	t.Helper()

	srv, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	oldRedis := global.Redis
	global.Redis = client
	t.Cleanup(func() {
		global.Redis = oldRedis
		_ = client.Close()
		srv.Close()
	})
}
