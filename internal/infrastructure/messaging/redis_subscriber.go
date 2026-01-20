package messaging

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"personal_assistant/global"
)

// RedisStreamSubscriber 基于 Redis Stream 的订阅者实现
type RedisStreamSubscriber struct {
	client *redis.Client
	logger *zap.Logger
	group  string // 消费者组
	name   string // 消费者名称
}

func NewRedisStreamSubscriber(client *redis.Client, logger *zap.Logger, group, name string) *RedisStreamSubscriber {
	return &RedisStreamSubscriber{
		client: client,
		logger: logger,
		group:  group,
		name:   name,
	}
}

// Subscribe 订阅并处理消息
// 注意：这是一个阻塞调用，通常需要在 goroutine 中运行
func (s *RedisStreamSubscriber) Subscribe(
	ctx context.Context,
	topic string,
	handler MessageHandler,
) error {
	// 1. 尝试创建消费者组（如果不存在）
	// MKSTREAM 选项会自动创建 Stream Key
	err := s.client.XGroupCreateMkStream(ctx, topic, s.group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		s.logger.Error("创建消费者组失败", zap.String("topic", topic), zap.Error(err))
		// 不直接返回，因为可能只是组已存在
	}

	// 2. 循环读取消息
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// XReadGroup 读取消息
			count := int64(global.Config.Messaging.RedisStreamReadCount)
			if count <= 0 {
				count = 1
			}
			blockMs := time.Duration(global.Config.Messaging.RedisStreamBlockMs)
			if blockMs <= 0 {
				blockMs = 5000 // 默认 5 秒
			}

			streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    s.group,
				Consumer: s.name,
				Streams:  []string{topic, ">"}, // ">" 表示读取未被其他消费者读取的新消息
				Count:    count,
				Block:    blockMs * time.Millisecond,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					// 超时未读到消息，继续下一次循环
					continue
				}
				s.logger.Error("读取消息失败", zap.Error(err))
				time.Sleep(time.Second) // 发生错误时稍作休眠
				continue
			}

			// 处理读取到的消息
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					// 解析消息
					parsedMsg := s.parseMessage(msg)
					parsedMsg.Topic = topic

					// 调用处理函数
					if err := handler(ctx, parsedMsg); err != nil {
						s.logger.Error("处理消息失败",
							zap.String("msg_id", msg.ID),
							zap.Error(err))
						// 这里可以实现死信队列或重试逻辑
						continue
					}

					// 确认消息 (ACK)
					if err := s.client.XAck(ctx, topic, s.group, msg.ID).Err(); err != nil {
						s.logger.Error("ACK 失败", zap.String("msg_id", msg.ID), zap.Error(err))
					}
				}
			}
		}
	}
}

func (s *RedisStreamSubscriber) parseMessage(xMsg redis.XMessage) *Message {
	values := xMsg.Values
	msg := &Message{
		ID:       xMsg.ID, // 默认使用 Redis ID，下面尝试覆盖
		Metadata: make(map[string]string),
	}

	// 优先尝试获取业务 ID
	if v, ok := values["id"].(string); ok && v != "" {
		msg.ID = v
	}

	// 获取 Partition Key
	if v, ok := values["key"].(string); ok {
		msg.Key = v
	}

	if v, ok := values["payload"].(string); ok {
		msg.Payload = []byte(v)
	}

	// 提取元数据
	for key, v := range values {
		if val, ok := v.(string); ok {
			if len(key) > 5 && key[:5] == "meta_" {
				msg.Metadata[key[5:]] = val
			}
		}
	}

	return msg
}

func (s *RedisStreamSubscriber) Close() error {
	return nil
}
