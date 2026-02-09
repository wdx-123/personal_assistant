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

// runServiceTask 通用定时任务执行器
// 封装 service 层 nil 检查、错误日志、成功日志的重复样板
func runServiceTask(name string, fn func(ctx context.Context) error) {
	if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil {
		global.Log.Error(name + ": service group not initialized")
		return
	}
	ctx := context.Background()
	if err := fn(ctx); err != nil {
		global.Log.Error(name+" failed", zap.Error(err))
	} else {
		global.Log.Info(name + " completed successfully")
	}
}

// OutboxCleanupTask 清理已发布的 Outbox 记录（保留天数由配置驱动）
func OutboxCleanupTask() {
	if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil {
		global.Log.Error("OutboxCleanupTask: service group not initialized")
		return
	}

	days := global.Config.Task.OutboxCleanupRetentionDays
	if days <= 0 {
		days = 7 // 零值兜底
	}
	before := time.Now().Add(time.Duration(-days) * 24 * time.Hour)

	ctx := context.Background()
	repo := repository.GroupApp.SystemRepositorySupplier.GetOutboxRepository()
	if err := repo.DeletePublishedBefore(ctx, before); err != nil {
		global.Log.Error("OutboxCleanupTask failed", zap.Error(err))
	} else {
		global.Log.Info("OutboxCleanupTask completed successfully")
	}
}

// LuoguSyncTask 洛谷用户数据定时全量同步
func LuoguSyncTask() {
	runServiceTask("LuoguSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().SyncAllLuoguUsers(ctx)
	})
}

// LeetcodeSyncTask 力扣用户数据定时全量同步
func LeetcodeSyncTask() {
	runServiceTask("LeetcodeSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().SyncAllLeetcodeUsers(ctx)
	})
}

// RankingSyncTask 排行榜缓存定时重建
func RankingSyncTask() {
	runServiceTask("RankingSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().RebuildRankingCaches(ctx)
	})
}

// ImageOrphanCleanupTask 孤儿图片定时清理
// 查找已软删除且无活跃引用的存储 key，删除对应物理文件后清除 DB 记录
func ImageOrphanCleanupTask() {
	runServiceTask("ImageOrphanCleanupTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetImageSvc().CleanOrphanFiles(ctx)
	})
}

// RegisterScheduledTasks 注册所有定时任务到 cron 调度器
func RegisterScheduledTasks(c *cron.Cron) error {
	// Outbox 清理 — 每天一次
	if _, err := c.AddFunc("@daily", OutboxCleanupTask); err != nil {
		return fmt.Errorf("注册 OutboxCleanupTask 失败: %w", err)
	}

	// 洛谷同步 — 每小时一次
	if _, err := c.AddFunc("@hourly", LuoguSyncTask); err != nil {
		return fmt.Errorf("注册 LuoguSyncTask 失败: %w", err)
	}

	// 力扣同步 — 间隔由配置驱动
	leetcodeInterval := global.Config.Task.LeetcodeSyncIntervalSeconds
	if leetcodeInterval <= 0 {
		leetcodeInterval = 3600
	}
	if _, err := c.AddFunc(fmt.Sprintf("@every %ds", leetcodeInterval), LeetcodeSyncTask); err != nil {
		return fmt.Errorf("注册 LeetcodeSyncTask 失败: %w", err)
	}

	// 排行榜重建 — 间隔由配置驱动
	rankingInterval := global.Config.Task.RankingSyncIntervalSeconds
	if rankingInterval <= 0 {
		rankingInterval = 300
	}
	if _, err := c.AddFunc(fmt.Sprintf("@every %ds", rankingInterval), RankingSyncTask); err != nil {
		return fmt.Errorf("注册 RankingSyncTask 失败: %w", err)
	}

	// 孤儿图片清理 — cron 表达式由配置驱动
	orphanCron := global.Config.Task.ImageOrphanCleanupCron
	if orphanCron == "" {
		orphanCron = "@daily"
	}
	if _, err := c.AddFunc(orphanCron, ImageOrphanCleanupTask); err != nil {
		return fmt.Errorf("注册 ImageOrphanCleanupTask 失败: %w", err)
	}

	return nil
}
