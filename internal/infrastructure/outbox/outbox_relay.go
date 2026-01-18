package outbox

import (
	"context"
	"fmt"
	"manpao-service/internal/models"
	"manpao-service/pkg/globals"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
)

// RelayConfig Relay 配置
type RelayConfig struct {
	PollInterval time.Duration // 轮询间隔（默认 100ms）
	BatchSize    int           // 每次处理事件数（默认 100）
	MaxRetry     int           // 最大重试次数（默认 3）
}

// Relay Outbox 事件中继接口
type Relay interface {
	Start(ctx context.Context) error // 启动轮询循环
	Stop() error                     // 停止中继器
}

// relayImpl Relay 接口实现
type relayImpl struct {
	repo      Repository        // Outbox Repository
	publisher message.Publisher // Watermill Publisher
	config    RelayConfig       // 配置
	stopCh    chan struct{}     // 停止信号通道
}

// NewRelay 创建 Outbox Relay 实例
func NewRelay(repo Repository, publisher message.Publisher, config RelayConfig) Relay {
	// 默认值
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.MaxRetry == 0 {
		config.MaxRetry = 3
	}
	return &relayImpl{
		repo:      repo,
		publisher: publisher,
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动 Relay (阻塞运行)
func (r *relayImpl) Start(ctx context.Context) error {
	globals.Log.Info("Outbox Relay 启动...")

	// 创建 Ticker（定时器）
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	// 主循环
	for {
		select {
		case <-ctx.Done():
			// 收到取消信号,优雅推出
			globals.Log.Info("Outbox Relay 停止(context 取消)...")
			return ctx.Err()

		case <-r.stopCh:
			// 收到停止信号
			globals.Log.Info("Outbox Relay 停止 (手动停止)...")
			return nil

		case <-ticker.C:
			// 每隔 PollInterval 执行一次
			if err := r.processEvents(ctx); err != nil {
				globals.Log.Errorf("处理 事件 失败: %v", err)
				// 不中断循环，继续重试
			}
		}
	}
}

// Stop 停止 Relay
func (r *relayImpl) Stop() error {
	close(r.stopCh)
	return nil
}

// processEvents 处理待发布的事件
// nolint
func (r *relayImpl) processEvents(ctx context.Context) error {
	// 1. 查询待发布的事件
	events, err := r.repo.GetPendingEvents(ctx, r.config.BatchSize)
	if err != nil {
		return fmt.Errorf("获取待处理 事件: %w", err)
	}

	if len(events) == 0 {
		return nil
	}
	globals.Log.Infof("发现 %d 待处理事件, 处理中...", len(events))

	// 2. 逐个发布事件
	successCount := 0
	failedCount := 0

	for _, event := range events {
		if err := r.publishEvent(ctx, event); err != nil {
			globals.Log.Errorf("发布事件失败 %s: %v", event.EventID, err)

			// 标记为失败
			if markErr := r.repo.MarkAsFailed(ctx, event.EventID, err.Error()); markErr != nil {
				globals.Log.Errorf("标记事件失败状态出错 %s: %v", event.EventID, markErr)
			}
			failedCount++

			// 如果超过最大重试次数，记录错误日志
			if event.RetryCount >= r.config.MaxRetry {
				globals.Log.Errorf("⚠事件 %s 超出最大尝试次数 (%d), giving up",
					event.EventID, r.config.MaxRetry)
			}
		} else {
			// 标记为已发布
			if markErr := r.repo.MarkAsPublished(ctx, event.EventID); markErr != nil {
				globals.Log.Errorf("标记事件已发布状态出错 %s: %v", event.EventID, markErr)
			}
			successCount++
		}
	}
	globals.Log.Infof("处理 %d events: %d 成功, %d 失败",
		len(events), successCount, failedCount)

	return nil
}

// publishEvent 发布单个事件到 Watermill
// ctx 参数保留用于未来可能的取消操作支持
func (r *relayImpl) publishEvent(_ context.Context, event *models.OutboxEvent) error {
	// 1. 构建 Watermill Message
	msg := message.NewMessage(event.EventID, []byte(event.Payload))

	// 2. 设置 Metadata
	msg.Metadata.Set("event_id", event.EventID)
	msg.Metadata.Set("event_type", event.EventType)
	msg.Metadata.Set("aggregate_id", event.AggregateID)
	msg.Metadata.Set("aggregate_type", event.AggregateType)
	msg.Metadata.Set("created_at", event.CreatedAt.Format(time.RFC3339))

	// 发布到 Redis Streams
	// Topic 使用 event_type （如 "wechat.payment.succeeded"）
	if err := r.publisher.Publish(event.EventType, msg); err != nil {
		return fmt.Errorf("发布到 watermill: %w", err)
	}
	globals.Log.Debugf("发布 事件 : %s (type=%s)",
		event.EventID, event.EventType)

	return nil
}
