package messaging

import (
	"context"
	"strings"
	"time"

	"personal_assistant/pkg/observability/contextid"
	"personal_assistant/pkg/observability/w3c"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisStreamPublisher 基于 Redis Stream 的发布者实现
type RedisStreamPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

func NewRedisStreamPublisher(
	client *redis.Client,
	logger *zap.Logger,
) *RedisStreamPublisher {
	return &RedisStreamPublisher{
		client: client,
		logger: logger,
	}
}

func (p *RedisStreamPublisher) Publish(
	ctx context.Context,
	msg *Message,
) error {
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}
	ctx, ids := contextid.EnsureIDs(ctx)
	ctx, tc := contextid.EnsureTraceContext(ctx)

	if traceparent := w3c.BuildTraceparent(tc); traceparent != "" {
		if _, ok := msg.Metadata["traceparent"]; !ok {
			msg.Metadata["traceparent"] = traceparent
		}
	}
	if tracestate := strings.TrimSpace(tc.TraceState); tracestate != "" {
		if _, ok := msg.Metadata["tracestate"]; !ok {
			msg.Metadata["tracestate"] = tracestate
		}
	}
	if ids.RequestID != "" {
		if _, ok := msg.Metadata["request_id"]; !ok {
			msg.Metadata["request_id"] = ids.RequestID
		}
	}

	// 如果没有设置发布时间，则设置为当前时间
	if msg.PublishedAt.IsZero() {
		msg.PublishedAt = time.Now()
	}

	values := map[string]interface{}{
		"id":           msg.ID,
		"key":          msg.Key, // 增加 Key 字段传递
		"payload":      msg.Payload,
		"occurred_at":  msg.OccurredAt.Format(time.RFC3339),
		"published_at": msg.PublishedAt.Format(time.RFC3339),
	}

	// 添加元数据
	for k, v := range msg.Metadata {
		values["meta_"+k] = v
	}

	// 使用 XAdd 发布到 Redis Stream
	// Stream 名称即为 Topic
	cmd := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: msg.Topic,
		Values: values,
	})

	return cmd.Err()
}

func (p *RedisStreamPublisher) Close() error {
	return nil // Redis client 通常由外部管理生命周期
}
