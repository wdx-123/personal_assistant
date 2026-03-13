package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"personal_assistant/global"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"

	"go.uber.org/zap"
)

const (
	defaultOJDailyStatsRepairWindowDays = 35
	defaultOJDailyStatsRepairBatchSize  = 100
)

// OJDailyStatsProjectionService 负责处理 OJ 每日统计数据的投影事件
type OJDailyStatsProjectionService struct {
	userRepo                 interfaces.UserRepository
	leetcodeDetailRepo       interfaces.LeetcodeUserDetailRepository
	luoguDetailRepo          interfaces.LuoguUserDetailRepository
	leetcodeUserQuestionRepo interfaces.LeetcodeUserQuestionRepository
	luoguUserQuestionRepo    interfaces.LuoguUserQuestionRepository
	ojDailyStatsRepo         interfaces.OJDailyStatsRepository
	projectionEventPublisher ojDailyStatsProjectionEventPublisher
}

func NewOJDailyStatsProjectionService(repositoryGroup *repository.Group) *OJDailyStatsProjectionService {
	return &OJDailyStatsProjectionService{
		userRepo:                 repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		leetcodeDetailRepo:       repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserDetailRepository(),
		luoguDetailRepo:          repositoryGroup.SystemRepositorySupplier.GetLuoguUserDetailRepository(),
		leetcodeUserQuestionRepo: repositoryGroup.SystemRepositorySupplier.GetLeetcodeUserQuestionRepository(),
		luoguUserQuestionRepo:    repositoryGroup.SystemRepositorySupplier.GetLuoguUserQuestionRepository(),
		ojDailyStatsRepo:         repositoryGroup.SystemRepositorySupplier.GetOJDailyStatsRepository(),
		projectionEventPublisher: newOJDailyStatsProjectionOutboxPublisher(
			repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		),
	}
}

// PublishOJDailyStatsProjectionEvent 发布 OJ 每日统计投影事件，事件会被投递到 Outbox 以保证可靠投递
func (s *OJDailyStatsProjectionService) PublishOJDailyStatsProjectionEvent(
	ctx context.Context,
	event *eventdto.OJDailyStatsProjectionEvent,
) error {
	if s == nil || s.projectionEventPublisher == nil {
		return nil
	}
	return s.projectionEventPublisher.PublishOJDailyStatsProjectionEvent(ctx, event)
}

// HandleOJDailyStatsProjectionEvent 处理 OJ 每日统计投影事件，根据事件内容重建指定用户和平台的最近窗口数据
func (s *OJDailyStatsProjectionService) HandleOJDailyStatsProjectionEvent(
	ctx context.Context,
	event *eventdto.OJDailyStatsProjectionEvent,
) error {
	if event == nil {
		return errors.New("nil oj daily stats projection event")
	}
	// 如果事件类型是重置并重建最近窗口，则在重建时删除最近窗口内的旧数据；否则保留旧数据，仅补充缺失数据
	reset := strings.TrimSpace(event.Kind) == eventdto.OJDailyStatsProjectionKindResetAndRebuildRecentWindow
	return s.RebuildRecentWindow(ctx, event.UserID, event.Platform, reset)
}

// RebuildRecentWindow 重建指定用户和平台的最近窗口数据，适用于数据不完整或错误的情况。
// 会删除最近窗口内的旧数据，并根据用户详情和题目记录重新计算生成新数据。
func (s *OJDailyStatsProjectionService) RebuildRecentWindow(
	ctx context.Context,
	userID uint,
	platform string,
	reset bool,
) error {
	platform = normalizeOJDailyStatsPlatform(platform)
	if userID == 0 || platform == "" {
		return errors.New("invalid oj daily stats rebuild input")
	}
	if s == nil {
		return errors.New("nil oj daily stats projection service")
	}

	if reset {
		if err := s.ojDailyStatsRepo.DeleteByUserPlatform(ctx, userID, platform); err != nil {
			return err
		}
	}

	// 修复窗口默认为最近 35 天，既包含今天在内的连续日期范围。可以通过配置调整窗口大小以覆盖更长或更短的历史数据。
	windowDays := resolveOJDailyStatsRepairWindowDays()
	// 构建最近窗口的日期范围，startDate 是窗口开始日期，endDateExclusive 是窗口结束日期（不包含）
	startDate, endDateExclusive := buildOJDailyStatsWindowRange(windowDays)
	var (
		currentTotal     int                          // 当前累计总数
		sourceUpdatedAt  time.Time                    // 数据来源的更新时间
		dateSolvedCounts []*readmodel.DateSolvedCount // 最近窗口内每天的新增题目数量
		err              error
	)

	switch platform {
	case "leetcode":
		detail, detailErr := s.leetcodeDetailRepo.GetByUserID(ctx, userID)
		if detailErr != nil {
			return detailErr
		}
		if detail == nil {
			return s.ojDailyStatsRepo.DeleteByUserPlatform(ctx, userID, platform)
		}
		currentTotal = detail.TotalNumber
		sourceUpdatedAt = detail.UpdatedAt
		dateSolvedCounts, err = s.leetcodeUserQuestionRepo.CountSolvedByDateRange(ctx, detail.ID, startDate, endDateExclusive)
	case "luogu":
		detail, detailErr := s.luoguDetailRepo.GetByUserID(ctx, userID)
		if detailErr != nil {
			return detailErr
		}
		if detail == nil {
			return s.ojDailyStatsRepo.DeleteByUserPlatform(ctx, userID, platform)
		}
		currentTotal = detail.PassedNumber
		sourceUpdatedAt = detail.UpdatedAt
		dateSolvedCounts, err = s.luoguUserQuestionRepo.CountSolvedByDateRange(ctx, detail.ID, startDate, endDateExclusive)
	default:
		return errors.New("unsupported oj daily stats platform")
	}
	if err != nil {
		return err
	}
	if sourceUpdatedAt.IsZero() {
		sourceUpdatedAt = time.Now()
	}

	// 构建连续日期的 OJUserDailyStat 列表，填充缺失日期，计算累计总数，并批量 upsert 到数据库
	rows := buildDenseOJDailyStatsRows(userID, platform, currentTotal, sourceUpdatedAt, startDate, windowDays, dateSolvedCounts)
	return s.ojDailyStatsRepo.UpsertBatch(ctx, rows)
}

// RepairRecentWindow 批量修复最近窗口的用户数据，适用于修复历史数据不完整或错误的情况。会根据当前活跃用户列表和平台用户详情列表，逐个用户重建最近窗口的数据。
func (s *OJDailyStatsProjectionService) RepairRecentWindow(ctx context.Context) error {
	if s == nil {
		return errors.New("nil oj daily stats projection service")
	}

	activeUsers, err := s.userRepo.GetActiveUsers(ctx)
	if err != nil {
		return err
	}
	activeSet := make(map[uint]struct{}, len(activeUsers))
	for _, user := range activeUsers {
		if user != nil && user.ID > 0 {
			activeSet[user.ID] = struct{}{}
		}
	}

	leetcodeDetails, err := s.leetcodeDetailRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	if err := s.repairPlatformUsers(ctx, "leetcode", activeSet, leetcodeDetails); err != nil {
		return err
	}

	luoguDetails, err := s.luoguDetailRepo.GetAll(ctx)
	if err != nil {
		return err
	}
	return s.repairPlatformUsers(ctx, "luogu", activeSet, luoguDetails)
}

// repairPlatformUsers 批量修复指定平台的用户数据，activeSet 用于过滤非活跃用户，details 是平台用户详情列表
func (s *OJDailyStatsProjectionService) repairPlatformUsers(
	ctx context.Context,
	platform string,
	activeSet map[uint]struct{},
	details any,
) error {
	batchSize := resolveOJDailyStatsRepairBatchSize()
	var userIDs []uint
	switch typed := details.(type) {
	case []*entity.LeetcodeUserDetail:
		for _, detail := range typed {
			if detail == nil {
				continue
			}
			if _, ok := activeSet[detail.UserID]; ok {
				userIDs = append(userIDs, detail.UserID)
			}
		}
	case []*entity.LuoguUserDetail:
		for _, detail := range typed {
			if detail == nil {
				continue
			}
			if _, ok := activeSet[detail.UserID]; ok {
				userIDs = append(userIDs, detail.UserID)
			}
		}
	default:
		return errors.New("unsupported repair detail type")
	}

	for start := 0; start < len(userIDs); start += batchSize {
		end := start + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		for _, userID := range userIDs[start:end] {
			if err := s.RebuildRecentWindow(ctx, userID, platform, false); err != nil {
				if global.Log != nil {
					global.Log.Error("repair oj daily stats failed",
						zap.Uint("user_id", userID),
						zap.String("platform", platform),
						zap.Error(err))
				}
				return err
			}
		}
	}
	return nil
}

func normalizeOJDailyStatsPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "leetcode":
		return "leetcode"
	case "luogu":
		return "luogu"
	default:
		return ""
	}
}

// resolveOJDailyStatsRepairWindowDays 获取修复窗口天数，默认为 35 天
func resolveOJDailyStatsRepairWindowDays() int {
	if global.Config == nil || global.Config.Task.OJDailyStatsRepairWindowDays <= 0 {
		return defaultOJDailyStatsRepairWindowDays
	}
	return global.Config.Task.OJDailyStatsRepairWindowDays
}

// resolveOJDailyStatsRepairBatchSize 获取修复批次大小，默认为 100
func resolveOJDailyStatsRepairBatchSize() int {
	if global.Config == nil || global.Config.Task.OJDailyStatsRepairBatchSize <= 0 {
		return defaultOJDailyStatsRepairBatchSize
	}
	return global.Config.Task.OJDailyStatsRepairBatchSize
}

// buildOJDailyStatsWindowRange 构建最近 windowDays 天的日期范围，返回开始日期和结束日期（不包含）
func buildOJDailyStatsWindowRange(windowDays int) (time.Time, time.Time) {
	loc := ojDailyStatsLocation()
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := today.AddDate(0, 0, -(windowDays - 1))
	endExclusive := today.AddDate(0, 0, 1)
	return start, endExclusive
}

// buildDenseOJDailyStatsRows 构建连续日期的 OJUserDailyStat 列表，填充缺失日期，计算累计总数
func buildDenseOJDailyStatsRows(
	userID uint,
	platform string,
	currentTotal int,
	sourceUpdatedAt time.Time,
	startDate time.Time,
	windowDays int,
	counts []*readmodel.DateSolvedCount,
) []*entity.OJUserDailyStat {
	countMap := make(map[string]int, len(counts))
	windowSum := 0 // 窗口内的新增题目数量总和
	for _, item := range counts {
		if item == nil {
			continue
		}
		key := item.StatDate.Format("2006-01-02")
		countMap[key] = item.SolvedCount
		windowSum += item.SolvedCount
	}

	baseTotal := currentTotal - windowSum
	if baseTotal < 0 {
		baseTotal = 0
	}

	rows := make([]*entity.OJUserDailyStat, 0, windowDays)
	runningTotal := baseTotal
	for i := 0; i < windowDays; i++ {
		statDate := startDate.AddDate(0, 0, i)
		solvedCount := countMap[statDate.Format("2006-01-02")]
		runningTotal += solvedCount
		rows = append(rows, &entity.OJUserDailyStat{
			UserID:          userID,
			Platform:        platform,
			StatDate:        statDate,
			SolvedCount:     solvedCount,
			SolvedTotal:     runningTotal,
			SourceUpdatedAt: sourceUpdatedAt,
		})
	}
	return rows
}

// ojDailyStatsLocation 统一时区
func ojDailyStatsLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return loc
	}
	return time.FixedZone("CST", 8*60*60)
}
