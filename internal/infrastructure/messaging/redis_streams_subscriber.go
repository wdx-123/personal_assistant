package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/go-redis/redis/v8"
)

// Context key 类型定义（避免 lint 警告）
type contextKey string

const (
	contextKeyRedisStream contextKey = "redis_stream"
	contextKeyRedisMsgID  contextKey = "redis_msg_id"
)

// RedisStreamsSubscriber  自定义的 Redis Streams Subscriber
type RedisStreamsSubscriber struct {
	client        *redis.Client
	consumerGroup string // 消费者组名称
	consumerName  string // 消费者名称
	logger        watermill.LoggerAdapter
}

// SubscriberConfig Subscriber 配置
type SubscriberConfig struct {
	Client        *redis.Client
	ConsumerGroup string // 消费者组名称，如 "points-service-group"
	ConsumerName  string // 消费者名称, 如 "consumer-1"
	Logger        watermill.LoggerAdapter
}

// NewRedisStreamsSubscriber 创建 Redis Streams Subscriber
func NewRedisStreamsSubscriber(config SubscriberConfig) (message.Subscriber, error) {
	if config.ConsumerGroup == "" {
		config.ConsumerGroup = "manpao-events-group"
	}
	if config.ConsumerName == "" {
		config.ConsumerName = fmt.Sprintf("consumer-%d",
			time.Now().Unix())
	}
	if config.Logger == nil {
		config.Logger = watermill.NopLogger{}
	}

	return &RedisStreamsSubscriber{
		client:        config.Client,
		consumerGroup: config.ConsumerGroup,
		consumerName:  config.ConsumerName,
		logger:        config.Logger,
	}, nil
}

// Subscribe 订阅指定的 topic 的消息
// topic : Stream 名称
func (s *RedisStreamsSubscriber) Subscribe(
	ctx context.Context,
	topic string,
) (<-chan *message.Message, error) {
	// 1. 确保消费组的存在
	if err := s.ensureConsumerGroup(ctx, topic); err != nil {
		return nil, fmt.Errorf("ensure consumer group:%w", err)
	}

	// 2.创建消息通道
	msgChan := make(chan *message.Message)
	// 启动后台 goroutine 读取消息
	go s.readMessages(ctx, topic, msgChan)

	s.logger.Info("Subscribed to topic",
		watermill.LogFields{
			"topic":          topic,
			"consumer_group": s.consumerGroup,
			"consumer_name":  s.consumerName,
		})

	return msgChan, nil
}

// ensureConsumerGroup 确保消费组的存在
func (s *RedisStreamsSubscriber) ensureConsumerGroup(ctx context.Context, stream string) error {
	// 尝试创建消费者组
	err := s.client.XGroupCreateMkStream(ctx, stream, s.consumerGroup, "0").Err()

	if err != nil {
		// 如果消费者组已存在, 忽略错误
		if strings.Contains(err.Error(), "BUSYGROUP") {
			return nil
		}
		return err
	}
	s.logger.Info("Consumer group created",
		watermill.LogFields{
			"stream":         stream,
			"consumer_group": s.consumerGroup,
		})

	return nil
}

// readMessages 读取消息的后台循环
//
//nolint:revive // 函数需要处理多种情况（context取消、Redis读取、错误处理、消息分发），复杂度合理
func (s *RedisStreamsSubscriber) readMessages(ctx context.Context, topic string,
	msgChan chan *message.Message) {
	defer close(msgChan)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Subscriber stopped", watermill.LogFields{"topic": topic})
			return
		default:
			// 使用 XREADGROUP 读取消息
			// ">" 表示读取未投递的新消息
			streams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    s.consumerGroup,
				Consumer: s.consumerName,
				Streams:  []string{topic, ">"},    // Stream 名称 + 起始位置
				Count:    10,                      // 每次最多读取 10 条
				Block:    1000 * time.Millisecond, // 阻塞等待 1 秒
			}).Result()

			if err != nil {
				// 超时或其它错误, 继续循环
				if err == redis.Nil {
					continue // 无新消息
				}

				s.logger.Error(
					"XReadGroup error",
					err,
					watermill.LogFields{"topic": topic})
				time.Sleep(time.Second)
				continue
			}

			// 处理收到的消息
			for _, stream := range streams {
				for _, redisMsg := range stream.Messages {
					msg := s.convertToWatermillMessage(topic, redisMsg)

					// 设置 ACK 回调
					msg.SetContext(context.WithValue(
						msg.Context(),
						contextKeyRedisMsgID,
						redisMsg.ID,
					))
					msg.SetContext(context.WithValue(
						msg.Context(),
						contextKeyRedisStream,
						topic,
					))

					// 发送到通道
					select {
					case msgChan <- msg:
						s.logger.Debug(
							"Message received",
							watermill.LogFields{
								"topic":    topic,
								"msg_id":   redisMsg.ID,
								"msg_uuid": msg.UUID,
							},
						)
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}
}

// convertToWatermillMessage 将 Redis 消息转换为 Watermill Message
func (s *RedisStreamsSubscriber) convertToWatermillMessage(
	topic string,
	redisMsg redis.XMessage,
) *message.Message {
	// 1. 提取 UUID 和 Payload（带类型检查）
	uuid, ok := redisMsg.Values["uuid"].(string)
	if !ok {
		uuid = "" // 如果类型断言失败，使用空字符串
	}

	payload, ok := redisMsg.Values["payload"].(string)
	if !ok {
		payload = "" // 如果类型断言失败，使用空字符串
	}

	// 2. 创建 Watermill Message
	msg := message.NewMessage(uuid, []byte(payload))

	// 3. 恢复 Metadata
	for key, value := range redisMsg.Values {
		if key == "uuid" || key == "payload" {
			continue
		}
		// metadata_event_type → event_type
		if len(key) > 9 && key[:9] == "metadata_" {
			metaKey := key[9:]
			if strValue, ok := value.(string); ok {
				msg.Metadata.Set(metaKey, strValue)
			}
		}
	}

	// 4. 存储 Redis 消息信息到 Context（供外部 ACK 使用）
	// 注意：Watermill 的 msg.Ack 字段不可直接赋值，需要在外部处理
	ctx := context.WithValue(context.Background(), contextKeyRedisStream, topic)
	ctx = context.WithValue(ctx, contextKeyRedisMsgID, redisMsg.ID)
	msg.SetContext(ctx)

	return msg
}

// AckMessage 手动确认消息（从 Context 中提取 Redis 信息）
func (s *RedisStreamsSubscriber) AckMessage(msg *message.Message) error {
	stream, ok := msg.Context().Value(contextKeyRedisStream).(string)
	if !ok {
		return errors.New("redis_stream not found in context")
	}

	msgID, ok := msg.Context().Value(contextKeyRedisMsgID).(string)
	if !ok {
		return errors.New("redis_msg_id not found in context")
	}

	ctx := context.Background()
	return s.client.XAck(ctx, stream, s.consumerGroup, msgID).Err()
}

// Close 关闭 Subscriber
func (s *RedisStreamsSubscriber) Close() error {
	s.logger.Info("Redis Streams Subscriber closed",
		nil)
	return nil
}
