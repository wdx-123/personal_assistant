package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/service"
	"personal_assistant/pkg/observability/tasktrace"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// runServiceTask 通用定时任务执行器，统一挂载 tracing、成功日志与失败日志。
func runServiceTask(name string, fn func(ctx context.Context) error) {
	wrapTask(name, func(ctx context.Context) error {
		if service.GroupApp == nil || service.GroupApp.SystemServiceSupplier == nil {
			return fmt.Errorf("%s: service group not initialized", name)
		}
		if fn == nil {
			return nil
		}
		return fn(ctx)
	})()
}

// OutboxCleanupTask 清理已发布的 Outbox 记录（保留天数由配置驱动）。
func OutboxCleanupTask() {
	wrapTask("OutboxCleanupTask", func(ctx context.Context) error {
		if repository.GroupApp == nil || repository.GroupApp.SystemRepositorySupplier == nil {
			return fmt.Errorf("OutboxCleanupTask: repository group not initialized")
		}

		days := global.Config.Task.OutboxCleanupRetentionDays
		if days <= 0 {
			days = 7 // 零值兜底
		}
		failedDays := global.Config.Task.OutboxFailedCleanupRetentionDays
		if failedDays <= 0 {
			failedDays = 30
		}
		now := time.Now()
		before := now.Add(time.Duration(-days) * 24 * time.Hour)
		failedBefore := now.Add(time.Duration(-failedDays) * 24 * time.Hour)
		repo := repository.GroupApp.SystemRepositorySupplier.GetOutboxRepository()
		if err := repo.DeletePublishedBefore(ctx, before); err != nil {
			return err
		}
		return repo.DeleteFailedBefore(ctx, failedBefore)
	})()
}

// ObservabilityMetricsRollupTask 指标汇总与清理任务。
func ObservabilityMetricsRollupTask() {
	wrapTask("ObservabilityMetricsRollupTask", func(ctx context.Context) error {
		if global.ObservabilityMetrics == nil {
			return nil
		}
		return global.ObservabilityMetrics.RollupAndCleanup(ctx, time.Now())
	})()
}

// ObservabilityTraceCleanupTask Trace 明细清理任务。
func ObservabilityTraceCleanupTask() {
	wrapTask("ObservabilityTraceCleanupTask", func(ctx context.Context) error {
		if global.ObservabilityTraces == nil || global.Config == nil {
			return nil
		}
		cfg := global.Config.Observability.Traces
		successDays := cfg.SuccessRetentionDays
		errorDays := cfg.ErrorRetentionDays
		if successDays <= 0 {
			successDays = 5
		}
		if errorDays <= 0 {
			errorDays = 10
		}

		successBefore := time.Now().Add(-time.Duration(successDays) * 24 * time.Hour)
		if err := global.ObservabilityTraces.CleanupBeforeByStatus(ctx, "ok", successBefore); err != nil {
			return err
		}
		errorBefore := time.Now().Add(-time.Duration(errorDays) * 24 * time.Hour)
		return global.ObservabilityTraces.CleanupBeforeByStatus(ctx, "error", errorBefore)
	})()
}

// LuoguSyncTask 洛谷用户数据定时全量同步。
func LuoguSyncTask() {
	runServiceTask("LuoguSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().SyncAllLuoguUsers(ctx)
	})
}

// LeetcodeSyncTask 力扣用户数据定时全量同步。
func LeetcodeSyncTask() {
	runServiceTask("LeetcodeSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().SyncAllLeetcodeUsers(ctx)
	})
}

// LanqiaoSyncTask 蓝桥用户数据定时增量同步。
func LanqiaoSyncTask() {
	runServiceTask("LanqiaoSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().SyncAllLanqiaoUsers(ctx)
	})
}

// LanqiaoStatsRefreshTask 蓝桥提交成功/失败次数低频刷新。
func LanqiaoStatsRefreshTask() {
	runServiceTask("LanqiaoStatsRefreshTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().RefreshAllLanqiaoSubmissionStats(ctx)
	})
}

// RankingSyncTask 排行榜缓存定时重建。
func RankingSyncTask() {
	runServiceTask("RankingSyncTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetOJSvc().RebuildRankingCaches(ctx)
	})
}

// OJDailyStatsRepairTask 刷题曲线日聚合修复任务。
func OJDailyStatsRepairTask() {
	runServiceTask("OJDailyStatsRepairTask", func(ctx context.Context) error {
		svc := service.GroupApp.SystemServiceSupplier.GetOJDailyStatsProjectionSvc()
		if svc == nil {
			return nil
		}
		return svc.RepairRecentWindow(ctx)
	})
}

// ImageOrphanCleanupTask 孤儿图片定时清理。
// 查找已软删除且无活跃引用的存储 key，删除对应物理文件后清除 DB 记录。
func ImageOrphanCleanupTask() {
	runServiceTask("ImageOrphanCleanupTask", func(ctx context.Context) error {
		return service.GroupApp.SystemServiceSupplier.GetImageSvc().CleanOrphanFiles(ctx)
	})
}

// DisabledUserCleanupTask 禁用账号清理任务（软删+匿名化）。
func DisabledUserCleanupTask() {
	runServiceTask("DisabledUserCleanupTask", func(ctx context.Context) error {
		_, err := service.GroupApp.SystemServiceSupplier.GetUserSvc().CleanupDisabledUsers(ctx)
		return err
	})
}

// RegisterScheduledTasks 注册所有定时任务到 cron 调度器。
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

	lanqiaoInterval := global.Config.Task.LanqiaoSyncIntervalSeconds
	if lanqiaoInterval <= 0 {
		lanqiaoInterval = 3600
	}
	if _, err := c.AddFunc(fmt.Sprintf("@every %ds", lanqiaoInterval), LanqiaoSyncTask); err != nil {
		return fmt.Errorf("注册 LanqiaoSyncTask 失败: %w", err)
	}

	lanqiaoStatsCron := strings.TrimSpace(global.Config.Task.LanqiaoStatsRefreshCron)
	if lanqiaoStatsCron == "" {
		lanqiaoStatsCron = "@daily"
	}
	if _, err := c.AddFunc(lanqiaoStatsCron, LanqiaoStatsRefreshTask); err != nil {
		return fmt.Errorf("注册 LanqiaoStatsRefreshTask 失败: %w", err)
	}

	// 排行榜重建 — 间隔由配置驱动
	rankingInterval := global.Config.Task.RankingSyncIntervalSeconds
	if rankingInterval <= 0 {
		rankingInterval = 300
	}
	if _, err := c.AddFunc(fmt.Sprintf("@every %ds", rankingInterval), RankingSyncTask); err != nil {
		return fmt.Errorf("注册 RankingSyncTask 失败: %w", err)
	}

	curveRepairCron := strings.TrimSpace(global.Config.Task.OJDailyStatsRepairCron)
	if curveRepairCron == "" {
		curveRepairCron = "@daily"
	}
	if _, err := c.AddFunc(curveRepairCron, OJDailyStatsRepairTask); err != nil {
		return fmt.Errorf("注册 OJDailyStatsRepairTask 失败: %w", err)
	}

	// 孤儿图片清理 — cron 表达式由配置驱动
	orphanCron := global.Config.Task.ImageOrphanCleanupCron
	if orphanCron == "" {
		orphanCron = "@daily"
	}
	if _, err := c.AddFunc(orphanCron, ImageOrphanCleanupTask); err != nil {
		return fmt.Errorf("注册 ImageOrphanCleanupTask 失败: %w", err)
	}

	if global.Config.Task.DisabledUserCleanupEnabled {
		disabledCron := strings.TrimSpace(global.Config.Task.DisabledUserCleanupCron)
		if disabledCron == "" {
			disabledCron = "@daily"
		}
		if _, err := c.AddFunc(disabledCron, DisabledUserCleanupTask); err != nil {
			return fmt.Errorf("注册 DisabledUserCleanupTask 失败: %w", err)
		}
	}

	rollupCron := global.Config.Observability.Metrics.RollupCron
	if rollupCron == "" {
		rollupCron = "10 2 * * *"
	}
	if _, err := c.AddFunc(rollupCron, ObservabilityMetricsRollupTask); err != nil {
		return fmt.Errorf("注册 ObservabilityMetricsRollupTask 失败: %w", err)
	}

	traceCleanupCron := global.Config.Observability.Traces.CleanupCron
	if traceCleanupCron == "" {
		traceCleanupCron = "30 2 * * *"
	}
	if _, err := c.AddFunc(traceCleanupCron, ObservabilityTraceCleanupTask); err != nil {
		return fmt.Errorf("注册 ObservabilityTraceCleanupTask 失败: %w", err)
	}

	return nil
}

func wrapTask(name string, fn func(context.Context) error) func() {
	return tasktrace.Wrap(name, tasktrace.Options{
		Backend:     global.ObservabilityTraces,
		ServiceName: resolveTaskServiceName(),
		Kind:        "cron",
		Trigger:     "cron",
		LockEnabled: resolveTaskLockEnabled(),
		LockKey:     buildTaskLockKey(name),
		LockTTL:     resolveTaskLockTTL(),
	}, func(ctx context.Context) error {
		if fn == nil {
			return nil
		}
		err := fn(ctx)
		if err != nil {
			if global.Log != nil {
				global.Log.Error(name+" failed", zap.Error(err))
			}
			return err
		}
		if global.Log != nil {
			global.Log.Info(name + " completed successfully")
		}
		return nil
	})
}

func resolveTaskServiceName() string {
	if global.Config == nil {
		return "personal_assistant"
	}
	serviceName := strings.TrimSpace(global.Config.Observability.ServiceName)
	if serviceName == "" {
		return "personal_assistant"
	}
	return serviceName
}

func resolveTaskLockEnabled() bool {
	if global.Config == nil {
		return true
	}
	return global.Config.Task.DistributedLockEnabled
}

func resolveTaskLockTTL() time.Duration {
	if global.Config == nil || global.Config.Task.DistributedLockTTLSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(global.Config.Task.DistributedLockTTLSeconds) * time.Second
}

func buildTaskLockKey(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	return "task:cron:" + name
}
