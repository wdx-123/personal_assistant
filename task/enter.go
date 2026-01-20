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
