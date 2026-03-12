package system

import (
	"context"
	"strings"
	"testing"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestGetRankingListCurrentOrgFallsBackToAllMembersScope(t *testing.T) {
	setupRankingRedis(t)

	if err := global.Redis.ZAdd(context.Background(), rediskey.RankingAllMembersZSetKey("luogu"), &redis.Z{
		Score:  321,
		Member: "2",
	}).Err(); err != nil {
		t.Fatalf("seed ranking zset error = %v", err)
	}

	allMembersKey := consts.OrgBuiltinKeyAllMembers
	currentOrgID := uint(100)
	svc := &OJService{
		userRepo: &stubRankingUserRepository{
			users: map[uint]*entity.User{
				1: {
					MODEL:        entity.MODEL{ID: 1},
					Status:       consts.UserStatusActive,
					CurrentOrgID: &currentOrgID,
					CurrentOrg: &entity.Org{
						MODEL:      entity.MODEL{ID: currentOrgID},
						IsBuiltin:  true,
						BuiltinKey: &allMembersKey,
					},
				},
			},
		},
		roleRepo: &stubRankingRoleRepository{},
		rankingReadModelRepo: &stubRankingReadModelRepository{
			items: map[uint]*readmodel.Ranking{
				2: {
					UserID:          2,
					Username:        "alice",
					Avatar:          "base-avatar",
					Status:          consts.UserStatusActive,
					LuoguIdentifier: "lg-2",
					LuoguAvatar:     "luogu-avatar",
					LuoguScore:      321,
				},
			},
		},
	}

	out, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "luogu",
		Scope:    rankingScopeCurrentOrg,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetRankingList() error = %v", err)
	}
	if out.Total != 1 {
		t.Fatalf("total = %d, want 1", out.Total)
	}
	if len(out.List) != 1 {
		t.Fatalf("list len = %d, want 1", len(out.List))
	}
	if out.List[0].UserID != 2 {
		t.Fatalf("user id = %d, want 2", out.List[0].UserID)
	}
	if out.List[0].Avatar != "luogu-avatar" {
		t.Fatalf("avatar = %q, want %q", out.List[0].Avatar, "luogu-avatar")
	}

	values, err := global.Redis.HGetAll(context.Background(), rediskey.RankingUserHashKey(2)).Result()
	if err != nil {
		t.Fatalf("HGetAll() error = %v", err)
	}
	if strings.TrimSpace(values["username"]) != "alice" {
		t.Fatalf("cached username = %q, want %q", values["username"], "alice")
	}
}

func TestGetRankingListOrgScopeRequiresActiveMembership(t *testing.T) {
	setupRankingRedis(t)

	currentOrgID := uint(200)
	targetOrgID := uint(300)
	svc := &OJService{
		userRepo: &stubRankingUserRepository{
			users: map[uint]*entity.User{
				1: {
					MODEL:        entity.MODEL{ID: 1},
					Status:       consts.UserStatusActive,
					CurrentOrgID: &currentOrgID,
				},
			},
		},
		roleRepo:      &stubRankingRoleRepository{},
		orgMemberRepo: &stubRankingOrgMemberRepository{activeByOrg: map[uint]bool{targetOrgID: false}},
		orgRepo: &stubRankingOrgRepository{
			orgs: map[uint]*entity.Org{
				targetOrgID: {MODEL: entity.MODEL{ID: targetOrgID}, Name: "Target"},
			},
		},
	}

	_, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "leetcode",
		Scope:    rankingScopeOrg,
		OrgID:    &targetOrgID,
		Page:     1,
		PageSize: 10,
	})
	if err == nil {
		t.Fatal("GetRankingList() error = nil, want permission error")
	}
	if !strings.Contains(err.Error(), "organization not active") {
		t.Fatalf("error = %v, want organization not active", err)
	}
}

func setupRankingRedis(t *testing.T) {
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
		if err := client.Close(); err != nil {
			t.Errorf("client.Close() error = %v", err)
		}
		srv.Close()
	})
}

type stubRankingUserRepository struct {
	interfaces.UserRepository
	users map[uint]*entity.User
}

func (r *stubRankingUserRepository) GetByID(_ context.Context, userID uint) (*entity.User, error) {
	return r.users[userID], nil
}

type stubRankingRoleRepository struct {
	interfaces.RoleRepository
	roles []*entity.Role
}

func (r *stubRankingRoleRepository) GetUserGlobalRoles(_ context.Context, _ uint) ([]*entity.Role, error) {
	return r.roles, nil
}

type stubRankingOrgMemberRepository struct {
	interfaces.OrgMemberRepository
	activeByOrg map[uint]bool
}

func (r *stubRankingOrgMemberRepository) IsUserActiveInOrg(_ context.Context, _ uint, orgID uint) (bool, error) {
	return r.activeByOrg[orgID], nil
}

type stubRankingOrgRepository struct {
	interfaces.OrgRepository
	orgs map[uint]*entity.Org
}

func (r *stubRankingOrgRepository) GetByID(_ context.Context, orgID uint) (*entity.Org, error) {
	return r.orgs[orgID], nil
}

type stubRankingReadModelRepository struct {
	interfaces.RankingReadModelRepository
	items map[uint]*readmodel.Ranking
}

func (r *stubRankingReadModelRepository) GetByUserIDs(_ context.Context, userIDs []uint) ([]*readmodel.Ranking, error) {
	result := make([]*readmodel.Ranking, 0, len(userIDs))
	for _, userID := range userIDs {
		if item, ok := r.items[userID]; ok {
			result = append(result, item)
		}
	}
	return result, nil
}
