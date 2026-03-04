package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/service"
	obstrace "personal_assistant/pkg/observability/trace"

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
	spanCtx, spanEvent := startTaskTrace(ctx, name)
	ctx = spanCtx
	if err := fn(ctx); err != nil {
		finishTaskTrace(ctx, spanEvent, err)
		global.Log.Error(name+" failed", zap.Error(err))
	} else {
		finishTaskTrace(ctx, spanEvent, nil)
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

// ObservabilityMetricsRollupTask 指标汇总与清理任务
func ObservabilityMetricsRollupTask() {
	if global.ObservabilityMetrics == nil {
		return
	}
	ctx := context.Background()
	spanCtx, spanEvent := startTaskTrace(ctx, "ObservabilityMetricsRollupTask")
	ctx = spanCtx
	if err := global.ObservabilityMetrics.RollupAndCleanup(ctx, time.Now()); err != nil {
		finishTaskTrace(ctx, spanEvent, err)
		global.Log.Error("ObservabilityMetricsRollupTask failed", zap.Error(err))
		return
	}
	finishTaskTrace(ctx, spanEvent, nil)
	global.Log.Info("ObservabilityMetricsRollupTask completed successfully")
}

// ObservabilityTraceCleanupTask Trace 明细清理任务
func ObservabilityTraceCleanupTask() {
	if global.ObservabilityTraces == nil || global.Config == nil {
		return
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

	ctx := context.Background()
	spanCtx, spanEvent := startTaskTrace(ctx, "ObservabilityTraceCleanupTask")
	ctx = spanCtx
	successBefore := time.Now().Add(-time.Duration(successDays) * 24 * time.Hour)
	if err := global.ObservabilityTraces.CleanupBeforeByStatus(ctx, "ok", successBefore); err != nil {
		finishTaskTrace(ctx, spanEvent, err)
		global.Log.Error("ObservabilityTraceCleanupTask cleanup ok failed", zap.Error(err))
		return
	}

	errorBefore := time.Now().Add(-time.Duration(errorDays) * 24 * time.Hour)
	if err := global.ObservabilityTraces.CleanupBeforeByStatus(ctx, "error", errorBefore); err != nil {
		finishTaskTrace(ctx, spanEvent, err)
		global.Log.Error("ObservabilityTraceCleanupTask cleanup error failed", zap.Error(err))
		return
	}

	finishTaskTrace(ctx, spanEvent, nil)
	global.Log.Info("ObservabilityTraceCleanupTask completed successfully")
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

func startTaskTrace(ctx context.Context, name string) (context.Context, *obstrace.SpanEvent) {
	if global.ObservabilityTraces == nil {
		return ctx, nil
	}
	serviceName := ""
	if global.Config != nil {
		serviceName = strings.TrimSpace(global.Config.Observability.ServiceName)
	}
	return obstrace.StartSpan(ctx, obstrace.StartOptions{
		Service: serviceName,
		Stage:   "task",
		Name:    name,
		Kind:    "cron",
		Tags: map[string]string{
			"task": name,
		},
	})
}

func finishTaskTrace(ctx context.Context, spanEvent *obstrace.SpanEvent, err error) {
	if spanEvent == nil || global.ObservabilityTraces == nil {
		return
	}
	status := obstrace.SpanStatusOK
	code := ""
	message := ""
	if err != nil {
		status = obstrace.SpanStatusError
		code = "task_error"
		message = err.Error()
		spanEvent.WithErrorDetail(buildTaskErrorDetail(spanEvent, err))
	}
	span := spanEvent.End(status, code, message, nil)
	_ = global.ObservabilityTraces.RecordSpan(ctx, span)
}

func buildTaskErrorDetail(spanEvent *obstrace.SpanEvent, err error) string {
	span := spanEvent.Span()
	payload := map[string]string{}
	if span != nil {
		payload["task"] = strings.TrimSpace(span.Name)
		payload["stage"] = strings.TrimSpace(span.Stage)
	}
	if err != nil {
		payload["error"] = strings.TrimSpace(err.Error())
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return ""
	}
	return string(data)
}
