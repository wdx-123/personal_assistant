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
	"personal_assistant/pkg/rankingcache"
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
	if out.List[0].CurrentOrg != nil {
		t.Fatalf("CurrentOrg = %+v, want nil for current org scope", out.List[0].CurrentOrg)
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

func TestGetRankingListLanqiaoUsesLanqiaoScore(t *testing.T) {
	setupRankingRedis(t)

	if err := global.Redis.ZAdd(context.Background(), rediskey.RankingAllMembersZSetKey("lanqiao"), &redis.Z{
		Score:  12,
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
					UserID:            2,
					Username:          "alice",
					Avatar:            "base-avatar",
					Status:            consts.UserStatusActive,
					LanqiaoIdentifier: "138****0000",
					LanqiaoScore:      12,
				},
			},
		},
	}

	out, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "lanqiao",
		Scope:    rankingScopeCurrentOrg,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetRankingList() error = %v", err)
	}
	if out.Total != 1 || len(out.List) != 1 {
		t.Fatalf("unexpected ranking result: total=%d len=%d", out.Total, len(out.List))
	}
	if out.List[0].TotalPassed != 12 {
		t.Fatalf("TotalPassed = %d, want 12", out.List[0].TotalPassed)
	}
	if out.List[0].PlatformDetails == nil || out.List[0].PlatformDetails.Lanqiao != 12 {
		t.Fatalf("PlatformDetails = %+v, want lanqiao score 12", out.List[0].PlatformDetails)
	}
	if out.List[0].Avatar != "base-avatar" {
		t.Fatalf("avatar = %q, want %q", out.List[0].Avatar, "base-avatar")
	}
	if out.List[0].CurrentOrg != nil {
		t.Fatalf("CurrentOrg = %+v, want nil for current org scope", out.List[0].CurrentOrg)
	}
}

func TestGetRankingListAllMembersIncludesCurrentOrg(t *testing.T) {
	setupRankingRedis(t)

	if err := global.Redis.ZAdd(context.Background(), rediskey.RankingAllMembersZSetKey("leetcode"), &redis.Z{
		Score:  88,
		Member: "2",
	}).Err(); err != nil {
		t.Fatalf("seed ranking zset error = %v", err)
	}

	allMembersKey := consts.OrgBuiltinKeyAllMembers
	currentOrgID := uint(100)
	memberOrgID := uint(201)
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
					UserID:             2,
					Username:           "alice",
					Avatar:             "base-avatar",
					Status:             consts.UserStatusActive,
					CurrentOrgID:       &memberOrgID,
					CurrentOrgName:     "算法一组",
					LeetcodeIdentifier: "alice-lc",
					LeetcodeAvatar:     "leetcode-avatar",
					LeetcodeScore:      88,
				},
			},
		},
	}

	out, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "leetcode",
		Scope:    rankingScopeAllMembers,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetRankingList() error = %v", err)
	}
	if out.Total != 1 || len(out.List) != 1 {
		t.Fatalf("unexpected ranking result: total=%d len=%d", out.Total, len(out.List))
	}
	if out.List[0].CurrentOrg == nil {
		t.Fatal("CurrentOrg = nil, want current org info")
	}
	if out.List[0].CurrentOrg.ID != memberOrgID || out.List[0].CurrentOrg.Name != "算法一组" {
		t.Fatalf("CurrentOrg = %+v, want id=%d name=%q", out.List[0].CurrentOrg, memberOrgID, "算法一组")
	}
}

func TestGetRankingListAllMembersSkipsEmptyCurrentOrg(t *testing.T) {
	setupRankingRedis(t)

	if err := global.Redis.ZAdd(context.Background(), rediskey.RankingAllMembersZSetKey("luogu"), &redis.Z{
		Score:  55,
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
					LuoguScore:      55,
				},
			},
		},
	}

	out, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "luogu",
		Scope:    rankingScopeAllMembers,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetRankingList() error = %v", err)
	}
	if out.Total != 1 || len(out.List) != 1 {
		t.Fatalf("unexpected ranking result: total=%d len=%d", out.Total, len(out.List))
	}
	if out.List[0].CurrentOrg != nil {
		t.Fatalf("CurrentOrg = %+v, want nil when user has no current org", out.List[0].CurrentOrg)
	}
}

func TestGetRankingListAllMembersBackfillsLegacyProjectionCache(t *testing.T) {
	setupRankingRedis(t)

	if err := global.Redis.ZAdd(context.Background(), rediskey.RankingAllMembersZSetKey("leetcode"), &redis.Z{
		Score:  88,
		Member: "2",
	}).Err(); err != nil {
		t.Fatalf("seed ranking zset error = %v", err)
	}
	if err := global.Redis.HSet(context.Background(), rediskey.RankingUserHashKey(2), map[string]any{
		"username":            "alice",
		"avatar":              "base-avatar",
		"active":              "1",
		"leetcode_identifier": "alice-lc",
		"leetcode_avatar":     "leetcode-avatar",
		"leetcode_score":      88,
		"luogu_identifier":    "",
		"luogu_avatar":        "",
		"luogu_score":         0,
		"lanqiao_identifier":  "",
		"lanqiao_avatar":      "",
		"lanqiao_score":       0,
	}).Err(); err != nil {
		t.Fatalf("seed legacy projection hash error = %v", err)
	}

	allMembersKey := consts.OrgBuiltinKeyAllMembers
	currentOrgID := uint(100)
	memberOrgID := uint(201)
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
					UserID:             2,
					Username:           "alice",
					Avatar:             "base-avatar",
					Status:             consts.UserStatusActive,
					CurrentOrgID:       &memberOrgID,
					CurrentOrgName:     "智能小组",
					LeetcodeIdentifier: "alice-lc",
					LeetcodeAvatar:     "leetcode-avatar",
					LeetcodeScore:      88,
				},
			},
		},
	}

	out, err := svc.GetRankingList(context.Background(), 1, &request.OJRankingListReq{
		Platform: "leetcode",
		Scope:    rankingScopeAllMembers,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetRankingList() error = %v", err)
	}
	if out.Total != 1 || len(out.List) != 1 {
		t.Fatalf("unexpected ranking result: total=%d len=%d", out.Total, len(out.List))
	}
	if out.List[0].CurrentOrg == nil || out.List[0].CurrentOrg.Name != "智能小组" {
		t.Fatalf("CurrentOrg = %+v, want 智能小组", out.List[0].CurrentOrg)
	}

	values, err := global.Redis.HGetAll(context.Background(), rediskey.RankingUserHashKey(2)).Result()
	if err != nil {
		t.Fatalf("HGetAll() error = %v", err)
	}
	if strings.TrimSpace(values["current_org_name"]) != "智能小组" {
		t.Fatalf("cached current_org_name = %q, want %q", values["current_org_name"], "智能小组")
	}
	if strings.TrimSpace(values["current_org_id"]) != "201" {
		t.Fatalf("cached current_org_id = %q, want %q", values["current_org_id"], "201")
	}
}

func TestBuildRankingListKeepsValidPlatformAvatar(t *testing.T) {
	list := buildRankingList(
		[]rankingEntry{{UserID: 7, Score: 42}},
		0,
		"leetcode",
		rankingScopeCurrentOrg,
		map[uint]*rankingcache.UserProjection{
			7: {
				UserID:   7,
				Username: "alice",
				Avatar:   "https://cdn.example.com/base-avatar.png",
				Active:   true,
				Leetcode: rankingcache.PlatformProfile{
					Identifier: "alice-lc",
					Avatar:     "https://cdn.example.com/alice-profile.png",
					Score:      42,
				},
			},
		},
	)

	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Avatar != "https://cdn.example.com/alice-profile.png" {
		t.Fatalf("avatar = %q, want valid platform avatar", list[0].Avatar)
	}
}

func TestBuildRankingListFallsBackToBaseAvatarWhenPlatformAvatarLooksPlaceholder(t *testing.T) {
	list := buildRankingList(
		[]rankingEntry{{UserID: 8, Score: 21}},
		0,
		"leetcode",
		rankingScopeCurrentOrg,
		map[uint]*rankingcache.UserProjection{
			8: {
				UserID:   8,
				Username: "bob",
				Avatar:   "https://cdn.example.com/users/bob-real.png",
				Active:   true,
				Leetcode: rankingcache.PlatformProfile{
					Identifier: "bob-lc",
					Avatar:     "https://assets.example.com/default-avatar.png",
					Score:      21,
				},
			},
		},
	)

	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Avatar != "https://cdn.example.com/users/bob-real.png" {
		t.Fatalf("avatar = %q, want fallback base avatar", list[0].Avatar)
	}
}

func TestBuildRankingListClearsAvatarWhenAllCandidatesLookPlaceholder(t *testing.T) {
	list := buildRankingList(
		[]rankingEntry{{UserID: 9, Score: 13}},
		0,
		"luogu",
		rankingScopeCurrentOrg,
		map[uint]*rankingcache.UserProjection{
			9: {
				UserID:   9,
				Username: "carol",
				Avatar:   "https://cdn.example.com/img/no-avatar.svg",
				Active:   true,
				Luogu: rankingcache.PlatformProfile{
					Identifier: "lg-9",
					Avatar:     "https://cdn.example.com/img/default_user.png",
					Score:      13,
				},
			},
		},
	)

	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Avatar != "" {
		t.Fatalf("avatar = %q, want empty avatar", list[0].Avatar)
	}
}

func TestBuildRankingListDoesNotMisclassifyNormalAvatarURL(t *testing.T) {
	list := buildRankingList(
		[]rankingEntry{{UserID: 10, Score: 8}},
		0,
		"luogu",
		rankingScopeCurrentOrg,
		map[uint]*rankingcache.UserProjection{
			10: {
				UserID:   10,
				Username: "dave",
				Avatar:   "https://cdn.example.com/avatar/dave.png",
				Active:   true,
				Luogu: rankingcache.PlatformProfile{
					Identifier: "lg-10",
					Avatar:     "https://cdn.example.com/avatar/contest/dave-final.png",
					Score:      8,
				},
			},
		},
	)

	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Avatar != "https://cdn.example.com/avatar/contest/dave-final.png" {
		t.Fatalf("avatar = %q, want normal avatar url preserved", list[0].Avatar)
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
