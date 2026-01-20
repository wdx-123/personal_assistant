package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/infrastructure"
	lc "personal_assistant/internal/infrastructure/leetcode"
	lg "personal_assistant/internal/infrastructure/luogu"
	"personal_assistant/internal/infrastructure/outbox"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrInvalidPlatform   = errors.New("invalid platform")
	ErrInvalidIdentifier = errors.New("invalid identifier")
	ErrBindCoolDown      = errors.New("bind operation is in cooldown")
	ErrOJAccountNotBound = errors.New("oj account not bound")
)

type OJService struct {
	userRepo              interfaces.UserRepository
	orgRepo               interfaces.OrgRepository
	leetcodeRepo          interfaces.LeetcodeUserDetailRepository
	luoguRepo             interfaces.LuoguUserDetailRepository
	luoguQuestionBankRepo interfaces.LuoguQuestionBankRepository
	luoguUserQuestionRepo interfaces.LuoguUserQuestionRepository
	outboxRepo            interfaces.OutboxRepository
}

func NewOJService(repositoryGroup *repository.Group) *OJService {
	return &OJService{
		userRepo:              repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		orgRepo:               repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		leetcodeRepo:          repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		luoguRepo:             repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
		luoguQuestionBankRepo: repositoryGroup.SystemRepositorySupplier.GetLuoguQuestionBankRepository(),
		luoguUserQuestionRepo: repositoryGroup.SystemRepositorySupplier.GetLuoguUserQuestionRepository(),
		outboxRepo:            repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
	}
}

func (s *OJService) BindOJAccount(
	ctx context.Context,
	userID uint,
	req *request.BindOJAccountReq,
) (*resp.BindOJAccountResp, error) {
	if req == nil {
		return nil, ErrInvalidIdentifier
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return nil, ErrInvalidIdentifier
	}

	const sleepSec = 0.5

	switch platform {
	case "leetcode":
		return s.bindLeetCode(ctx, userID, identifier, sleepSec)
	case "luogu":
		return s.bindLuogu(ctx, userID, identifier, sleepSec)
	default:
		return nil, ErrInvalidPlatform
	}
}

func (s *OJService) GetUserStats(
	ctx context.Context,
	userID uint,
	req *request.OJStatsReq,
) (*resp.BindOJAccountResp, error) {
	if req == nil {
		return nil, errors.New("invalid request")
	}
	if userID == 0 {
		return nil, errors.New("invalid user id")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "luogu" && platform != "leetcode" {
		return nil, ErrInvalidPlatform
	}

	if platform == "luogu" {
		detail, err := s.luoguRepo.GetByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if detail == nil {
			return nil, ErrOJAccountNotBound
		}
		return &resp.BindOJAccountResp{
			Platform:     "luogu",
			Identifier:   detail.Identification,
			RealName:     detail.RealName,
			UserAvatar:   detail.UserAvatar,
			PassedNumber: detail.PassedNumber,
		}, nil
	}

	detail, err := s.leetcodeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, ErrOJAccountNotBound
	}
	return &resp.BindOJAccountResp{
		Platform:     "leetcode",
		Identifier:   detail.UserSlug,
		RealName:     detail.RealName,
		UserAvatar:   detail.UserAvatar,
		PassedNumber: detail.TotalNumber,
	}, nil
}

func (s *OJService) bindLeetCode(
	ctx context.Context,
	userID uint,
	identifier string,
	sleepSec float64,
) (*resp.BindOJAccountResp, error) {
	// 1. 检查冷却时间
	existing, err := s.leetcodeRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// 检查是否在冷却期内 (配置化时间)
		coolDownHours := global.Config.System.BindCoolDownHours
		if coolDownHours <= 0 {
			coolDownHours = 24 // 默认24小时
		}
		if time.Since(existing.UpdatedAt) < time.Duration(coolDownHours)*time.Hour {
			return nil, ErrBindCoolDown
		}
		// 2. 如果存在旧记录且ID变更（即换绑），则清空旧数据
		// 注意：如果只是更新同一个账号的信息，其实不需要删，但为了逻辑简化统一，
		// 这里选择：只要是“绑定”操作，就视为重置。
		// 更精细的逻辑是：if existing.UserSlug != identifier { ... }
		// 按照用户要求：“换绑时必须删除”。
		if existing.UserSlug != identifier {
			if err := s.leetcodeRepo.DeleteByUserID(ctx, userID); err != nil {
				return nil, err
			}
		}
	}

	out, err := infrastructure.LeetCode().PublicProfile(ctx, identifier, sleepSec)
	if err != nil {
		return nil, err
	}
	if out == nil || !out.OK {
		return nil, errors.New("leetcode request failed")
	}

	easy, medium, hard := extractLeetCodeCounts(out)
	total := easy + medium + hard

	detail := &entity.LeetcodeUserDetail{
		UserSlug:     strings.TrimSpace(out.Data.Profile.UserSlug),
		RealName:     strings.TrimSpace(out.Data.Profile.RealName),
		UserAvatar:   strings.TrimSpace(out.Data.Profile.UserAvatar),
		EasyNumber:   easy,
		MediumNumber: medium,
		HardNumber:   hard,
		TotalNumber:  total,
		UserID:       userID,
	}

	saved, err := s.leetcodeRepo.UpsertByUserID(ctx, detail)
	if err != nil {
		return nil, err
	}

	if err := s.updateRankingCache(ctx, userID, "leetcode", saved.TotalNumber); err != nil {
		return nil, err
	}

	return &resp.BindOJAccountResp{
		Platform:     "leetcode",
		Identifier:   saved.UserSlug,
		RealName:     saved.RealName,
		UserAvatar:   saved.UserAvatar,
		PassedNumber: saved.TotalNumber,
	}, nil
}

// SyncAllLuoguUsers 同步所有洛谷用户的做题记录
func (s *OJService) SyncAllLuoguUsers(ctx context.Context) error {
	lockKey := redislock.LockKeyLuoguSyncAllUsers
	err := redislock.WithLock(ctx, lockKey, 30*time.Second, func() error {
		return s.syncAllLuoguUsersLocked(ctx)
	})
	if err != nil && errors.Is(err, redislock.ErrLockFailed) {
		global.Log.Info("luogu sync skipped: lock is held", zap.String("lock_key", lockKey))
		return nil
	}
	return err
}

func (s *OJService) syncAllLuoguUsersLocked(ctx context.Context) error {

	users, err := s.luoguRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	return s.syncLuoguUsersWithRateLimit(ctx, users)
}

func (s *OJService) syncLuoguUsersWithRateLimit(
	ctx context.Context,
	users []*entity.LuoguUserDetail,
) error {
	intervalSeconds := global.Config.Task.LuoguSyncUserIntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 10
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for _, u := range users {
		if err := s.waitTickerOrCancel(ctx, ticker); err != nil {
			global.Log.Info("sync luogu users canceled", zap.Error(err))
			return err
		}

		// 加用户级细粒度锁，防止与 Bind 操作冲突
		// 如果获取锁失败（例如用户正在绑定），记录日志并跳过，不阻塞整个任务
		userLockKey := redislock.LockKeyLuoguSyncSingleUser(u.Identification)
		err := redislock.WithLock(ctx, userLockKey, 10*time.Second, func() error {
			return s.syncSingleLuoguUser(ctx, u)
		})

		if err != nil {
			if errors.Is(err, redislock.ErrLockFailed) {
				global.Log.Warn("skip syncing user: lock held (possibly binding)", zap.String("uid", u.Identification))
				continue
			}
			global.Log.Error("failed to sync luogu user", zap.String("uid", u.Identification), zap.Error(err))
			continue
		}
	}

	return nil
}

func (s *OJService) waitTickerOrCancel(ctx context.Context, ticker *time.Ticker) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return nil
	}
}

func (s *OJService) syncSingleLuoguUser(
	ctx context.Context,
	u *entity.LuoguUserDetail,
) error {
	out, err := s.fetchLuoguPractice(ctx, u)
	if err != nil {
		return err
	}
	if out == nil || !out.OK {
		return errors.New("luogu api response not ok")
	}

	if err := s.upsertLuoguProblems(ctx, out.Data.Passed); err != nil {
		global.Log.Error("failed to upsert luogu problems", zap.Error(err))
	}

	newRecords, err := s.syncLuoguUserSolvedRelations(ctx, u.ID, out.Data.Passed)
	if err != nil {
		global.Log.Error("failed to sync luogu user relations", zap.Error(err))
	} else if newRecords > 0 {
		global.Log.Info("synced luogu user records",
			zap.String("user", u.RealName),
			zap.Int("new_records", newRecords))
	}

	if err := s.updateLuoguUserInfoIfChanged(ctx, u, out.Data.User.Name, out.Data.User.Avatar, out.Data.PassedCount, len(out.Data.Passed)); err != nil {
		global.Log.Error("failed to update luogu user info", zap.Error(err))
	}

	passed := out.Data.PassedCount
	if passed <= 0 {
		passed = len(out.Data.Passed)
	}
	if err := s.updateRankingCache(ctx, u.UserID, "luogu", passed); err != nil {
		global.Log.Error("failed to update luogu ranking cache", zap.Error(err))
	}

	return nil
}

func (s *OJService) fetchLuoguPractice(ctx context.Context, u *entity.LuoguUserDetail) (*lg.GetPracticeResponse, error) {
	if u == nil {
		return nil, errors.New("nil luogu user")
	}
	uid, err := strconv.Atoi(strings.TrimSpace(u.Identification))
	if err != nil || uid <= 0 {
		return nil, fmt.Errorf("invalid luogu uid: %s", u.Identification)
	}

	out, err := infrastructure.Luogu().GetPractice(ctx, uid, 0.5)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(out.Data.User.Name)
	avatar := strings.TrimSpace(out.Data.User.Avatar)
	passed := out.Data.PassedCount
	if passed <= 0 {
		passed = len(out.Data.Passed)
	}
	if name == "" && avatar == "" && passed == 0 {
		return nil, ErrOJAccountNotBound
	}
	return out, nil
}

// upsertLuoguProblems 批量插入洛谷题目
func (s *OJService) upsertLuoguProblems(
	ctx context.Context,
	passed []lg.PassedProblem,
) error {
	var newProblems []*entity.LuoguQuestionBank
	seenInBatch := make(map[string]struct{})

	for _, p := range passed {
		id, ok, err := s.luoguQuestionBankRepo.GetCachedID(ctx, p.PID)
		if err == nil && ok && id > 0 {
			continue
		}

		if _, seen := seenInBatch[p.PID]; seen {
			continue
		}
		q, err := s.luoguQuestionBankRepo.GetByPID(ctx, p.PID)
		if err == nil && q.ID > 0 {
			_ = s.luoguQuestionBankRepo.CacheID(ctx, p.PID, q.ID)
			continue
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		newProblems = append(newProblems, &entity.LuoguQuestionBank{
			Pid:        p.PID,
			Title:      p.Title,
			Difficulty: strconv.Itoa(p.Difficulty),
			Type:       p.Type,
		})
		seenInBatch[p.PID] = struct{}{}
	}

	if len(newProblems) == 0 {
		return nil
	}

	// 批量插入 (Repository 层会自动回填 Redis)
	if err := s.luoguQuestionBankRepo.BatchCreate(ctx, newProblems); err != nil {
		return err
	}

	global.Log.Info("added new luogu problems", zap.Int("count", len(newProblems)))
	return nil
}

// syncLuoguUserSolvedRelations 同步洛谷用户做题记录
func (s *OJService) syncLuoguUserSolvedRelations(
	ctx context.Context,
	luoguUserDetailID uint,
	passed []lg.PassedProblem,
) (int, error) {
	solvedSet, err := s.luoguUserQuestionRepo.GetSolvedProblemIDs(ctx, luoguUserDetailID)
	if err != nil {
		return 0, err
	}

	var newRelations []*entity.LuoguUserQuestion
	for _, p := range passed {
		qID, ok, err := s.luoguQuestionBankRepo.GetCachedID(ctx, p.PID)
		if err != nil {
			continue
		}
		if !ok || qID == 0 {
			q, err := s.luoguQuestionBankRepo.GetByPID(ctx, p.PID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return 0, err
			}
			qID = q.ID
			if qID > 0 {
				_ = s.luoguQuestionBankRepo.CacheID(ctx, p.PID, qID)
			}
		}
		if qID == 0 {
			continue
		}

		if _, solved := solvedSet[qID]; solved {
			continue
		}
		newRelations = append(newRelations, &entity.LuoguUserQuestion{
			LuoguUserDetailID: luoguUserDetailID,
			LuoguQuestionID:   qID,
		})
	}

	if len(newRelations) == 0 {
		return 0, nil
	}

	if err := s.luoguUserQuestionRepo.BatchCreate(ctx, newRelations); err != nil {
		return 0, err
	}

	return len(newRelations), nil
}

func (s *OJService) updateLuoguUserInfoIfChanged(
	ctx context.Context,
	u *entity.LuoguUserDetail,
	name string,
	avatar string,
	passedCount int,
	passedLen int,
) error {
	if u == nil {
		return nil
	}

	if passedCount <= 0 {
		passedCount = passedLen
	}

	if u.PassedNumber == passedCount && u.UserAvatar == avatar && u.RealName == name {
		return nil
	}

	u.PassedNumber = passedCount
	u.RealName = name
	u.UserAvatar = avatar
	_, err := s.luoguRepo.UpsertByUserID(ctx, u)
	return err
}

func (s *OJService) updateRankingCache(
	ctx context.Context,
	userID uint,
	platform string,
	totalPassed int,
) error {
	if userID == 0 || platform == "" {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.CurrentOrgID == nil || *user.CurrentOrgID == 0 {
		return nil
	}
	key := rediskey.RankingZSetKey(*user.CurrentOrgID, platform)
	member := strconv.FormatUint(uint64(userID), 10)
	score := float64(totalPassed)
	return global.Redis.ZAdd(ctx, key, &redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

func (s *OJService) RebuildRankingCaches(ctx context.Context) error {
	orgs, err := s.orgRepo.GetAllOrgs(ctx)
	if err != nil {
		return err
	}
	platforms := []string{"luogu", "leetcode"}
	for _, org := range orgs {
		for _, platform := range platforms {
			if err := s.rebuildRankingCache(ctx, org.ID, platform); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *OJService) rebuildRankingCache(
	ctx context.Context,
	orgID uint,
	platform string,
) error {
	if orgID == 0 {
		return nil
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "luogu" && platform != "leetcode" {
		return ErrInvalidPlatform
	}
	key := rediskey.RankingZSetKey(orgID, platform)
	pipe := global.Redis.Pipeline()
	pipe.Del(ctx, key)

	if platform == "luogu" {
		details, err := s.luoguRepo.ListByOrgID(ctx, orgID)
		if err != nil {
			return err
		}
		for _, detail := range details {
			if detail == nil || detail.UserID == 0 {
				continue
			}
			member := strconv.FormatUint(uint64(detail.UserID), 10)
			pipe.ZAdd(ctx, key, &redis.Z{
				Score:  float64(detail.PassedNumber),
				Member: member,
			})
		}
	} else {
		details, err := s.leetcodeRepo.ListByOrgID(ctx, orgID)
		if err != nil {
			return err
		}
		for _, detail := range details {
			if detail == nil || detail.UserID == 0 {
				continue
			}
			member := strconv.FormatUint(uint64(detail.UserID), 10)
			pipe.ZAdd(ctx, key, &redis.Z{
				Score:  float64(detail.TotalNumber),
				Member: member,
			})
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *OJService) GetRankingList(
	ctx context.Context,
	userID uint,
	req *request.OJRankingListReq,
) (*resp.OJRankingListResp, error) {
	if req == nil {
		return nil, errors.New("invalid request")
	}
	if userID == 0 {
		return nil, errors.New("invalid user id")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "luogu" && platform != "leetcode" {
		return nil, ErrInvalidPlatform
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.CurrentOrgID == nil || *user.CurrentOrgID == 0 {
		return nil, errors.New("user organization not found")
	}

	key := rediskey.RankingZSetKey(*user.CurrentOrgID, platform)
	total, err := global.Redis.ZCard(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	start := int64((page - 1) * pageSize)
	stop := start + int64(pageSize) - 1
	ranges, err := global.Redis.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	userIDs := make([]uint, 0, len(ranges))
	orderedIDs := make([]uint, 0, len(ranges))
	for _, item := range ranges {
		member, ok := item.Member.(string)
		if !ok || member == "" {
			continue
		}
		id, parseErr := strconv.ParseUint(member, 10, 64)
		if parseErr != nil || id == 0 {
			continue
		}
		uid := uint(id)
		userIDs = append(userIDs, uid)
		orderedIDs = append(orderedIDs, uid)
	}

	userMap := make(map[uint]*entity.User)
	if len(userIDs) > 0 {
		users, err := s.userRepo.GetByIDs(ctx, userIDs)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if u != nil {
				userMap[u.ID] = u
			}
		}
	}

	luoguMap := make(map[uint]*entity.LuoguUserDetail)
	leetcodeMap := make(map[uint]*entity.LeetcodeUserDetail)
	if len(userIDs) > 0 {
		if platform == "luogu" {
			details, err := s.luoguRepo.GetByUserIDs(ctx, userIDs)
			if err != nil {
				return nil, err
			}
			for _, d := range details {
				if d != nil {
					luoguMap[d.UserID] = d
				}
			}
		} else {
			details, err := s.leetcodeRepo.GetByUserIDs(ctx, userIDs)
			if err != nil {
				return nil, err
			}
			for _, d := range details {
				if d != nil {
					leetcodeMap[d.UserID] = d
				}
			}
		}
	}

	list := make([]*resp.OJRankingListItem, 0, len(orderedIDs))
	for idx, uid := range orderedIDs {
		score := int(ranges[idx].Score)
		var realName string
		var avatar string
		if platform == "luogu" {
			if detail := luoguMap[uid]; detail != nil {
				realName = detail.RealName
				avatar = detail.UserAvatar
			}
		} else {
			if detail := leetcodeMap[uid]; detail != nil {
				realName = detail.RealName
				avatar = detail.UserAvatar
			}
		}
		if realName == "" {
			if u := userMap[uid]; u != nil {
				realName = u.Username
			}
		}
		if avatar == "" {
			if u := userMap[uid]; u != nil {
				avatar = u.Avatar
			}
		}
		item := &resp.OJRankingListItem{
			Rank:        int(start) + idx + 1,
			UserID:      uid,
			RealName:    realName,
			Avatar:      avatar,
			TotalPassed: score,
		}
		if platform == "luogu" {
			item.PlatformDetails = &resp.OJRankingPlatformDetails{
				Luogu: score,
			}
		} else {
			item.PlatformDetails = &resp.OJRankingPlatformDetails{
				Leetcode: score,
			}
		}
		list = append(list, item)
	}

	var myRank *resp.OJRankingMyRank
	rank, err := global.Redis.ZRevRank(ctx, key, strconv.FormatUint(uint64(userID), 10)).Result()
	if err == nil {
		score, scoreErr := global.Redis.ZScore(ctx, key, strconv.FormatUint(uint64(userID), 10)).Result()
		if scoreErr == nil {
			myRank = &resp.OJRankingMyRank{
				Rank:        int(rank) + 1,
				TotalPassed: int(score),
			}
		}
	} else if !errors.Is(err, redis.Nil) {
		return nil, err
	}

	return &resp.OJRankingListResp{
		List:   list,
		MyRank: myRank,
		Total:  total,
	}, nil
}

type LuoguBindPayload struct {
	Passed []lg.PassedProblem `json:"passed"`
}

func (s *OJService) HandleLuoguBindPayload(
	ctx context.Context,
	userID uint,
	payload *LuoguBindPayload,
) error {
	if payload == nil {
		return errors.New("nil luogu bind payload")
	}
	if userID == 0 {
		return errors.New("invalid luogu user id")
	}
	detail, err := s.luoguRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if detail == nil {
		return errors.New("luogu user detail not found")
	}
	identifier := strings.TrimSpace(detail.Identification)
	if identifier == "" {
		return errors.New("luogu identifier missing")
	}
	lockKey := redislock.LockKeyLuoguSyncSingleUser(identifier)
	return redislock.WithLock(ctx, lockKey, 10*time.Second, func() error {
		if err := s.upsertLuoguProblems(ctx, payload.Passed); err != nil {
			return err
		}

		if _, err := s.syncLuoguUserSolvedRelations(ctx, detail.ID, payload.Passed); err != nil {
			return err
		}

		return nil
	})
}

func extractLeetCodeCounts(out *lc.PublicProfileResponse) (int, int, int) {
	if out == nil {
		return 0, 0, 0
	}

	list := out.Data.SubmitStatsGlobal.AcSubmissionNum
	if len(list) == 0 {
		list = out.Data.SubmitStats.AcSubmissionNum
	}

	easy, medium, hard := 0, 0, 0
	for _, item := range list {
		switch strings.ToLower(strings.TrimSpace(item.Difficulty)) {
		case "easy":
			easy = item.Count
		case "medium":
			medium = item.Count
		case "hard":
			hard = item.Count
		}
	}
	return easy, medium, hard
}

func (s *OJService) bindLuogu(
	ctx context.Context,
	userID uint,
	identifier string,
	sleepSec float64,
) (*resp.BindOJAccountResp, error) {
	// 确保绑定条件
	if err := s.ensureLuoguBindReady(ctx, userID, identifier); err != nil {
		return nil, err
	}

	// 获取洛谷用户信息
	out, err := s.fetchLuoguPracticeForBind(ctx, identifier, sleepSec)
	if err != nil {
		return nil, err
	}

	// 保存并发布
	saved, err := s.upsertLuoguBindAndPublish(ctx, userID, identifier, out)
	if err != nil {
		return nil, err
	}

	return &resp.BindOJAccountResp{
		Platform:     "luogu",
		Identifier:   saved.Identification,
		RealName:     saved.RealName,
		UserAvatar:   saved.UserAvatar,
		PassedNumber: saved.PassedNumber,
	}, nil
}

func (s *OJService) ensureLuoguBindReady(
	ctx context.Context,
	userID uint,
	identifier string,
) error {
	existing, err := s.luoguRepo.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}

	coolDownHours := global.Config.System.BindCoolDownHours
	if coolDownHours <= 0 {
		coolDownHours = 24
	}
	if time.Since(existing.UpdatedAt) < time.Duration(coolDownHours)*time.Hour {
		return ErrBindCoolDown
	}
	if existing.Identification != identifier {
		return s.luoguRepo.DeleteByUserID(ctx, userID)
	}
	return nil
}

func (s *OJService) fetchLuoguPracticeForBind(
	ctx context.Context,
	identifier string,
	sleepSec float64,
) (*lg.GetPracticeResponse, error) {
	uid, err := strconv.Atoi(identifier)
	if err != nil || uid <= 0 {
		return nil, ErrInvalidIdentifier
	}
	out, err := infrastructure.Luogu().GetPractice(ctx, uid, sleepSec)
	if err != nil {
		// 转换用户友好提示
		if strings.Contains(err.Error(), "403") &&
			(strings.Contains(err.Error(), "Forbidden") ||
				strings.Contains(err.Error(), "Client Error")) {
			global.Log.Warn("luogu practice forbidden", zap.String("identifier", identifier), zap.Error(err))
			return nil, errors.New("由于您的洛谷账户受到官方保护，暂时无法爬取，请联系管理员")
		}
		return nil, err
	}
	if out == nil || !out.OK {
		return nil, errors.New("luogu request failed")
	}
	name := strings.TrimSpace(out.Data.User.Name)
	avatar := strings.TrimSpace(out.Data.User.Avatar)
	passed := out.Data.PassedCount
	if passed <= 0 {
		passed = len(out.Data.Passed)
	}
	if name == "" && avatar == "" && passed == 0 {
		return nil, ErrOJAccountNotBound
	}
	return out, nil
}

func (s *OJService) upsertLuoguBindAndPublish(
	ctx context.Context,
	userID uint,
	identifier string,
	out *lg.GetPracticeResponse,
) (*entity.LuoguUserDetail, error) {
	var saved *entity.LuoguUserDetail
	lockKey := redislock.LockKeyLuoguSyncSingleUser(identifier)
	err := redislock.WithLock(ctx, lockKey, 10*time.Second, func() error {
		passed := out.Data.PassedCount
		if passed <= 0 {
			passed = len(out.Data.Passed)
		}

		detail := &entity.LuoguUserDetail{
			Identification: identifier,
			RealName:       strings.TrimSpace(out.Data.User.Name),
			UserAvatar:     strings.TrimSpace(out.Data.User.Avatar),
			PassedNumber:   passed,
			UserID:         userID,
		}

		var upsertErr error
		saved, upsertErr = s.luoguRepo.UpsertByUserID(ctx, detail)
		if upsertErr != nil {
			return upsertErr
		}

		if err := s.updateRankingCache(ctx, userID, "luogu", passed); err != nil {
			return err
		}

		return s.publishLuoguBindOutbox(ctx, userID, out.Data.Passed)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}

func (s *OJService) publishLuoguBindOutbox(
	ctx context.Context,
	userID uint,
	passed []lg.PassedProblem,
) error {
	topic := strings.TrimSpace(global.Config.Messaging.LuoguBindTopic)
	if topic == "" {
		return errors.New("luogu bind topic config is empty")
	}

	payload := &LuoguBindPayload{
		Passed: passed,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	event := &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(userID), 10),
		AggregateType: "luogu_user",
		Payload:       string(payloadBytes),
	}
	if err := s.outboxRepo.Create(ctx, event); err != nil {
		return err
	}
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil {
		global.Log.Warn("luogu bind notify outbox failed", zap.Error(err))
	}
	return nil
}
