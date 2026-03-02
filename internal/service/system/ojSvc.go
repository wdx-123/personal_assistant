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
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/redislock"

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
	userRepo                 interfaces.UserRepository
	orgRepo                  interfaces.OrgRepository
	leetcodeRepo             interfaces.LeetcodeUserDetailRepository
	luoguRepo                interfaces.LuoguUserDetailRepository
	leetcodeQuestionBankRepo interfaces.LeetcodeQuestionBankRepository
	luoguQuestionBankRepo    interfaces.LuoguQuestionBankRepository
	leetcodeUserQuestionRepo interfaces.LeetcodeUserQuestionRepository
	luoguUserQuestionRepo    interfaces.LuoguUserQuestionRepository
	outboxRepo               interfaces.OutboxRepository
}

func NewOJService(repositoryGroup *repository.Group) *OJService {
	return &OJService{
		userRepo:                 repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		orgRepo:                  repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
		leetcodeRepo:             repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		luoguRepo:                repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
		leetcodeQuestionBankRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeQuestionBankRepository(),
		luoguQuestionBankRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguQuestionBankRepository(),
		leetcodeUserQuestionRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserQuestionRepository(),
		luoguUserQuestionRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguUserQuestionRepository(),
		outboxRepo:               repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
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

	const sleepSec = 0.2

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
		return nil, ErrOJAccountNotBound
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

func (s *OJService) syncLeetcodeUsersWithRateLimit( // 带限频的力扣用户同步
	ctx context.Context, // 请求上下文
	users []*entity.LeetcodeUserDetail, // 待同步用户列表
) error {
	intervalSeconds := global.Config.Task.LeetcodeSyncUserIntervalSeconds // 读取用户间隔配置
	if intervalSeconds <= 0 {                                             // 兜底默认值
		intervalSeconds = 10 // 默认 10 秒
	}
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second) // 构造节流器
	defer ticker.Stop()                                                    // 释放定时器资源

	for _, u := range users { // 遍历用户列表
		if u == nil { // 跳过空指针用户
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

func (s *OJService) syncSingleLeetcodeUser( // 单用户力扣同步
	ctx context.Context, // 请求上下文
	u *entity.LeetcodeUserDetail, // 用户详情
) error {
	if u == nil { // 校验用户详情是否为空
		return errors.New("nil leetcode user") // 返回参数错误
	}
	identifier := strings.TrimSpace(u.UserSlug) // 取用户 slug 作为标识
	if identifier == "" {                       // 校验标识合法性
		return ErrInvalidIdentifier // 返回无效标识错误
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

	if err := s.updateLeetcodeUserInfoIfChanged(ctx, u, userSlug, realName, avatar, easy, medium, hard, total); err != nil { // 写入用户详情
		global.Log.Error("failed to update leetcode user info", zap.Error(err)) // 记录失败日志
	}
	if err := s.updateRankingCache(ctx, u.UserID, "leetcode", total); err != nil { // 刷新排行榜缓存
		global.Log.Error("failed to update leetcode ranking cache", zap.Error(err)) // 记录失败日志
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
	if _, err := s.syncLeetcodeUserSolvedRelations(ctx, u.ID, recentOut.Data.RecentAccepted); err != nil { // 同步做题关系
		global.Log.Error("failed to sync leetcode user relations", zap.Error(err)) // 记录关系同步失败
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

	// Optimization: Check if score is already consistent
	currentScore, err := global.Redis.ZScore(ctx, key, member).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if err == nil && currentScore == score {
		return nil
	}

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

// GetRankingList 获取排行榜列表与当前用户排名
func (s *OJService) GetRankingList(
	ctx context.Context,
	userID uint,
	req *request.OJRankingListReq,
) (*resp.OJRankingListResp, error) {
	platform, page, pageSize, err := normalizeRankingRequest(userID, req)
	if err != nil {
		return nil, err
	}

	key, err := s.getRankingKey(ctx, userID, platform)
	if err != nil {
		return nil, err
	}

	total, start, ranges, err := fetchRankingRanges(ctx, key, page, pageSize)
	if err != nil {
		return nil, err
	}

	entries, userIDs := parseRankingEntries(ranges)
	userMap, err := s.loadUserMap(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	luoguMap, leetcodeMap, err := s.loadPlatformDetailMaps(ctx, platform, userIDs)
	if err != nil {
		return nil, err
	}

	list := buildRankingList(entries, int(start), platform, userMap, luoguMap, leetcodeMap)
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

// normalizeRankingRequest 校验请求并规范分页与平台参数
func normalizeRankingRequest(
	userID uint,
	req *request.OJRankingListReq,
) (string, int, int, error) {
	if req == nil {
		return "", 0, 0, errors.New("invalid request")
	}
	if userID == 0 {
		return "", 0, 0, errors.New("invalid user id")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform != "luogu" && platform != "leetcode" {
		return "", 0, 0, ErrInvalidPlatform
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

	return platform, page, pageSize, nil
}

// getRankingKey 根据用户组织与平台生成排行榜 Redis Key
func (s *OJService) getRankingKey(
	ctx context.Context,
	userID uint,
	platform string,
) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil || user.CurrentOrgID == nil || *user.CurrentOrgID == 0 {
		return "", errors.New("user organization not found")
	}
	return rediskey.RankingZSetKey(*user.CurrentOrgID, platform), nil
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
func (s *OJService) loadUserMap(
	ctx context.Context,
	userIDs []uint,
) (map[uint]*entity.User, error) {
	userMap := make(map[uint]*entity.User)
	if len(userIDs) == 0 {
		return userMap, nil
	}
	users, err := s.userRepo.GetByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u != nil {
			userMap[u.ID] = u
		}
	}
	return userMap, nil
}

// loadPlatformDetailMaps 按平台批量加载用户详情映射
func (s *OJService) loadPlatformDetailMaps(
	ctx context.Context,
	platform string,
	userIDs []uint,
) (map[uint]*entity.LuoguUserDetail, map[uint]*entity.LeetcodeUserDetail, error) {
	luoguMap := make(map[uint]*entity.LuoguUserDetail)
	leetcodeMap := make(map[uint]*entity.LeetcodeUserDetail)
	if len(userIDs) == 0 {
		return luoguMap, leetcodeMap, nil
	}
	if platform == "luogu" {
		details, err := s.luoguRepo.GetByUserIDs(ctx, userIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, d := range details {
			if d != nil {
				luoguMap[d.UserID] = d
			}
		}
		return luoguMap, leetcodeMap, nil
	}
	details, err := s.leetcodeRepo.GetByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	for _, d := range details {
		if d != nil {
			leetcodeMap[d.UserID] = d
		}
	}
	return luoguMap, leetcodeMap, nil
}

// buildRankingList 组装排行榜列表，用户名统一使用 user.username
func buildRankingList(
	entries []rankingEntry,
	start int,
	platform string,
	userMap map[uint]*entity.User,
	luoguMap map[uint]*entity.LuoguUserDetail,
	leetcodeMap map[uint]*entity.LeetcodeUserDetail,
) []*resp.OJRankingListItem {
	list := make([]*resp.OJRankingListItem, 0, len(entries))
	for idx, entry := range entries {
		uid := entry.UserID
		score := entry.Score
		realName := ""
		avatar := ""
		if platform == "luogu" {
			if detail := luoguMap[uid]; detail != nil {
				avatar = detail.UserAvatar
			}
		} else {
			if detail := leetcodeMap[uid]; detail != nil {
				avatar = detail.UserAvatar
			}
		}
		if u := userMap[uid]; u != nil {
			realName = u.Username
			if avatar == "" {
				avatar = u.Avatar
			}
		}
		item := &resp.OJRankingListItem{
			Rank:        start + idx + 1,
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
	if _, err := s.syncLeetcodeUserSolvedRelations(ctx, detail.ID, out.Data.RecentAccepted); err != nil { // 同步用户题目关系
		return err // 向上返回错误
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

	event := &entity.OutboxEvent{
		EventID:       uuid.New().String(),
		EventType:     topic,
		AggregateID:   strconv.FormatUint(uint64(userID), 10),
		AggregateType: "leetcode_user",
		Payload:       string(payloadBytes),
	}
	if err := s.outboxRepo.Create(ctx, event); err != nil {
		return err
	}
	if err := outbox.NotifyNewOutboxEvent(ctx, global.Redis); err != nil {
		global.Log.Warn("leetcode bind notify outbox failed", zap.Error(err))
	}
	return nil
}
