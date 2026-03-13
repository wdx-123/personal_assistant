package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure"
	lc "personal_assistant/internal/infrastructure/leetcode"
	lg "personal_assistant/internal/infrastructure/luogu"
	"personal_assistant/internal/infrastructure/outbox"
	"personal_assistant/internal/model/consts"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	svccontract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/observability/contextid"
	"personal_assistant/pkg/observability/w3c"
	"personal_assistant/pkg/rankingcache"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"

	"github.com/go-redis/redis/v8"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OJService struct {
	userRepo                  interfaces.UserRepository
	orgRepo                   interfaces.OrgRepository
	orgMemberRepo             interfaces.OrgMemberRepository
	roleRepo                  interfaces.RoleRepository
	leetcodeRepo              interfaces.LeetcodeUserDetailRepository
	luoguRepo                 interfaces.LuoguUserDetailRepository
	leetcodeQuestionBankRepo  interfaces.LeetcodeQuestionBankRepository
	luoguQuestionBankRepo     interfaces.LuoguQuestionBankRepository
	leetcodeUserQuestionRepo  interfaces.LeetcodeUserQuestionRepository
	luoguUserQuestionRepo     interfaces.LuoguUserQuestionRepository
	ojDailyStatsRepo          interfaces.OJDailyStatsRepository
	rankingReadModelRepo      interfaces.RankingReadModelRepository
	outboxRepo                interfaces.OutboxRepository
	cacheProjectionPublisher  cacheProjectionEventPublisher
	cacheProjectionSvc        svccontract.CacheProjectionServiceContract
	ojDailyStatsProjectionSvc svccontract.OJDailyStatsProjectionServiceContract
}

func NewOJService(
	repositoryGroup *repository.Group,
	cacheProjectionSvc svccontract.CacheProjectionServiceContract,
	ojDailyStatsProjectionSvc svccontract.OJDailyStatsProjectionServiceContract,
) *OJService {
	return &OJService{
		userRepo:                 repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		orgRepo:                  repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		orgMemberRepo:            repositoryGroup.SystemRepositorySupplier.GetOrgMemberRepository(),
		roleRepo:                 repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		leetcodeRepo:             repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		luoguRepo:                repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
		leetcodeQuestionBankRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeQuestionBankRepository(),
		luoguQuestionBankRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguQuestionBankRepository(),
		leetcodeUserQuestionRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserQuestionRepository(),
		luoguUserQuestionRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguUserQuestionRepository(),
		ojDailyStatsRepo:         repositoryGroup.SystemRepositorySupplier.GetOJDailyStatsRepository(),
		rankingReadModelRepo:     repositoryGroup.SystemRepositorySupplier.GetRankingReadModelRepository(),
		outboxRepo:               repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		cacheProjectionPublisher: newCacheProjectionOutboxPublisher(
			repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		),
		cacheProjectionSvc:        cacheProjectionSvc,
		ojDailyStatsProjectionSvc: ojDailyStatsProjectionSvc,
	}
}

func (s *OJService) BindOJAccount(
	ctx context.Context,
	userID uint,
	req *request.BindOJAccountReq,
) (*resp.BindOJAccountResp, error) {
	if req == nil {
		return nil, svccontract.ErrInvalidIdentifier
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return nil, svccontract.ErrInvalidIdentifier
	}

	const sleepSec = 0.2

	switch platform {
	case "leetcode":
		return s.bindLeetCode(ctx, userID, identifier, sleepSec)
	case "luogu":
		return s.bindLuogu(ctx, userID, identifier, sleepSec)
	default:
		return nil, svccontract.ErrInvalidPlatform
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
		return nil, svccontract.ErrInvalidPlatform
	}

	if platform == "luogu" {
		detail, err := s.luoguRepo.GetByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if detail == nil {
			return nil, svccontract.ErrOJAccountNotBound
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
		return nil, svccontract.ErrOJAccountNotBound
	}
	return &resp.BindOJAccountResp{
		Platform:     "leetcode",
		Identifier:   detail.UserSlug,
		RealName:     detail.RealName,
		UserAvatar:   detail.UserAvatar,
		PassedNumber: detail.TotalNumber,
	}, nil
}

// GetCurve 获取用户的 OJ 做题曲线数据，返回最近 30 天每天的做题数和累计总数，
// 以及当前总做题数和最后同步时间。如果数据不完整或过旧，会尝试修复后重新查询。
func (s *OJService) GetCurve(
	ctx context.Context,
	userID uint,
	req *request.OJCurveReq,
) (*resp.OJCurveResp, error) {
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}
	if userID == 0 {
		return nil, bizerrors.New(bizerrors.CodeLoginRequired)
	}

	// 1. 验证平台参数
	platform := normalizeOJDailyStatsPlatform(req.Platform)
	if platform == "" {
		return nil, bizerrors.New(bizerrors.CodeOJPlatformInvalid)
	}

	// 2. 加载构建曲线所需的原始数据，包括当前总做题数、详情更新时间和是否已绑定
	currentTotal, detailUpdatedAt, bound, err := s.loadOJCurveSource(ctx, userID, platform)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if !bound {
		return &resp.OJCurveResp{
			Platform: platform,
			Bound:    false,
			Points:   []*resp.OJCurvePoint{},
		}, nil
	}

	const curveDays = 30
	fromDate, toDateExclusive := buildOJDailyStatsWindowRange(curveDays)
	toDate := toDateExclusive.AddDate(0, 0, -1)

	// 2. 查询最近窗口的数据
	rows, err := s.ojDailyStatsRepo.ListRange(ctx, userID, platform, fromDate, toDate)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	// 3. 如果数据不完整或过旧，尝试修复后重新查询
	if needsOJCurveRebuild(rows, detailUpdatedAt, curveDays) && s.ojDailyStatsProjectionSvc != nil {
		if err := s.ojDailyStatsProjectionSvc.RebuildRecentWindow(ctx, userID, platform, false); err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeOJSyncFailed, err)
		}
		rows, err = s.ojDailyStatsRepo.ListRange(ctx, userID, platform, fromDate, toDate)
		if err != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
	}

	return buildOJCurveResponse(platform, currentTotal, detailUpdatedAt, fromDate, curveDays, rows), nil
}

// loadOJCurveSource 加载构建 OJ 曲线所需的原始数据，
// 包括当前总做题数、详情更新时间和是否已绑定。根据平台不同从对应的用户详情表加载。
func (s *OJService) loadOJCurveSource(
	ctx context.Context,
	userID uint,
	platform string,
) (int, time.Time, bool, error) {
	switch platform {
	case "leetcode":
		detail, err := s.leetcodeRepo.GetByUserID(ctx, userID)
		if err != nil {
			return 0, time.Time{}, false, err
		}
		if detail == nil {
			return 0, time.Time{}, false, nil
		}
		return detail.TotalNumber, detail.UpdatedAt, true, nil
	case "luogu":
		detail, err := s.luoguRepo.GetByUserID(ctx, userID)
		if err != nil {
			return 0, time.Time{}, false, err
		}
		if detail == nil {
			return 0, time.Time{}, false, nil
		}
		return detail.PassedNumber, detail.UpdatedAt, true, nil
	default:
		return 0, time.Time{}, false, svccontract.ErrInvalidPlatform
	}
}

func needsOJCurveRebuild(
	rows []*entity.OJUserDailyStat, // 最近窗口内的原始数据行列表，可能不连续或过旧
	detailUpdatedAt time.Time, // 用户详情的最后更新时间，用于判断数据是否过旧
	expectedDays int,
) bool {
	if len(rows) != expectedDays {
		return true
	}

	latestSourceUpdatedAt := time.Time{}
	seenDates := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row == nil {
			return true
		}
		// 数据行的日期必须在最近窗口范围内，否则视为不完整
		key := row.StatDate.In(ojDailyStatsLocation()).Format("2006-01-02")
		if _, exists := seenDates[key]; exists {
			return true
		}
		seenDates[key] = struct{}{}
		// 数据行的来源更新时间必须不晚于用户详情的更新时间，否则视为过旧
		if row.SourceUpdatedAt.After(latestSourceUpdatedAt) {
			latestSourceUpdatedAt = row.SourceUpdatedAt
		}
	}

	if detailUpdatedAt.IsZero() {
		return false
	}
	return latestSourceUpdatedAt.Before(detailUpdatedAt)
}

func buildOJCurveResponse(
	platform string,
	currentTotal int,
	detailUpdatedAt time.Time,
	startDate time.Time,
	curveDays int,
	rows []*entity.OJUserDailyStat,
) *resp.OJCurveResp {
	rowMap := make(map[string]*entity.OJUserDailyStat, len(rows))
	var (
		firstRow            *entity.OJUserDailyStat
		latestSourceUpdated time.Time
	)
	for _, row := range rows {
		if row == nil {
			continue
		}
		if firstRow == nil {
			firstRow = row
		}
		key := row.StatDate.In(ojDailyStatsLocation()).Format("2006-01-02")
		rowMap[key] = row
		if row.SourceUpdatedAt.After(latestSourceUpdated) {
			latestSourceUpdated = row.SourceUpdatedAt
		}
	}

	runningTotal := currentTotal
	if firstRow != nil {
		runningTotal = firstRow.SolvedTotal - firstRow.SolvedCount
		if runningTotal < 0 {
			runningTotal = 0
		}
	}

	// 按照日期顺序构建曲线点，填充缺失日期，计算累计总数
	points := make([]*resp.OJCurvePoint, 0, curveDays)
	for i := 0; i < curveDays; i++ {
		statDate := startDate.AddDate(0, 0, i)
		key := statDate.Format("2006-01-02")
		solvedCount := 0
		if row, ok := rowMap[key]; ok {
			solvedCount = row.SolvedCount
			runningTotal = row.SolvedTotal
		}
		points = append(points, &resp.OJCurvePoint{
			Date:        key,
			SolvedCount: solvedCount,
			SolvedTotal: runningTotal,
		})
	}

	lastSyncAt := detailUpdatedAt
	if latestSourceUpdated.After(lastSyncAt) {
		lastSyncAt = latestSourceUpdated
	}
	var lastSyncAtPtr *time.Time
	if !lastSyncAt.IsZero() {
		lastSyncCopy := lastSyncAt
		lastSyncAtPtr = &lastSyncCopy
	}

	return &resp.OJCurveResp{
		Platform:     platform,
		Bound:        true,
		CurrentTotal: currentTotal,
		LastSyncAt:   lastSyncAtPtr,
		Points:       points,
	}
}

// resolveOJDailyStatsRepairWindowDays 获取修复窗口天数，默认为 35 天
func shouldRefreshLeetcodeCurve(
	u *entity.LeetcodeUserDetail,
	total int,
) bool {
	if u == nil {
		return true
	}
	return u.TotalNumber != total
}

// shouldRefreshLuoguCurve 只根据当前总做题数变化决定是否刷新洛谷曲线。
func shouldRefreshLuoguCurve(
	u *entity.LuoguUserDetail,
	passedCount int,
	passedLen int,
) bool {
	if u == nil {
		return true
	}
	if passedCount <= 0 {
		passedCount = passedLen
	}
	return u.PassedNumber != passedCount
}

func (s *OJService) publishOJDailyStatsProjectionEvent(
	ctx context.Context,
	userID uint,
	platform string,
	reset bool,
) error {
	if s == nil || s.ojDailyStatsProjectionSvc == nil || userID == 0 {
		return nil
	}

	kind := eventdto.OJDailyStatsProjectionKindRefreshRecentWindow
	if reset {
		kind = eventdto.OJDailyStatsProjectionKindResetAndRebuildRecentWindow
	}
	return s.ojDailyStatsProjectionSvc.PublishOJDailyStatsProjectionEvent(ctx, &eventdto.OJDailyStatsProjectionEvent{
		Kind:       kind,
		UserID:     userID,
		Platform:   platform,
		WindowDays: resolveOJDailyStatsRepairWindowDays(),
	})
}

// bindLeetCode 绑定LeetCode账号
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
		// 指针为空视为从未绑定，直接放行；否则检查冷却
		if existing.LastBindAt != nil && time.Since(*existing.LastBindAt) < time.Duration(coolDownHours)*time.Hour {
			return nil, svccontract.ErrBindCoolDown
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

	statsOut, err := infrastructure.LeetCode().SubmitStats(ctx, identifier, sleepSec)
	if err != nil {
		return nil, err
	}
	if statsOut == nil || !statsOut.OK {
		return nil, errors.New("leetcode submit stats request failed")
	}

	easy, medium, hard := extractLeetCodeCounts(statsOut)
	total := easy + medium + hard
	userSlug := strings.TrimSpace(out.Data.Profile.UserSlug)
	realName := strings.TrimSpace(out.Data.Profile.RealName)
	userAvatar := strings.TrimSpace(out.Data.Profile.UserAvatar)
	if userSlug == "" && realName == "" && userAvatar == "" && total == 0 {
		return nil, svccontract.ErrOJAccountNotBound
	}

	now := time.Now()
	detail := &entity.LeetcodeUserDetail{
		UserSlug:     userSlug,
		RealName:     realName,
		UserAvatar:   userAvatar,
		EasyNumber:   easy,
		MediumNumber: medium,
		HardNumber:   hard,
		TotalNumber:  total,
		UserID:       userID,
		LastBindAt:   &now,
	}
	resetCurve := existing == nil || strings.TrimSpace(existing.UserSlug) != userSlug
	if err := s.publishLeetcodeBindOutbox(ctx, userID); err != nil {
		return nil, err
	}
	saved, err := s.leetcodeRepo.UpsertByUserID(ctx, detail)
	if err != nil {
		return nil, err
	}
	if err := s.updateRankingCache(ctx, userID, "leetcode", saved.TotalNumber); err != nil {
		return nil, err
	}
	if err := s.publishOJDailyStatsProjectionEvent(ctx, userID, "leetcode", resetCurve); err != nil {
		global.Log.Error("failed to publish oj daily stats projection event",
			zap.Uint("user_id", userID),
			zap.String("platform", "leetcode"),
			zap.Bool("reset", resetCurve),
			zap.Error(err))
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

func (s *OJService) SyncAllLeetcodeUsers(ctx context.Context) error { // 同步所有已绑定的力扣用户
	lockKey := redislock.LockKeyLeetcodeSyncAllUsers                       // 生成全局同步锁键
	err := redislock.WithLock(ctx, lockKey, 30*time.Second, func() error { // 全局互斥，防止并发全量同步
		return s.syncAllLeetcodeUsersLocked(ctx) // 执行带锁的全量同步
	})
	if err != nil && errors.Is(err, redislock.ErrLockFailed) { // 判断是否为锁竞争失败
		global.Log.Info("leetcode sync skipped: lock is held", zap.String("lock_key", lockKey)) // 记录跳过日志
		return nil                                                                              // 忽略锁占用导致的跳过
	}
	return err // 返回真实错误或 nil
}

func (s *OJService) syncAllLeetcodeUsersLocked(ctx context.Context) error { // 全量同步具体实现
	users, err := s.leetcodeRepo.GetAll(ctx) // 拉取全部已绑定用户
	if err != nil {                          // 数据访问异常
		return err // 向上返回错误
	}
	return s.syncLeetcodeUsersWithRateLimit(ctx, users) // 按频率逐个同步
}

func (s *OJService) syncLuoguUsersWithRateLimit(
	ctx context.Context,
	users []*entity.LuoguUserDetail,
) error {
	activeUsers, err := s.buildActiveUserSetFromLuoguDetails(ctx, users)
	if err != nil {
		return err
	}

	intervalSeconds := global.Config.Task.LuoguSyncUserIntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 10
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	for _, u := range users {
		if u == nil || !activeUsers[u.UserID] {
			continue
		}
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

func (s *OJService) syncLeetcodeUsersWithRateLimit( // 带限频的力扣用户同步
	ctx context.Context, // 请求上下文
	users []*entity.LeetcodeUserDetail, // 待同步用户列表
) error {
	activeUsers, err := s.buildActiveUserSetFromLeetcodeDetails(ctx, users)
	if err != nil {
		return err
	}

	intervalSeconds := global.Config.Task.LeetcodeSyncUserIntervalSeconds // 读取用户间隔配置
	if intervalSeconds <= 0 {                                             // 兜底默认值
		intervalSeconds = 10 // 默认 10 秒
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second) // 构造节流器
	defer ticker.Stop()                                                    // 释放定时器资源

	for _, u := range users { // 遍历用户列表
		if u == nil || !activeUsers[u.UserID] { // 跳过空指针和禁用用户
			continue // 继续下一位用户
		}
		if err := s.waitTickerOrCancel(ctx, ticker); err != nil { // 等待节流或取消
			global.Log.Info("sync leetcode users canceled", zap.Error(err)) // 记录取消日志
			return err                                                      // 终止同步流程
		}

		identifier := strings.TrimSpace(u.UserSlug) // 提取用户 slug 作为锁粒度
		if identifier == "" {                       // slug 为空时兜底
			identifier = strconv.FormatUint(uint64(u.UserID), 10) // 使用用户 ID 兜底
		}
		userLockKey := redislock.LockKeyLeetcodeSyncSingleUser(identifier)         // 生成用户级锁键
		err := redislock.WithLock(ctx, userLockKey, 10*time.Second, func() error { // 加用户级锁
			return s.syncSingleLeetcodeUser(ctx, u) // 执行单用户同步
		})

		if err != nil { // 处理单用户同步错误
			if errors.Is(err, redislock.ErrLockFailed) { // 锁被占用说明正在绑定或同步
				global.Log.Warn("skip syncing user: lock held (possibly binding)", zap.String("slug", u.UserSlug)) // 记录跳过
				continue                                                                                           // 跳过该用户
			}
			global.Log.Error("failed to sync leetcode user", zap.String("slug", u.UserSlug), zap.Error(err)) // 记录错误
			continue                                                                                         // 保持任务继续
		}
	}

	return nil // 全部处理完成
}

func (s *OJService) waitTickerOrCancel(ctx context.Context, ticker *time.Ticker) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		return nil
	}
}

func (s *OJService) buildActiveUserSetFromLuoguDetails(
	ctx context.Context,
	details []*entity.LuoguUserDetail,
) (map[uint]bool, error) {
	userIDs := make([]uint, 0, len(details))
	for _, item := range details {
		if item != nil && item.UserID > 0 {
			userIDs = append(userIDs, item.UserID)
		}
	}
	return s.buildActiveUserSet(ctx, userIDs)
}

func (s *OJService) buildActiveUserSetFromLeetcodeDetails(
	ctx context.Context,
	details []*entity.LeetcodeUserDetail,
) (map[uint]bool, error) {
	userIDs := make([]uint, 0, len(details))
	for _, item := range details {
		if item != nil && item.UserID > 0 {
			userIDs = append(userIDs, item.UserID)
		}
	}
	return s.buildActiveUserSet(ctx, userIDs)
}

func (s *OJService) buildActiveUserSet(ctx context.Context, userIDs []uint) (map[uint]bool, error) {
	set := make(map[uint]bool)
	if len(userIDs) == 0 {
		return set, nil
	}
	users, err := s.userRepo.GetByIDsActive(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u != nil {
			set[u.ID] = true
		}
	}
	return set, nil
}

func (s *OJService) syncSingleLeetcodeUser( // 单用户力扣同步
	ctx context.Context, // 请求上下文
	u *entity.LeetcodeUserDetail, // 用户详情
) error {
	if u == nil { // 校验用户详情是否为空
		return errors.New("nil leetcode user") // 返回参数错误
	}
	identifier := strings.TrimSpace(u.UserSlug) // 取用户 slug 作为标识
	if identifier == "" {                       // 校验标识合法性
		return svccontract.ErrInvalidIdentifier // 返回无效标识错误
	}

	out, err := infrastructure.LeetCode().PublicProfile(ctx, identifier, 0) // 拉取公开资料
	if err != nil {                                                         // 处理请求错误
		return err // 向上返回错误
	}
	if out == nil || !out.OK { // 校验响应有效性
		return errors.New("leetcode api response not ok") // 返回响应异常错误
	}

	statsOut, err := infrastructure.LeetCode().SubmitStats(ctx, identifier, 0) // 拉取提交统计
	if err != nil {
		return err
	}
	if statsOut == nil || !statsOut.OK {
		return errors.New("leetcode submit stats response not ok")
	}

	easy, medium, hard := extractLeetCodeCounts(statsOut)    // 解析题目难度统计
	total := easy + medium + hard                            // 计算总解题数
	realName := strings.TrimSpace(out.Data.Profile.RealName) // 提取真实姓名
	avatar := strings.TrimSpace(out.Data.Profile.UserAvatar) // 提取头像
	userSlug := strings.TrimSpace(out.Data.Profile.UserSlug) // 提取标准 slug
	if userSlug == "" {                                      // slug 为空时兜底
		userSlug = identifier // 使用输入标识
	}
	curveShouldRefresh := false
	if shouldRefreshLeetcodeCurve(u, total) {
		curveShouldRefresh = true
	}

	if err := s.updateLeetcodeUserInfoIfChanged(ctx, u, userSlug, realName, avatar, easy, medium, hard, total); err != nil { // 写入用户详情
		global.Log.Error("failed to update leetcode user info", zap.Error(err)) // 记录失败日志
	}
	if err := s.updateRankingCache(ctx, u.UserID, "leetcode", total); err != nil { // 发布排行榜投影事件
		global.Log.Error("failed to publish leetcode ranking projection", zap.Error(err)) // 记录失败日志
	}

	recentOut, err := infrastructure.LeetCode().RecentAC(ctx, identifier, 0) // 拉取最近 AC 题目
	if err != nil {                                                          // 处理请求错误
		global.Log.Error("failed to fetch leetcode recent ac", zap.Error(err)) // 记录请求错误
		return nil                                                             // 不阻断后续任务
	}
	if recentOut == nil || !recentOut.OK { // 校验响应状态
		global.Log.Error("leetcode recent ac response not ok") // 记录响应异常
		return nil                                             // 不阻断后续任务
	}

	if err := s.upsertLeetcodeProblems(ctx, recentOut.Data.RecentAccepted); err != nil { // 同步题库
		global.Log.Error("failed to upsert leetcode problems", zap.Error(err)) // 记录题库同步失败
	}
	newRecords, err := s.syncLeetcodeUserSolvedRelations(ctx, u.ID, recentOut.Data.RecentAccepted)
	if err != nil { // 同步做题关系
		global.Log.Error("failed to sync leetcode user relations", zap.Error(err)) // 记录关系同步失败
	} else if newRecords > 0 {
		curveShouldRefresh = true
	}
	if curveShouldRefresh {
		if err := s.publishOJDailyStatsProjectionEvent(ctx, u.UserID, "leetcode", false); err != nil {
			global.Log.Error("failed to publish oj daily stats projection event",
				zap.Uint("user_id", u.UserID),
				zap.String("platform", "leetcode"),
				zap.Error(err))
		}
	}

	return nil // 正常结束
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
	curveShouldRefresh := shouldRefreshLuoguCurve(
		u,
		out.Data.PassedCount,
		len(out.Data.Passed),
	)

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
		global.Log.Error("failed to publish luogu ranking projection", zap.Error(err))
	}
	if curveShouldRefresh || newRecords > 0 {
		if err := s.publishOJDailyStatsProjectionEvent(ctx, u.UserID, "luogu", false); err != nil {
			global.Log.Error("failed to publish oj daily stats projection event",
				zap.Uint("user_id", u.UserID),
				zap.String("platform", "luogu"),
				zap.Error(err))
		}
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
		return nil, svccontract.ErrOJAccountNotBound
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

func (s *OJService) upsertLeetcodeProblems( // 写入 LeetCode 题库（去重 + 批量）
	ctx context.Context, // 请求上下文
	passed []lc.RecentAcceptedItem, // 最近 AC 列表
) error {
	var newProblems []*entity.LeetcodeQuestionBank // 待插入题目集合
	seenInBatch := make(map[string]struct{})       // 批次内去重集合

	for _, p := range passed { // 遍历 AC 题目
		slug := strings.TrimSpace(p.Slug) // 提取题目 slug
		if slug == "" {                   // 跳过空 slug
			continue // 继续处理下一条
		}

		id, ok, err := s.leetcodeQuestionBankRepo.GetCachedID(ctx, slug) // 查询缓存
		if err == nil && ok && id > 0 {                                  // 缓存命中直接跳过
			continue // 继续处理下一条
		}
		if _, seen := seenInBatch[slug]; seen { // 批次内重复
			continue // 继续处理下一条
		}
		q, err := s.leetcodeQuestionBankRepo.GetByTitleSlug(ctx, slug) // 查询数据库
		if err == nil && q.ID > 0 {                                    // 数据库命中
			_ = s.leetcodeQuestionBankRepo.CacheID(ctx, slug, q.ID) // 回填缓存
			continue                                                // 继续处理下一条
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { // 非未找到错误
			return err // 向上返回错误
		}
		newProblems = append(newProblems, &entity.LeetcodeQuestionBank{ // 组装新题目
			TitleSlug: slug,                       // 标题 slug
			Title:     strings.TrimSpace(p.Title), // 标题
		})
		seenInBatch[slug] = struct{}{} // 记录批次内已处理
	}

	if len(newProblems) == 0 { // 无新增题目
		return nil // 直接返回
	}
	if err := s.leetcodeQuestionBankRepo.BatchCreate(ctx, newProblems); err != nil { // 批量插入
		return err // 向上返回错误
	}
	global.Log.Info("added new leetcode problems", zap.Int("count", len(newProblems))) // 记录新增数量
	return nil                                                                         // 正常结束
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

func (s *OJService) syncLeetcodeUserSolvedRelations( // 同步 LeetCode 用户题目关系
	ctx context.Context, // 请求上下文
	leetcodeUserDetailID uint, // LeetCode 详情 ID
	passed []lc.RecentAcceptedItem, // 最近 AC 列表
) (int, error) {
	solvedSet, err := s.leetcodeUserQuestionRepo.GetSolvedProblemIDs(ctx, leetcodeUserDetailID) // 预加载已解题目
	if err != nil {                                                                             // 处理查询错误
		return 0, err // 向上返回错误
	}

	var newRelations []*entity.LeetcodeUserQuestion // 待插入关系集合
	seenInBatch := make(map[uint]struct{})          // 批次内去重集合
	for _, p := range passed {                      // 遍历 AC 题目
		slug := strings.TrimSpace(p.Slug) // 提取题目 slug
		if slug == "" {                   // 跳过空 slug
			continue // 继续处理下一条
		}
		qID, ok, err := s.leetcodeQuestionBankRepo.GetCachedID(ctx, slug) // 查询缓存
		if err != nil {                                                   // 忽略缓存异常
			continue // 继续处理下一条
		}
		if !ok || qID == 0 { // 缓存未命中
			q, err := s.leetcodeQuestionBankRepo.GetByTitleSlug(ctx, slug) // 查询数据库
			if err != nil {                                                // 处理查询错误
				if errors.Is(err, gorm.ErrRecordNotFound) { // 数据不存在则跳过
					continue // 继续处理下一条
				}
				return 0, err // 向上返回错误
			}
			qID = q.ID   // 取出题库 ID
			if qID > 0 { // 有效 ID 才回填缓存
				_ = s.leetcodeQuestionBankRepo.CacheID(ctx, slug, qID) // 回填缓存
			}
		}

		if qID == 0 { // 无效题库 ID
			continue // 继续处理下一条
		}
		if _, solved := solvedSet[qID]; solved { // 已存在关系
			continue // 继续处理下一条
		}
		if _, seen := seenInBatch[qID]; seen { // 批次内重复
			continue // 继续处理下一条
		}
		newRelations = append(newRelations, &entity.LeetcodeUserQuestion{ // 组装新关系
			LeetcodeUserDetailID: leetcodeUserDetailID, // 关联用户详情
			LeetcodeQuestionID:   qID,                  // 关联题库 ID
		})
		seenInBatch[qID] = struct{}{} // 记录批次内已处理
	}

	if len(newRelations) == 0 { // 无新增关系
		return 0, nil // 直接返回
	}
	if err := s.leetcodeUserQuestionRepo.BatchCreate(ctx, newRelations); err != nil { // 批量插入
		return 0, err // 向上返回错误
	}
	return len(newRelations), nil // 返回新增数量
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

func (s *OJService) updateLeetcodeUserInfoIfChanged( // 条件更新力扣用户详情
	ctx context.Context, // 请求上下文
	u *entity.LeetcodeUserDetail, // 现有用户详情
	userSlug string, // 目标用户 slug
	name string, // 目标用户名
	avatar string, // 目标头像
	easy int, // 简单题数
	medium int, // 中等题数
	hard int, // 困难题数
	total int, // 总题数
) error {
	if u == nil { // 校验用户详情是否存在
		return nil // 无需更新
	}

	if total <= 0 { // 总数为空时重新计算
		total = easy + medium + hard // 以分难度统计为准
	}

	if u.UserSlug == userSlug && // slug 未变化
		u.RealName == name && // 用户名未变化
		u.UserAvatar == avatar && // 头像未变化
		u.EasyNumber == easy && // 简单题未变化
		u.MediumNumber == medium && // 中等题未变化
		u.HardNumber == hard && // 困难题未变化
		u.TotalNumber == total { // 总数未变化
		return nil // 数据一致则直接返回
	}

	detail := &entity.LeetcodeUserDetail{ // 组装待写入数据
		UserSlug:     userSlug, // 写入 slug
		RealName:     name,     // 写入姓名
		UserAvatar:   avatar,   // 写入头像
		EasyNumber:   easy,     // 写入简单题
		MediumNumber: medium,   // 写入中等题
		HardNumber:   hard,     // 写入困难题
		TotalNumber:  total,    // 写入总数
		UserID:       u.UserID, // 关联用户 ID
	}
	_, err := s.leetcodeRepo.UpsertByUserID(ctx, detail) // 更新或插入详情
	return err                                           // 返回写入结果
}

func (s *OJService) updateRankingCache(
	ctx context.Context,
	userID uint,
	platform string,
	totalPassed int,
) error {
	_ = totalPassed
	return s.publishOJProfileProjectionEvent(ctx, userID, platform)
}

func (s *OJService) RebuildRankingCaches(ctx context.Context) error {
	if s.cacheProjectionSvc == nil {
		return nil
	}
	return s.cacheProjectionSvc.RebuildAll(ctx)
}

// GetRankingList 获取排行榜列表与当前用户排名
func (s *OJService) GetRankingList(
	ctx context.Context,
	userID uint,
	req *request.OJRankingListReq,
) (*resp.OJRankingListResp, error) {
	platform, page, pageSize, scope, orgID, err := normalizeRankingRequest(userID, req)
	if err != nil {
		return nil, err
	}
	if global.Redis == nil {
		return nil, errors.New("redis is not initialized")
	}

	// 获取请求用户信息和权限，确保用户有效且有权限访问排行榜
	requester, isSuperAdmin, err := s.getActiveRankingRequester(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 解析排行榜键，确保用户有权限访问对应范围的排行榜
	key, err := s.resolveRankingKey(ctx, requester, isSuperAdmin, platform, scope, orgID)
	if err != nil {
		return nil, err
	}

	// 从 Redis 获取排行榜数据，包含总数、分页起始位置和当前页数据范围
	total, start, ranges, err := fetchRankingRanges(ctx, key, page, pageSize)
	if err != nil {
		return nil, err
	}

	entries, userIDs := parseRankingEntries(ranges)
	projectionMap, err := s.loadRankingProjectionMap(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	list := buildRankingList(entries, int(start), platform, projectionMap)
	myRank, err := loadMyRank(ctx, key, userID)
	if err != nil {
		return nil, err
	}

	return &resp.OJRankingListResp{
		List:   list,
		MyRank: myRank,
		Total:  total,
	}, nil
}

// rankingEntry 表示排行榜单条数据的解析结果
type rankingEntry struct {
	UserID uint
	Score  int
}

const (
	rankingScopeCurrentOrg = "current_org"
	rankingScopeAllMembers = "all_members"
	rankingScopeOrg        = "org"
)

// normalizeRankingRequest 校验请求并规范分页与平台参数
func normalizeRankingRequest(
	userID uint,
	req *request.OJRankingListReq,
) (string, int, int, string, *uint, error) {
	if req == nil {
		return "", 0, 0, "", nil, errors.New("invalid request")
	}
	if userID == 0 {
		return "", 0, 0, "", nil, errors.New("invalid user id")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "luogu" && platform != "leetcode" {
		return "", 0, 0, "", nil, svccontract.ErrInvalidPlatform
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

	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = rankingScopeCurrentOrg
	}

	var orgID *uint
	if scope == rankingScopeOrg {
		if req.OrgID == nil || *req.OrgID == 0 {
			return "", 0, 0, "", nil, errors.New("org_id is required for org scope")
		}
		orgID = cloneUintPtr(req.OrgID)
	}

	return platform, page, pageSize, scope, orgID, nil
}

// getActiveRankingRequester 获取请求排行榜的用户信息，并判断是否为超级管理员
func (s *OJService) getActiveRankingRequester(
	ctx context.Context,
	userID uint,
) (*entity.User, bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	if user == nil {
		return nil, false, errors.New("user not found")
	}
	if user.Freeze || user.Status != consts.UserStatusActive {
		return nil, false, errors.New("user is disabled")
	}
	roles, err := s.roleRepo.GetUserGlobalRoles(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	for _, role := range roles {
		if role != nil && role.Code == consts.RoleCodeSuperAdmin {
			return user, true, nil
		}
	}
	return user, false, nil
}

// resolveRankingKey 根据排行榜范围解析对应的 Redis 键
func (s *OJService) resolveRankingKey(
	ctx context.Context,
	requester *entity.User,
	isSuperAdmin bool,
	platform string,
	scope string,
	orgID *uint,
) (string, error) {
	switch scope {
	// 全员范围直接使用全局排行榜键，无需组织校验
	case rankingScopeAllMembers:
		return rediskey.RankingAllMembersZSetKey(platform), nil
		// 组织范围需要校验组织 ID 和成员资格，并根据是否全员组织解析对应的排行榜键
	case rankingScopeOrg:
		if orgID == nil || *orgID == 0 {
			return "", errors.New("org_id is required for org scope")
		}
		if !isSuperAdmin {
			active, err := s.orgMemberRepo.IsUserActiveInOrg(ctx, requester.ID, *orgID)
			if err != nil {
				return "", err
			}
			if !active {
				return "", errors.New("user organization not active")
			}
		}
		return s.resolveOrgRankingKey(ctx, *orgID, platform)
	case rankingScopeCurrentOrg:
		if requester == nil || requester.CurrentOrgID == nil || *requester.CurrentOrgID == 0 {
			return "", errors.New("user organization not found")
		}
		if requester.CurrentOrg != nil && isAllMembersBuiltinOrg(requester.CurrentOrg) {
			return rediskey.RankingAllMembersZSetKey(platform), nil
		}
		active, err := s.orgMemberRepo.IsUserActiveInOrg(ctx, requester.ID, *requester.CurrentOrgID)
		if err != nil {
			return "", err
		}
		if !active {
			return "", errors.New("user organization not active")
		}
		return s.resolveOrgRankingKey(ctx, *requester.CurrentOrgID, platform)
	default:
		return "", errors.New("invalid ranking scope")
	}
}

// resolveOrgRankingKey 根据组织 ID 解析对应的排行榜 Redis 键
func (s *OJService) resolveOrgRankingKey(
	ctx context.Context,
	orgID uint,
	platform string,
) (string, error) {
	// 查询组织信息，判断是否为全员组织
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return "", err
	}
	if org == nil {
		return "", errors.New("organization not found")
	}
	// 全员组织使用全局排行榜键，非全员组织使用特定组织排行榜键
	if isAllMembersBuiltinOrg(org) {
		return rediskey.RankingAllMembersZSetKey(platform), nil
	}

	// 非全员组织使用特定组织排行榜键
	return rediskey.RankingOrgZSetKey(orgID, platform), nil
}

// fetchRankingRanges 从 Redis 拉取分页排行榜数据
func fetchRankingRanges(
	ctx context.Context,
	key string,
	page int,
	pageSize int,
) (int64, int64, []redis.Z, error) {
	total, err := global.Redis.ZCard(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, nil, err
	}

	start := int64((page - 1) * pageSize)
	stop := start + int64(pageSize) - 1
	ranges, err := global.Redis.ZRevRangeWithScores(ctx, key, start, stop).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, 0, nil, err
	}

	return total, start, ranges, nil
}

// parseRankingEntries 解析 ZSet 结果为有序的用户列表
func parseRankingEntries(ranges []redis.Z) ([]rankingEntry, []uint) {
	entries := make([]rankingEntry, 0, len(ranges))
	userIDs := make([]uint, 0, len(ranges))
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
		entries = append(entries, rankingEntry{UserID: uid, Score: int(item.Score)})
		userIDs = append(userIDs, uid)
	}
	return entries, userIDs
}

// loadUserMap 批量加载用户信息并构建映射
func (s *OJService) loadRankingProjectionMap(
	ctx context.Context,
	userIDs []uint,
) (map[uint]*rankingcache.UserProjection, error) {
	projectionMap := make(map[uint]*rankingcache.UserProjection)
	if len(userIDs) == 0 {
		return projectionMap, nil
	}

	pipe := global.Redis.Pipeline()
	cacheReads := make(map[uint]*redis.StringStringMapCmd, len(userIDs))
	for _, userID := range userIDs {
		cacheReads[userID] = pipe.HGetAll(ctx, rediskey.RankingUserHashKey(userID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}

	missingUserIDs := make([]uint, 0)
	for _, userID := range userIDs {
		values, err := cacheReads[userID].Result()
		if err != nil {
			return nil, err
		}
		if len(values) == 0 {
			missingUserIDs = append(missingUserIDs, userID)
			continue
		}
		projection, ok := rankingcache.ProjectionFromHash(userID, values)
		if !ok {
			missingUserIDs = append(missingUserIDs, userID)
			continue
		}
		projectionMap[userID] = projection
	}

	if len(missingUserIDs) == 0 {
		return projectionMap, nil
	}

	items, err := s.rankingReadModelRepo.GetByUserIDs(ctx, missingUserIDs)
	if err != nil {
		return nil, err
	}

	cacheBackfills := make([]*rankingcache.UserProjection, 0, len(items))
	for _, item := range items {
		projection := rankingcache.FromReadModel(item)
		if projection == nil {
			continue
		}
		cacheBackfills = append(cacheBackfills, projection)
		projectionMap[projection.UserID] = projection
	}
	if err := rankingcache.WriteProjections(ctx, global.Redis, cacheBackfills); err != nil {
		return nil, err
	}
	return projectionMap, nil
}

// buildRankingList 组装排行榜列表，用户名统一使用 user.username
func buildRankingList(
	entries []rankingEntry,
	start int,
	platform string,
	projectionMap map[uint]*rankingcache.UserProjection,
) []*resp.OJRankingListItem {
	list := make([]*resp.OJRankingListItem, 0, len(entries))
	for _, entry := range entries {
		projection := projectionMap[entry.UserID]
		if projection == nil || !projection.Active {
			continue
		}
		profile := projection.Platform(platform)
		if strings.TrimSpace(profile.Identifier) == "" {
			continue
		}
		avatar := strings.TrimSpace(profile.Avatar)
		if avatar == "" {
			avatar = projection.Avatar
		}
		item := &resp.OJRankingListItem{
			Rank:        start + len(list) + 1,
			UserID:      entry.UserID,
			RealName:    projection.Username,
			Avatar:      avatar,
			TotalPassed: entry.Score,
		}
		if platform == "luogu" {
			item.PlatformDetails = &resp.OJRankingPlatformDetails{
				Luogu: entry.Score,
			}
		} else {
			item.PlatformDetails = &resp.OJRankingPlatformDetails{
				Leetcode: entry.Score,
			}
		}
		list = append(list, item)
	}
	return list
}

// loadMyRank 获取当前用户在排行榜中的名次与分数
func loadMyRank(
	ctx context.Context,
	key string,
	userID uint,
) (*resp.OJRankingMyRank, error) {
	member := strconv.FormatUint(uint64(userID), 10)
	rank, err := global.Redis.ZRevRank(ctx, key, member).Result()
	if err == nil {
		score, scoreErr := global.Redis.ZScore(ctx, key, member).Result()
		if scoreErr == nil {
			return &resp.OJRankingMyRank{
				Rank:        int(rank) + 1,
				TotalPassed: int(score),
			}, nil
		}
		return nil, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, err
	}
	return nil, nil
}

func (s *OJService) publishOJProfileProjectionEvent(
	ctx context.Context,
	userID uint,
	platform string,
) error {
	if userID == 0 || s.cacheProjectionPublisher == nil {
		return nil
	}
	return s.cacheProjectionPublisher.Publish(ctx, newOJProfileChangedProjectionEvent(userID, platform))
}

func (s *OJService) HandleLuoguBindPayload(
	ctx context.Context,
	userID uint,
	payload *eventdto.LuoguBindPayload,
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

		newRecords, err := s.syncLuoguUserSolvedRelations(ctx, detail.ID, payload.Passed)
		if err != nil {
			return err
		}
		if newRecords > 0 {
			if err := s.publishOJDailyStatsProjectionEvent(ctx, userID, "luogu", false); err != nil {
				global.Log.Error("failed to publish oj daily stats projection event",
					zap.Uint("user_id", userID),
					zap.String("platform", "luogu"),
					zap.Error(err))
			}
		}

		return nil
	})
}

func (s *OJService) HandleLeetcodeBindSignal( // 处理 LeetCode 绑定后的异步信号
	ctx context.Context, // 请求上下文
	userID uint, // 业务用户 ID
) error {
	if userID == 0 { // 校验用户 ID
		return errors.New("invalid leetcode user id") // 返回非法用户错误
	}
	detail, err := s.leetcodeRepo.GetByUserID(ctx, userID) // 读取绑定的 LeetCode 详情
	if err != nil {                                        // 处理仓储错误
		return err // 向上返回错误
	}
	if detail == nil { // 判断是否已绑定
		return errors.New("leetcode user detail not found") // 绑定记录不存在
	}
	identifier := strings.TrimSpace(detail.UserSlug) // 取出并清理用户标识
	if identifier == "" {                            // 校验标识
		return errors.New("leetcode user slug missing") // 标识缺失
	}

	out, err := infrastructure.LeetCode().RecentAC(ctx, identifier, 0) // 拉取最近 AC 列表
	if err != nil {                                                    // 处理请求错误
		return err // 向上返回错误
	}
	if out == nil || !out.OK { // 校验响应状态
		return errors.New("leetcode recent ac request failed") // 请求失败
	}

	if err := s.upsertLeetcodeProblems(ctx, out.Data.RecentAccepted); err != nil { // 写入题库
		return err // 向上返回错误
	}
	newRecords, err := s.syncLeetcodeUserSolvedRelations(ctx, detail.ID, out.Data.RecentAccepted)
	if err != nil { // 同步用户题目关系
		return err // 向上返回错误
	}
	if newRecords > 0 {
		if err := s.publishOJDailyStatsProjectionEvent(ctx, userID, "leetcode", false); err != nil {
			global.Log.Error("failed to publish oj daily stats projection event",
				zap.Uint("user_id", userID),
				zap.String("platform", "leetcode"),
				zap.Error(err))
		}
	}
	return nil // 正常结束
}

func extractLeetCodeCounts(out *lc.SubmitStatsResponse) (int, int, int) {
	if out == nil {
		return 0, 0, 0
	}

	// 优先使用 stats.userProfileUserQuestionProgress.numAcceptedQuestions
	// 这是 LeetCode 新版接口返回的准确数据，不包含重复提交
	list := out.Data.Stats.UserProfileUserQuestionProgress.NumAcceptedQuestions

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
	if existing.LastBindAt != nil && time.Since(*existing.LastBindAt) < time.Duration(coolDownHours)*time.Hour {
		return svccontract.ErrBindCoolDown
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
		return nil, svccontract.ErrInvalidIdentifier
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
		return nil, svccontract.ErrOJAccountNotBound
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
		// 发布失败，则直接回退，不要入库
		err := s.publishLuoguBindOutbox(ctx, userID, out.Data.Passed)
		if err != nil {
			return err
		}
		passed := out.Data.PassedCount
		if passed <= 0 {
			passed = len(out.Data.Passed)
		}

		now := time.Now()
		detail := &entity.LuoguUserDetail{
			Identification: identifier,
			RealName:       strings.TrimSpace(out.Data.User.Name),
			UserAvatar:     strings.TrimSpace(out.Data.User.Avatar),
			PassedNumber:   passed,
			UserID:         userID,
			LastBindAt:     &now,
		}

		var upsertErr error
		saved, upsertErr = s.luoguRepo.UpsertByUserID(ctx, detail)
		if upsertErr != nil {
			return upsertErr
		}

		if err := s.updateRankingCache(ctx, userID, "luogu", passed); err != nil {
			return err
		}
		if err := s.publishOJDailyStatsProjectionEvent(ctx, userID, "luogu", true); err != nil {
			global.Log.Error("failed to publish oj daily stats projection event",
				zap.Uint("user_id", userID),
				zap.String("platform", "luogu"),
				zap.Bool("reset", true),
				zap.Error(err))
		}

		return nil
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

	payload := &eventdto.LuoguBindPayload{
		Passed: passed,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)

	event := &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(userID), 10),
		AggregateType: "luogu_user",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}
	if err := s.outboxRepo.Create(ctx, event); err != nil {
		return err
	}
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil {
		global.Log.Warn("luogu bind notify outbox failed", zap.Error(err))
	}
	return nil
}

func (s *OJService) publishLeetcodeBindOutbox(
	ctx context.Context,
	userID uint,
) error {
	topic := strings.TrimSpace(global.Config.Messaging.LeetcodeBindTopic)
	if topic == "" {
		return errors.New("leetcode bind topic config is empty")
	}

	payloadBytes, err := json.Marshal(struct{}{})
	if err != nil {
		return err
	}
	ids, traceparent, tracestate := extractOutboxTraceFields(ctx)

	event := &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(userID), 10),
		AggregateType: "leetcode_user",
		Payload:       string(payloadBytes),
		TraceID:       ids.TraceID,
		RequestID:     ids.RequestID,
		TraceParent:   traceparent,
		TraceState:    tracestate,
	}
	if err := s.outboxRepo.Create(ctx, event); err != nil {
		return err
	}
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil {
		global.Log.Warn("leetcode bind notify outbox failed", zap.Error(err))
	}
	return nil
}

func extractOutboxTraceFields(ctx context.Context) (contextid.IDs, string, string) {
	ids := contextid.FromContext(ctx)
	traceparent := ""
	tracestate := ""
	if tc, ok := contextid.TraceContextFromContext(ctx); ok {
		traceparent = strings.TrimSpace(w3c.BuildTraceparent(tc))
		tracestate = strings.TrimSpace(tc.TraceState)
	}
	return ids, traceparent, tracestate
}
