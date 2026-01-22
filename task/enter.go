package task

import (
	"context"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/service"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

func OutboxCleanupTask() {
	ctx := context.Background()
	repo := repository.GroupApp.SystemRepositorySupplier.GetOutboxRepository()

	days := global.Config.Task.OutboxCleanupRetentionDays
	if days <= 0 {
		days = 7 // 默认 7 天
	}
	before := time.Now().Add(time.Duration(-days) * 24 * time.Hour)

	if err := repo.DeletePublishedBefore(ctx, before); err != nil {
		global.Log.Error("OutboxCleanup: 清理失败", zap.Error(err))
	}
}

func LuoguSyncTask() {
	ctx := context.Background()
	// 注意: 确保 service.GroupApp 已经初始化
	if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil {
		global.Log.Error("LuoguSyncTask: service group not initialized")
		return
	}
	svc := service.GroupApp.SystemServiceSupplier.GetOJSvc()
	if err := svc.SyncAllLuoguUsers(ctx); err != nil {
		global.Log.Error("LuoguSyncTask failed", zap.Error(err))
	} else {
		global.Log.Info("LuoguSyncTask completed successfully")
	}
}

func LeetcodeSyncTask() { // 力扣用户定时同步任务
	ctx := context.Background()                                                   // 创建后台上下文
	if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil { // 校验服务是否初始化
		global.Log.Error("LeetcodeSyncTask: service group not initialized") // 记录初始化失败
		return                                                              // 直接返回
	}
	svc := service.GroupApp.SystemServiceSupplier.GetOJSvc() // 获取 OJ 服务
	if err := svc.SyncAllLeetcodeUsers(ctx); err != nil {    // 执行全量同步
		global.Log.Error("LeetcodeSyncTask failed", zap.Error(err)) // 记录错误日志
	} else {
		global.Log.Info("LeetcodeSyncTask completed successfully") // 记录成功日志
	}
}

func RankingSyncTask() {
	ctx := context.Background()
	if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil {
		global.Log.Error("RankingSyncTask: service group not initialized")
		return
	}
	svc := service.GroupApp.SystemServiceSupplier.GetOJSvc()
	if err := svc.RebuildRankingCaches(ctx); err != nil {
		global.Log.Error("RankingSyncTask failed", zap.Error(err))
	} else {
		global.Log.Info("RankingSyncTask completed successfully")
	}
}

func RegisterScheduledTasks(c *cron.Cron) error {
	_, err := c.AddFunc("@daily", OutboxCleanupTask)
	if err != nil {
		return fmt.Errorf("注册OutboxCleanup任务失败: %w", err)
	}

	// 洛谷同步任务 - 每小时执行一次
	_, err = c.AddFunc("@hourly", LuoguSyncTask)
	if err != nil {
		return fmt.Errorf("注册LuoguSyncTask任务失败: %w", err)
	}

	leetcodeInterval := global.Config.Task.LeetcodeSyncIntervalSeconds // 读取力扣同步间隔
	if leetcodeInterval <= 0 {                                         // 兜底默认值
		leetcodeInterval = 3600 // 默认 1 小时
	}
	leetcodeSpec := fmt.Sprintf("@every %ds", leetcodeInterval) // 转换为 cron 表达式
	_, err = c.AddFunc(leetcodeSpec, LeetcodeSyncTask)          // 注册定时任务
	if err != nil {                                             // 处理注册失败
		return fmt.Errorf("注册LeetcodeSyncTask任务失败: %w", err) // 返回错误
	}

	interval := global.Config.Task.RankingSyncIntervalSeconds
	if interval <= 0 {
		interval = 300
	}
	spec := fmt.Sprintf("@every %ds", interval)
	_, err = c.AddFunc(spec, RankingSyncTask)
	if err != nil {
		return fmt.Errorf("注册RankingSyncTask任务失败: %w", err)
	}

	return nil
}
