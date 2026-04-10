package sse

/*
	本接口，暂时只对 频道channel做了回放，后续如果需要对主体subject做回放。
*/

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisReplayStore 使用 Redis Stream 保存可回放的 durable 事件。
// 这里选择 Stream 而不是普通列表，是因为 Stream 天然支持按事件 ID 续读，适合 SSE 断线重连补发。
type RedisReplayStore struct {
	client       *redis.Client
	streamPrefix string
}

// persistedStreamEvent 是写入 Redis 前的持久化载体。
// 它把 `[]byte` 数据改为字符串，是为了简化 JSON 编码并避免 Redis 端出现二进制兼容问题。
type persistedStreamEvent struct {
	StreamKind string            `json:"stream_kind"`
	Channel    string            `json:"channel"`
	TenantID   uint64            `json:"tenant_id"`
	SubjectID  uint64            `json:"subject_id"`
	EventName  string            `json:"event_name"`
	Data       string            `json:"data"`
	OccurredAt time.Time         `json:"occurred_at"`
	RetryMS    int64             `json:"retry_ms"`
	Durable    bool              `json:"durable"`
	RequestID  string            `json:"request_id"`
	TraceID    string            `json:"trace_id"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// NewRedisReplayStore 创建一个 Redis Stream 回放存储。
// 参数：
//   - client：Redis 客户端。
//   - streamPrefix：流名前缀。
//
// 返回值：
//   - *RedisReplayStore：回放存储实例。
//
// 核心流程：
//  1. 仅清洗前缀，不在此处探测 Redis 是否可用。
//
// 注意事项：
//   - 可用性检查延迟到真实读写阶段，是为了让初始化流程保持轻量，并由运行时错误决定是否降级。
func NewRedisReplayStore(client *redis.Client, streamPrefix string) *RedisReplayStore {
	return &RedisReplayStore{
		client:       client,
		streamPrefix: strings.TrimSpace(streamPrefix),
	}
}

// Append 负责把 durable 事件写入 Redis Stream。
// 参数：
//   - ctx：写入上下文。
//   - evt：待持久化事件。
//
// 返回值：
//   - error：序列化或 Redis 写入失败时返回错误。
//
// 核心流程：
//  1. 未启用存储、事件为空或事件本身不可回放时直接空返回。
//  2. 把运行时事件转换成持久化载体并编码成 JSON。
//  3. 使用 XADD 追加到 channel 对应的 Stream 中。
//  4. 若事件尚未带 EventID，则回填 Redis 生成的 Stream ID。
//
// 注意事项：
//   - 只有 `Durable=true` 的事件才入库，是为了把实时噪声与真正需要补发的状态变更区分开。
func (r *RedisReplayStore) Append(ctx context.Context, evt *StreamEvent) error {
	if r == nil || r.client == nil || evt == nil || !evt.Durable {
		return nil
	}

	// 先转换成稳定的持久化结构，避免直接把运行时对象序列化导致未来字段调整影响兼容性。
	store := persistedStreamEvent{
		StreamKind: string(evt.StreamKind),
		Channel:    evt.Channel,
		TenantID:   evt.TenantID,
		SubjectID:  evt.SubjectID,
		EventName:  evt.EventName,
		Data:       string(evt.Data),
		OccurredAt: evt.OccurredAt,
		RetryMS:    evt.RetryMS,
		Durable:    evt.Durable,
		RequestID:  evt.RequestID,
		TraceID:    evt.TraceID,
		Meta:       evt.Meta,
	}
	raw, err := json.Marshal(store)
	if err != nil {
		return err
	}

	// Stream ID 由 Redis 生成，天然适合用作 Last-Event-ID 的续读锚点。
	id, err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: r.streamKey(evt.Channel),
		Values: map[string]interface{}{"event": string(raw)},
	}).Result()
	if err != nil {
		return err
	}

	// 回填 EventID 是为了让后续实时链路与回放链路使用同一套事件标识。
	if evt.EventID == "" {
		evt.EventID = id
	}
	return nil
}

// ReplayAfter 负责从某个 Last-Event-ID 之后读取历史事件。
// 参数：
//   - ctx：读取上下文。
//   - channel：目标频道。
//   - lastEventID：客户端最后一次确认收到的事件 ID。
//   - limit：最大回放条数。
//
// 返回值：
//   - []*StreamEvent：可继续发送给客户端的事件列表。
//   - error：Redis 读取失败时返回错误。
//
// 核心流程：
//  1. 基础依赖为空时直接降级为空结果。
//  2. 归一化 limit，并根据 lastEventID 生成排他起点。
//  3. 读取 Stream 区间并逐条反序列化回运行时事件。
//  4. 对损坏记录做跳过处理，保证回放链路尽量继续前进。
//
// 注意事项：
//   - 使用 `(%s` 构造起点是为了排除 lastEventID 本身，避免重连后把已收到的最后一条事件重复推送。
func (r *RedisReplayStore) ReplayAfter(
	ctx context.Context,
	channel string,
	lastEventID string,
	limit int,
) ([]*StreamEvent, error) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if r == nil || r.client == nil || channel == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}

	start := "-"
	if strings.TrimSpace(lastEventID) != "" {
		start = fmt.Sprintf("(%s", strings.TrimSpace(lastEventID))
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	items, err := r.client.XRangeN(ctx, r.streamKey(channel), start, "+", int64(limit)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	result := make([]*StreamEvent, 0, len(items))
	for _, item := range items {
		raw, _ := item.Values["event"].(string)
		if strings.TrimSpace(raw) == "" {
			continue
		}

		// 单条历史记录损坏时跳过当前项，而不是让整个重放失败，尽量提高重连恢复成功率。
		var persisted persistedStreamEvent
		if err := json.Unmarshal([]byte(raw), &persisted); err != nil {
			continue
		}
		result = append(result, &StreamEvent{
			EventID:    item.ID,
			StreamKind: StreamKind(persisted.StreamKind),
			Channel:    persisted.Channel,
			TenantID:   persisted.TenantID,
			SubjectID:  persisted.SubjectID,
			EventName:  persisted.EventName,
			Data:       []byte(persisted.Data),
			OccurredAt: persisted.OccurredAt,
			RetryMS:    persisted.RetryMS,
			Durable:    persisted.Durable,
			RequestID:  persisted.RequestID,
			TraceID:    persisted.TraceID,
			Meta:       persisted.Meta,
		})
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return result, nil
}

// streamKey 负责把业务 channel 映射成 Redis Stream 键名。
// 参数：
//   - channel：业务频道名。
//
// 返回值：
//   - string：最终 Redis Stream key。
//
// 核心流程：
//  1. 若未配置前缀则使用默认前缀。
//  2. 统一 trim channel，避免因为空白差异导致写入和读取到不同 key。
//
// 注意事项：
//   - 键名拼装集中到这里，可以确保 Append 与 ReplayAfter 始终命中同一条 Stream。
func (r *RedisReplayStore) streamKey(channel string) string {
	prefix := r.streamPrefix
	if prefix == "" {
		prefix = "sse:replay"
	}
	return prefix + ":" + strings.TrimSpace(channel)
}
