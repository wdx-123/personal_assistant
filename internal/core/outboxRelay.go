package core

import (
	"context"
	"errors"
	"sync"

	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/infrastructure/outbox"
	"personal_assistant/internal/repository/interfaces"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var outboxRelayOnce sync.Once

func StartOutboxRelay(
	ctx context.Context,
	repo interfaces.OutboxRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
) {
	if ctx == nil {
		ctx = context.Background()
	}
	if repo == nil || redisClient == nil || logger == nil {
		return
	}

	outboxRelayOnce.Do(func() {
		publisher := messaging.NewRedisStreamPublisher(redisClient, logger)
		processor := outbox.NewRelayProcessor(repo, publisher, logger)
		go func() {
			if err := processor.Run(ctx, redisClient); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("OutboxRelay: 运行失败", zap.Error(err))
			}
		}()
	})
}
