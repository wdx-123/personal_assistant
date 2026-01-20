package outbox

import (
	"context"
	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/repository/interfaces"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RelayProcessor 负责将 Outbox 表中的事件转发到消息队列
type RelayProcessor struct {
	repo       interfaces.OutboxRepository
	publisher  messaging.Publisher
	logger     *zap.Logger
	batchSize  int
	maxRetries int

	pollInterval  time.Duration
	notifyChannel string
}

func NewRelayProcessor(
	repo interfaces.OutboxRepository,
	publisher messaging.Publisher,
	logger *zap.Logger,
) *RelayProcessor {
	return &RelayProcessor{
		repo:          repo,
		publisher:     publisher,
		logger:        logger,
		batchSize:     100, // 默认批次大小
		maxRetries:    3,
		pollInterval:  time.Second,
		notifyChannel: "outbox_new_event",
	}
}

// Process 执行一次轮询和转发
func (p *RelayProcessor) Process(ctx context.Context) error {
	// 1. 获取待发布事件
	events, err := p.repo.GetPendingEvents(ctx, p.batchSize, p.maxRetries)
	if err != nil {
		p.logger.Error("OutboxRelay: 获取待发布事件失败", zap.Error(err))
		return err
	}

	if len(events) == 0 {
		return nil
	}

	for _, event := range events {
		// 2. 构建消息
		msg := &messaging.Message{
			ID:          event.EventID,
			Topic:       event.EventType,
			Payload:     []byte(event.Payload),
			OccurredAt:  event.CreatedAt,
			PublishedAt: time.Now(),
			Metadata: map[string]string{
				"aggregate_id":   event.AggregateID,
				"aggregate_type": event.AggregateType,
			},
		}

		// 3. 发布消息
		if err := p.publisher.Publish(ctx, msg); err != nil {
			p.logger.Error("OutboxRelay: 推送消息失败",
				zap.String("event_id", event.EventID),
				zap.Error(err))

			// 标记失败
			if markErr := p.repo.MarkAsFailed(ctx, event.EventID, err.Error(), p.maxRetries); markErr != nil {
				p.logger.Error("OutboxRelay: 标记失败状态出错", zap.Error(markErr))
			}
			continue
		}

		// 4. 标记成功
		if err := p.repo.MarkAsPublished(ctx, event.EventID); err != nil {
			p.logger.Error("OutboxRelay: 标记发布状态出错", zap.Error(err))
		}
	}

	return nil
}

func (p *RelayProcessor) Run(ctx context.Context, redisClient *redis.Client) error {
	pollTicker := time.NewTicker(p.pollInterval)
	defer pollTicker.Stop()

	var sub *redis.PubSub
	var subCh <-chan *redis.Message
	if redisClient != nil {
		sub = redisClient.Subscribe(ctx, p.notifyChannel)
		if _, err := sub.Receive(ctx); err != nil {
			_ = sub.Close()
			sub = nil
		} else {
			subCh = sub.Channel()
		}
	}
	if sub != nil {
		defer sub.Close()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			_ = p.Process(ctx)
		case <-subCh:
			_ = p.Process(ctx)
		}
	}
}

func NotifyNewOutboxEvent(
	ctx context.Context,
	redisClient *redis.Client,
) error {
	if redisClient == nil {
		return nil
	}
	return redisClient.Publish(ctx, "outbox_new_event", "1").Err()
}
