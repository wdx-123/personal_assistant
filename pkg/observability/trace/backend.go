package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/pkg/observability/w3c"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// streamAckSpan 用于在消费端追踪需要确认（ACK）的消息
// 包含原始 Redis 消息 ID 和解析后的 Span 实体
type streamAckSpan struct {
	msgID string
	span  *entity.ObservabilityTraceSpan
}

// Backend 轻量全链路追踪后端
// 实现了 TraceBackend 接口，负责 Span 的收集、缓冲、流式传输和持久化。
// 架构设计：
// 1. 内存缓冲：使用 Go Channel (Normal/Critical) 进行初步缓冲，防止阻塞业务线程。
// 2. 流式传输：使用 Redis Stream 作为持久化前的缓冲队列，支持多实例部署时的削峰填谷。
// 3. 异步持久化：后台 Goroutine 从 Redis Stream 消费数据并批量写入数据库 (Store)。
// 4. 降级策略：Redis 不可用时尝试直接写库；队列满时丢弃非关键 Span。
type Backend struct {
	redisClient *redis.Client       // Redis 客户端，用于 Stream 操作
	store       Store               // 存储接口，用于最终持久化 Span
	log         *zap.Logger         // 日志记录器
	opt         Options             // 配置选项
	redactKeys  map[string]struct{} // 需要脱敏的键集合

	// normalQueue 普通 Span 的内存缓冲队列
	// 用于存放采样后的成功请求 Span，队列满时可能会丢弃
	normalQueue chan *Span
	// criticalQueue 关键 Span 的内存缓冲队列
	// 用于存放错误 Span，队列满时会尝试阻塞等待，尽可能保证不丢失
	criticalQueue chan *Span

	// droppedSuccess 记录因队列满而丢弃的成功 Span 数量
	droppedSuccess atomic.Int64

	startOnce    sync.Once // 确保 Writer 仅启动一次
	consumerOnce sync.Once // 确保 Consumer 仅启动一次
}

// NewBackend 创建一个新的 Backend 实例
// 初始化配置默认值、脱敏键和内存队列
func NewBackend(redisClient *redis.Client, store Store, log *zap.Logger, opt Options) *Backend {
	if log == nil {
		log = zap.NewNop()
	}
	// 设置配置默认值
	if opt.StreamKey == "" {
		opt.StreamKey = "traces:stream"
	}
	if opt.StreamGroup == "" {
		opt.StreamGroup = "traces_persist_group"
	}
	if opt.StreamConsumer == "" {
		opt.StreamConsumer = fmt.Sprintf("go-trace-%d", os.Getpid())
	}
	if opt.StreamReadCount <= 0 {
		opt.StreamReadCount = 200
	}
	if opt.StreamBlock <= 0 {
		opt.StreamBlock = time.Second
	}
	if opt.PendingIdle <= 0 {
		opt.PendingIdle = time.Minute
	}
	if opt.DBBatchSize <= 0 {
		opt.DBBatchSize = 100
	}
	if opt.DBFlushInterval <= 0 {
		opt.DBFlushInterval = 800 * time.Millisecond
	}
	if opt.NormalQueueSize <= 0 {
		opt.NormalQueueSize = 4096
	}
	if opt.CriticalQueueSize <= 0 {
		opt.CriticalQueueSize = 1024
	}
	if opt.EnqueueTimeout <= 0 {
		opt.EnqueueTimeout = 30 * time.Millisecond
	}
	if opt.SuccessSampleRate <= 0 || opt.SuccessSampleRate > 1 {
		opt.SuccessSampleRate = 1
	}
	if opt.MaxPayloadBytes <= 0 {
		opt.MaxPayloadBytes = 4096
	}
	if opt.MaxStackBytes <= 0 {
		opt.MaxStackBytes = 8192
	}
	if opt.MaxDetailBytes <= 0 {
		opt.MaxDetailBytes = 4096
	}
	if opt.SuccessRetentionDays <= 0 {
		opt.SuccessRetentionDays = 5
	}
	if opt.ErrorRetentionDays <= 0 {
		opt.ErrorRetentionDays = 10
	}
	if strings.TrimSpace(opt.ServiceName) == "" {
		opt.ServiceName = "unknown_service"
	}

	return &Backend{
		redisClient:    redisClient,
		store:          store,
		log:            log,
		opt:            opt,
		redactKeys:     normalizeRedactKeys(opt.RedactKeys),
		normalQueue:    make(chan *Span, opt.NormalQueueSize),
		criticalQueue:  make(chan *Span, opt.CriticalQueueSize),
		droppedSuccess: atomic.Int64{},
	}
}

// Start 启动 Span 收集器（Writer）
// 开启后台 Goroutine 将内存队列中的 Span 写入 Redis Stream
func (b *Backend) Start(ctx context.Context) error {
	if !b.opt.Enabled {
		return nil
	}
	b.startOnce.Do(func() {
		go b.runWriter(ctx)
	})
	return nil
}

// StartConsumer 启动 Span 消费者
// 开启后台 Goroutine 从 Redis Stream 消费 Span 并持久化到数据库
// 会自动创建 Consumer Group
func (b *Backend) StartConsumer(ctx context.Context) error {
	if !b.opt.Enabled {
		return nil
	}
	if b.redisClient == nil || b.store == nil {
		return fmt.Errorf("start trace consumer failed: missing redis/store")
	}

	// 尝试创建消费者组，如果已存在则忽略 BUSYGROUP 错误
	err := b.redisClient.XGroupCreateMkStream(ctx, b.opt.StreamKey, b.opt.StreamGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}

	b.consumerOnce.Do(func() {
		go b.runConsumer(ctx)
	})
	return nil
}

// RecordSpan 记录一个 Span
// 包含采样逻辑：仅在采样命中或发生错误时记录
// 包含分流逻辑：错误 Span 进入 Critical 队列，正常 Span 进入 Normal 队列
func (b *Backend) RecordSpan(
	ctx context.Context,
	span *Span,
) error {
	if !b.opt.Enabled || span == nil {
		return nil
	}

	// 标准化并丰富 Span 信息（ID生成、截断、脱敏等）
	enriched := b.normalizeSpan(span)

	// 采样逻辑：如果是正常状态且采样率小于 1，则按概率丢弃
	if enriched.Status != SpanStatusError && b.opt.SuccessSampleRate < 1 {
		if rand.Float64() > b.opt.SuccessSampleRate {
			return nil
		}
	}

	isCritical := enriched.Status == SpanStatusError
	return b.enqueue(ctx, enriched, isCritical)
}

// ListByRequestID 根据 RequestID 查询 Span 列表
func (b *Backend) ListByRequestID(
	ctx context.Context,
	requestID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*Span, int64, error) {
	if b.store == nil {
		return nil, 0, nil
	}
	rows, total, err := b.store.ListByRequestID(
		ctx,
		strings.TrimSpace(requestID),
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		return nil, 0, err
	}
	return b.toSpans(rows), total, nil
}

// ListByTraceID 根据 TraceID 查询 Span 列表
func (b *Backend) ListByTraceID(
	ctx context.Context,
	traceID string,
	limit, offset int,
	includePayload bool,
	includeErrorDetail bool,
) ([]*Span, int64, error) {
	if b.store == nil {
		return nil, 0, nil
	}
	rows, total, err := b.store.ListByTraceID(
		ctx,
		strings.TrimSpace(traceID),
		limit,
		offset,
		includePayload,
		includeErrorDetail,
	)
	if err != nil {
		return nil, 0, err
	}
	return b.toSpans(rows), total, nil
}

// Query 根据复杂条件查询 Span 列表
func (b *Backend) Query(ctx context.Context, q *Query) ([]*Span, int64, error) {
	if b.store == nil {
		return nil, 0, nil
	}
	rows, total, err := b.store.Query(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	return b.toSpans(rows), total, nil
}

// CleanupBeforeByStatus 清理指定时间之前的 Span 数据
// 通常用于定时任务清理过期的日志
func (b *Backend) CleanupBeforeByStatus(
	ctx context.Context,
	status string,
	before time.Time,
) error {
	if !b.opt.Enabled || b.store == nil {
		return nil
	}
	return b.store.DeleteBeforeByStatus(ctx, status, before)
}

// runWriter 负责将内存队列中的 Span 写入 Redis Stream
// 采用批量写入策略，提高 Redis 吞吐量
func (b *Backend) runWriter(ctx context.Context) {
	ticker := time.NewTicker(b.opt.DBFlushInterval)
	defer ticker.Stop()

	buffer := make([]*Span, 0, b.opt.DBBatchSize)

	// flush 将缓冲区的数据写入 Redis Stream
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		// 优先写入 Redis Stream
		if err := b.flushToStream(ctx, buffer); err != nil {
			b.log.Error("trace flush to stream failed", zap.Error(err), zap.Int("rows", len(buffer)))
			// Redis 写入失败时，降级尝试直接写入 DB，避免数据丢失
			if persistErr := b.flushToDB(ctx, buffer); persistErr != nil {
				b.log.Error("trace fallback flush to db failed", zap.Error(persistErr), zap.Int("rows", len(buffer)))
			}
		}
		// 清空缓冲区
		buffer = buffer[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		// 优先处理关键队列（错误日志）
		case span := <-b.criticalQueue:
			if span != nil {
				buffer = append(buffer, span)
			}
		// 处理普通队列
		case span := <-b.normalQueue:
			if span != nil {
				buffer = append(buffer, span)
			}
		// 定时刷新
		case <-ticker.C:
			flush()
			continue
		}
		// 缓冲区满即刷新
		if len(buffer) >= b.opt.DBBatchSize {
			flush()
		}
	}
}

// flushToStream 批量写入 Redis Stream
func (b *Backend) flushToStream(
	ctx context.Context,
	spans []*Span,
) error {
	if b.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}
	pipe := b.redisClient.Pipeline()
	for _, span := range spans {
		if span == nil {
			continue
		}
		payload, err := json.Marshal(span)
		if err != nil {
			continue
		}
		// 使用 XAdd 添加消息到 Stream
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: b.opt.StreamKey,
			Values: map[string]any{
				"payload": string(payload),
			},
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

// flushToDB 批量写入数据库（降级路径）
func (b *Backend) flushToDB(
	ctx context.Context,
	spans []*Span,
) error {
	if b.store == nil {
		return nil
	}
	rows := make([]*entity.ObservabilityTraceSpan, 0, len(spans))
	for _, span := range spans {
		if span == nil {
			continue
		}
		rows = append(rows, b.toEntity(span))
	}
	return b.store.BatchCreateIgnoreDup(ctx, rows)
}

// runConsumer 消费者主循环
// 负责：
// 1. 处理长时间未确认的 Pending 消息 (XAutoClaim)
// 2. 读取新消息 (XReadGroup)
// 3. 批量持久化到 DB
// 4. 确认消息 (XAck)
func (b *Backend) runConsumer(
	ctx context.Context,
) {
	ticker := time.NewTicker(b.opt.DBFlushInterval)
	defer ticker.Stop()

	pendingCursor := "0-0"
	buffer := make([]*streamAckSpan, 0, b.opt.DBBatchSize)

	// flush 将缓冲区数据写入 DB 并 ACK
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		rows := make([]*entity.ObservabilityTraceSpan, 0, len(buffer))
		ackIDs := make([]string, 0, len(buffer))
		for _, item := range buffer {
			if item == nil || item.span == nil {
				continue
			}
			rows = append(rows, item.span)
			ackIDs = append(ackIDs, item.msgID)
		}
		if len(rows) == 0 {
			buffer = buffer[:0]
			return
		}
		// 批量写入数据库
		if err := b.store.BatchCreateIgnoreDup(ctx, rows); err != nil {
			b.log.Error("persist trace spans failed", zap.Error(err), zap.Int("rows", len(rows)))
			// 写入失败不 ACK，下次会被重新消费或 Claim
			buffer = buffer[:0]
			return
		}
		// 写入成功后批量 ACK
		if err := b.redisClient.XAck(ctx, b.opt.StreamKey, b.opt.StreamGroup, ackIDs...).Err(); err != nil {
			b.log.Error("ack trace stream messages failed", zap.Error(err))
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-ticker.C:
			flush()
		default:
		}

		// 1. 尝试认领（Claim）长时间 Pending 的消息（处理崩溃的消费者遗留的消息）
		claimed, next, claimErr := b.redisClient.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   b.opt.StreamKey,
			Group:    b.opt.StreamGroup,
			Consumer: b.opt.StreamConsumer,
			MinIdle:  b.opt.PendingIdle,
			Start:    pendingCursor,
			Count:    b.opt.StreamReadCount,
		}).Result()
		if claimErr == nil {
			pendingCursor = next
			b.consumeMessages(ctx, claimed, &buffer)
			if len(buffer) >= b.opt.DBBatchSize {
				flush()
			}
		} else if claimErr != redis.Nil {
			b.log.Error("trace xautoclaim failed", zap.Error(claimErr))
		}

		// 2. 读取新消息
		streams, err := b.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.opt.StreamGroup,
			Consumer: b.opt.StreamConsumer,
			Streams:  []string{b.opt.StreamKey, ">"}, // ">" 表示读取从未分发给其他消费者的消息
			Count:    b.opt.StreamReadCount,
			Block:    b.opt.StreamBlock,
		}).Result()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			b.log.Error("trace xreadgroup failed", zap.Error(err))
			time.Sleep(200 * time.Millisecond) // 避免错误循环中的紧密轮询
			continue
		}

		for _, stream := range streams {
			b.consumeMessages(ctx, stream.Messages, &buffer)
			if len(buffer) >= b.opt.DBBatchSize {
				flush()
			}
		}
	}
}

// consumeMessages 解析 Redis 消息并添加到缓冲区
func (b *Backend) consumeMessages(
	ctx context.Context,
	msgs []redis.XMessage,
	buffer *[]*streamAckSpan,
) {
	for _, msg := range msgs {
		payload, ok := msg.Values["payload"]
		if !ok {
			// 格式错误直接 ACK 丢弃，避免反复消费
			_ = b.redisClient.XAck(ctx, b.opt.StreamKey, b.opt.StreamGroup, msg.ID).Err()
			continue
		}
		raw := toString(payload)
		if raw == "" {
			_ = b.redisClient.XAck(ctx, b.opt.StreamKey, b.opt.StreamGroup, msg.ID).Err()
			continue
		}
		var span Span
		if err := json.Unmarshal([]byte(raw), &span); err != nil {
			b.log.Error("invalid trace span payload", zap.Error(err))
			_ = b.redisClient.XAck(ctx, b.opt.StreamKey, b.opt.StreamGroup, msg.ID).Err()
			continue
		}
		*buffer = append(*buffer, &streamAckSpan{
			msgID: msg.ID,
			span:  b.toEntity(b.normalizeSpan(&span)),
		})
	}
}

// enqueue 将 Span 放入内存队列
// critical=true: 关键 Span，会尝试等待 EnqueueTimeout，队列满则报错
// critical=false: 普通 Span，如果配置 DropSuccessOnOverload，队列满直接丢弃
func (b *Backend) enqueue(ctx context.Context, span *Span, critical bool) error {
	if critical {
		select {
		case b.criticalQueue <- span:
			return nil
		default:
		}
		// 关键队列满，稍微等待
		timer := time.NewTimer(b.opt.EnqueueTimeout)
		defer timer.Stop()
		select {
		case b.criticalQueue <- span:
			return nil
		case <-timer.C:
			return fmt.Errorf("critical trace queue overload")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if b.opt.DropSuccessOnOverload {
		select {
		case b.normalQueue <- span:
			return nil
		default:
			// 队列满直接丢弃并计数
			b.droppedSuccess.Add(1)
			return nil
		}
	}

	// 默认行为：稍微等待
	timer := time.NewTimer(b.opt.EnqueueTimeout)
	defer timer.Stop()
	select {
	case b.normalQueue <- span:
		return nil
	case <-timer.C:
		b.droppedSuccess.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// normalizeSpan 标准化 Span 数据
// 处理字段修剪、ID 生成、默认值填充、脱敏、截断等
func (b *Backend) normalizeSpan(span *Span) *Span {
	cpy := *span
	// 清理空白字符
	cpy.SpanID = strings.TrimSpace(cpy.SpanID)
	cpy.ParentSpanID = strings.TrimSpace(cpy.ParentSpanID)
	cpy.TraceID = strings.TrimSpace(cpy.TraceID)
	cpy.RequestID = strings.TrimSpace(cpy.RequestID)
	cpy.Service = strings.TrimSpace(cpy.Service)
	cpy.Stage = strings.TrimSpace(cpy.Stage)
	cpy.Name = strings.TrimSpace(cpy.Name)
	cpy.Kind = strings.TrimSpace(cpy.Kind)
	cpy.Status = normalizeStatus(cpy.Status)
	cpy.ErrorCode = strings.TrimSpace(cpy.ErrorCode)
	cpy.Message = strings.TrimSpace(cpy.Message)
	cpy.ErrorStack = strings.TrimSpace(cpy.ErrorStack)
	cpy.ErrorDetailJSON = strings.TrimSpace(cpy.ErrorDetailJSON)

	// 确保 ID 有效性，无效则生成新 ID 或复用 RequestID
	if !w3c.IsValidSpanID(cpy.SpanID) {
		cpy.SpanID = w3c.NewSpanID()
	}
	if !w3c.IsValidTraceID(cpy.TraceID) {
		if w3c.IsValidTraceID(cpy.RequestID) {
			cpy.TraceID = cpy.RequestID
		} else {
			cpy.TraceID = w3c.NewTraceID()
		}
	}
	if cpy.Service == "" {
		cpy.Service = b.opt.ServiceName
	}
	// 时间处理
	if cpy.StartAt.IsZero() {
		cpy.StartAt = time.Now().UTC()
	}
	if cpy.EndAt.IsZero() {
		cpy.EndAt = cpy.StartAt
	}
	if cpy.DurationMs <= 0 {
		cpy.DurationMs = cpy.EndAt.Sub(cpy.StartAt).Milliseconds()
	}

	// 成功状态下，清空错误相关字段以节省空间
	if cpy.Status != SpanStatusError {
		cpy.RequestSnippet = ""
		cpy.ResponseSnippet = ""
		cpy.ErrorStack = ""
		cpy.ErrorDetailJSON = ""
	} else {
		// 错误状态下，进行脱敏和截断
		if b.opt.CaptureErrorPayload {
			cpy.RequestSnippet = cutString(
				redactJSONIfPossible(strings.TrimSpace(cpy.RequestSnippet), b.redactKeys),
				b.opt.MaxPayloadBytes,
			)
			cpy.ResponseSnippet = cutString(
				redactJSONIfPossible(strings.TrimSpace(cpy.ResponseSnippet), b.redactKeys),
				b.opt.MaxPayloadBytes,
			)
		} else {
			cpy.RequestSnippet = ""
			cpy.ResponseSnippet = ""
		}

		if b.opt.CaptureErrorStack {
			cpy.ErrorStack = cutString(cpy.ErrorStack, b.opt.MaxStackBytes)
		} else {
			cpy.ErrorStack = ""
		}

		if b.opt.CaptureErrorDetail {
			cpy.ErrorDetailJSON = normalizeDetailJSON(cpy.ErrorDetailJSON, b.opt.MaxDetailBytes, b.redactKeys)
		} else {
			cpy.ErrorDetailJSON = ""
		}
	}
	cpy.Tags = cloneTags(cpy.Tags)
	return &cpy
}

// toEntity 将 Span 转换为数据库实体
func (b *Backend) toEntity(span *Span) *entity.ObservabilityTraceSpan {
	return &entity.ObservabilityTraceSpan{
		SpanID:          span.SpanID,
		ParentSpanID:    span.ParentSpanID,
		TraceID:         span.TraceID,
		RequestID:       span.RequestID,
		Service:         span.Service,
		Stage:           span.Stage,
		Name:            span.Name,
		Kind:            span.Kind,
		Status:          span.Status,
		StartAt:         span.StartAt,
		EndAt:           span.EndAt,
		DurationMs:      span.DurationMs,
		ErrorCode:       span.ErrorCode,
		Message:         span.Message,
		TagsJSON:        marshalTags(span.Tags),
		RequestSnippet:  span.RequestSnippet,
		ResponseSnippet: span.ResponseSnippet,
		ErrorStack:      span.ErrorStack,
		ErrorDetailJSON: span.ErrorDetailJSON,
	}
}

// toSpans 将数据库实体列表转换为 Span 列表
func (b *Backend) toSpans(rows []*entity.ObservabilityTraceSpan) []*Span {
	out := make([]*Span, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		out = append(out, &Span{
			SpanID:          row.SpanID,
			ParentSpanID:    row.ParentSpanID,
			TraceID:         row.TraceID,
			RequestID:       row.RequestID,
			Service:         row.Service,
			Stage:           row.Stage,
			Name:            row.Name,
			Kind:            row.Kind,
			Status:          row.Status,
			StartAt:         row.StartAt,
			EndAt:           row.EndAt,
			DurationMs:      row.DurationMs,
			ErrorCode:       row.ErrorCode,
			Message:         row.Message,
			Tags:            parseTags(row.TagsJSON),
			RequestSnippet:  row.RequestSnippet,
			ResponseSnippet: row.ResponseSnippet,
			ErrorStack:      row.ErrorStack,
			ErrorDetailJSON: row.ErrorDetailJSON,
		})
	}
	return out
}

// normalizeDetailJSON 标准化错误详情 JSON
// 解析 JSON -> 脱敏 -> 重新序列化 -> 截断
// 如果无法解析为 JSON，则将其包装在 {"raw": "..."} 中
func normalizeDetailJSON(
	raw string,
	maxBytes int,
	redactKeys map[string]struct{},
) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return packRawDetail(cutString(raw, maxBytes), maxBytes)
	}
	redactJSONValue(parsed, redactKeys)
	data, err := json.Marshal(parsed)
	if err != nil {
		return packRawDetail(cutString(raw, maxBytes), maxBytes)
	}
	if maxBytes <= 0 || len(data) <= maxBytes {
		return string(data)
	}
	return packRawDetail(cutString(string(data), maxBytes), maxBytes)
}

// packRawDetail 将原始字符串包装为 JSON 对象
// 会尝试截断以适应 maxBytes
func packRawDetail(raw string, maxBytes int) string {
	raw = strings.TrimSpace(raw)
	if maxBytes <= 0 {
		maxBytes = 4096
	}

	emptyJSON := `{"raw":""}`
	if maxBytes < len(emptyJSON) {
		// 当上限太小无法容纳最小合法包装时，返回固定空值 JSON。
		return emptyJSON
	}

	marshalWithinLimit := func(s string) (string, bool) {
		data, err := json.Marshal(struct {
			Raw string `json:"raw"`
		}{Raw: s})
		if err != nil {
			return "", false
		}
		if len(data) > maxBytes {
			return "", false
		}
		return string(data), true
	}

	if out, ok := marshalWithinLimit(raw); ok {
		return out
	}

	// 按 rune 二分截断，保证 UTF-8 安全且尽量接近 maxBytes 上限。
	runes := []rune(raw)
	lo, hi := 0, len(runes)
	best := emptyJSON
	for lo <= hi {
		mid := lo + (hi-lo)/2
		out, ok := marshalWithinLimit(string(runes[:mid]))
		if ok {
			best = out
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	return best
}

// normalizeRedactKeys 标准化脱敏键配置
// 转小写、去下划线等，以便不区分大小写和格式进行匹配
func normalizeRedactKeys(keys []string) map[string]struct{} {
	if len(keys) == 0 {
		keys = []string{
			"password",
			"token",
			"authorization",
			"cookie",
			"secret",
			"apikey",
			"access_token",
			"refresh_token",
		}
	}
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := normalizeRedactKey(key)
		if normalized == "" {
			continue
		}
		out[normalized] = struct{}{}
	}
	return out
}

// normalizeRedactKey 归一化键名
func normalizeRedactKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")
	return key
}

// shouldRedactKey 检查是否应该脱敏该键
func shouldRedactKey(key string, redactKeys map[string]struct{}) bool {
	if len(redactKeys) == 0 {
		return false
	}
	_, ok := redactKeys[normalizeRedactKey(key)]
	return ok
}

// redactJSONIfPossible 尝试解析并脱敏 JSON 字符串
func redactJSONIfPossible(raw string, redactKeys map[string]struct{}) string {
	if strings.TrimSpace(raw) == "" || len(redactKeys) == 0 {
		return raw
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return raw
	}
	redactJSONValue(parsed, redactKeys)
	data, err := json.Marshal(parsed)
	if err != nil {
		return raw
	}
	return string(data)
}

// redactJSONValue 递归脱敏 JSON 值
// 支持 Map 和 Slice 结构
func redactJSONValue(v any, redactKeys map[string]struct{}) {
	switch typed := v.(type) {
	case map[string]any:
		for k, child := range typed {
			if shouldRedactKey(k, redactKeys) {
				typed[k] = "***"
				continue
			}
			redactJSONValue(child, redactKeys)
		}
	case []any:
		for _, child := range typed {
			redactJSONValue(child, redactKeys)
		}
	}
}

// cutString 字符串截断辅助函数
func cutString(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes]
}

// marshalTags 序列化标签 Map 为 JSON
func marshalTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "{}"
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// parseTags 解析 JSON 标签为 Map
func parseTags(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := make(map[string]string)
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// toString 辅助转换函数
func toString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

var _ TraceBackend = (*Backend)(nil)
