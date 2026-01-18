package task

import (
	"context"
	"fmt"
	"personal_assistant/global"
	"personal_assistant/internal/repository"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// OutboxRelayTask 轮询Outbox表并推送到Redis Stream
func OutboxRelayTask() {
	ctx := context.Background()
	repo := repository.GroupApp.SystemRepositorySupplier.GetOutboxRepository()

	// 1. 获取待发布事件
	events, err := repo.GetPendingEvents(ctx, 100) // 每次最多取100条
	if err != nil {
		global.Log.Error("OutboxRelay: 获取待发布事件失败", zap.Error(err))
		return
	}

	if len(events) == 0 {
		return
	}

	for _, event := range events {
		// 2. 推送到 Redis Stream
		err := global.Redis.XAdd(ctx, &redis.XAddArgs{
			Stream: event.EventType, // 使用事件类型作为 Stream Key
			Values: map[string]interface{}{
				"event_id":       event.EventID,
				"event_type":     event.EventType,
				"aggregate_id":   event.AggregateID,
				"aggregate_type": event.AggregateType,
				"payload":        event.Payload,
				"created_at":     event.CreatedAt.Format(time.RFC3339),
			},
		}).Err()

		if err != nil {
			global.Log.Error("OutboxRelay: 推送Redis失败",
				zap.String("event_id", event.EventID),
				zap.Error(err))

			// 标记失败
			if markErr := repo.MarkAsFailed(ctx, event.EventID, err.Error()); markErr != nil {
				global.Log.Error("OutboxRelay: 标记失败状态出错", zap.Error(markErr))
			}
			continue
		}

		// 3. 标记成功
		if err := repo.MarkAsPublished(ctx, event.EventID); err != nil {
			global.Log.Error("OutboxRelay: 标记发布状态出错", zap.Error(err))
		}
	}
}

func RegisterScheduledTasks(c *cron.Cron) error {
	// 注册 Outbox Relay 任务，每秒执行一次
	// 注意：robfig/cron/v3 默认只支持分钟级，需要开启秒级支持或使用 @every 1s
	// 假设 core/corn.go 中使用了 WithSeconds() 选项，或者我们使用 @every 1s 语法（v3支持）
	_, err := c.AddFunc("@every 1s", OutboxRelayTask)
	if err != nil {
		return fmt.Errorf("注册OutboxRelay任务失败: %w", err)
	}

	return nil
}
