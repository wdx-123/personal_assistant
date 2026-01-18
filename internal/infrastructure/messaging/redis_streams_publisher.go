package messaging

import (
	"context"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-redis/redis/v8"
)

// RedisStreamsPublisher 自定义的 Redis Streams Publisher
// 实现 watermill 的 message.Publisher 接口
type RedisStreamsPublisher struct {
	client *redis.Client
	logger watermill.LoggerAdapter
}

// NewRedisStreamsPublisher 创建 Redis Streams Publisher
func NewRedisStreamsPublisher(
	client *redis.Client,
	logger watermill.LoggerAdapter,
) message.Publisher {
	if logger == nil {
		logger = watermill.NopLogger{}
	}

	return &RedisStreamsPublisher{
		client: client,
		logger: logger,
	}
}

// Publish 发布消息到 Redis Streams
// topic: Stream 名称（如 "order.payment.succeeded"）
// messages: 要发布的消息列表
func (p *RedisStreamsPublisher) Publish(topic string, messages ...*message.Message) error {
	ctx := context.Background()

	for _, msg := range messages {
		// 1. 构建 Redis Streams 的 values（字段-值对）
		values := map[string]any{
			"uuid":    msg.UUID,            // 消息 UUID
			"payload": string(msg.Payload), // 消息体（JSON）
		}

		// 2. 添加 Metadata（元数据）
		for key, value := range msg.Metadata {
			values["metadata_"+key] = value
		}

		// 3. 使用 XADD 命令发布到 Redis Streams
		// "*" 表示自动生成消息 ID（时间戳-序 号）
		id, err := p.client.XAdd(ctx, &redis.XAddArgs{
			Stream: topic,
			Values: values,
		}).Result()

		if err != nil {
			p.logger.Error("Failed to publish message", err,
				watermill.LogFields{
					"topic":        topic,
					"message_uuid": msg.UUID,
				})
			return fmt.Errorf("redis xadd failed: %w", err)
		}

		p.logger.Debug("Message published", watermill.LogFields{
			"topic":        topic,
			"message_uuid": msg.UUID,
			"stream_id":    id,
		})
	}
	return nil
}

// Close 关闭 Publisher（实现接口要求）
func (p *RedisStreamsPublisher) Close() error {
	// Redis Client 由外部管理，这里不关闭
	p.logger.Info("Redis Streams Publisher closed", nil)
	return nil
}
