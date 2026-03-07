package trace

import (
	"context"
	"strings"
	"time"

	"personal_assistant/pkg/observability/contextid"
	"personal_assistant/pkg/observability/w3c"
)

// spanContextKey 定义了私有的 Context Key 类型，防止 Key 冲突
type spanContextKey string

const (
	// ctxCurrentSpanIDKey 用于在 Context 中存储当前正在执行的 Span ID
	// 后续创建子 Span 时，会自动将其作为 ParentID
	ctxCurrentSpanIDKey spanContextKey = "observability_current_span_id"
)

// StartOptions 启动 Span 时的可选参数
// 提供给 StartSpan 函数，用于定制化 Span 的初始状态
type StartOptions struct {
	// SpanID 指定当前 Span 的 ID。若为空，将自动生成符合 W3C 标准的 ID
	SpanID string
	// ParentSpanID 指定父 Span 的 ID。若为空，将尝试从 Context 中自动获取
	ParentSpanID string
	// ParentTraceSpanID 显式指定父 Span ID（通常用于跨进程边界传递），优先级低于 ParentSpanID
	ParentTraceSpanID string
	// TraceID 指定链路 ID。若为空，将尝试从 Context 获取或生成新的
	TraceID string
	// RequestID 指定请求 ID。若为空，将尝试从 Context 获取
	RequestID string
	// Service 当前服务名称（如 "order-service"），用于区分不同服务的 Span
	Service string
	// Stage 当前阶段（如 "controller", "service", "db"），用于链路分层展示
	Stage string
	// Name 操作名称（如 "GET /users"），应保持低基数
	Name string
	// Kind Span 类型（如 "server", "client", "internal"），遵循 OpenTelemetry 语义
	Kind string
	// Tags 初始标签集合，用于携带上下文元数据
	Tags map[string]string
}

// SpanEvent 运行中的 Span 句柄
// 用于在 Span 生命周期内更新状态（如追加错误、完成 Span）
// 它持有 *Span 指针，确保状态更新是原地的
type SpanEvent struct {
	span *Span
}

// Span 获取当前 Span 的数据快照（指针）
// 注意：返回的 Span 可能正在被并发修改，仅限读取基础字段
func (e *SpanEvent) Span() *Span {
	if e == nil {
		return nil
	}
	return e.span
}

// WithErrorPayload 仅在错误场景补充载荷片段
// requestSnippet/responseSnippet: 请求/响应体的前 N 个字节
// 通常由中间件在检测到 HTTP 错误状态码时调用
func (e *SpanEvent) WithErrorPayload(requestSnippet, responseSnippet string) {
	if e == nil || e.span == nil {
		return
	}
	e.span.RequestSnippet = strings.TrimSpace(requestSnippet)
	e.span.ResponseSnippet = strings.TrimSpace(responseSnippet)
}

// WithErrorStack 在错误场景补充堆栈信息
// stack: 通常由 runtime/debug.Stack() 获取
func (e *SpanEvent) WithErrorStack(stack string) {
	if e == nil || e.span == nil {
		return
	}
	e.span.ErrorStack = strings.TrimSpace(stack)
}

// WithErrorDetail 在错误场景补充结构化错误明细(JSON 字符串)
// detail: 通常包含错误码、错误消息、发生错误的上下文参数等
func (e *SpanEvent) WithErrorDetail(detail string) {
	if e == nil || e.span == nil {
		return
	}
	e.span.ErrorDetailJSON = strings.TrimSpace(detail)
}

// End 结束 Span 并返回最终快照
// 此方法会标记 Span 的结束时间，计算耗时，并冻结状态
// 返回的 *Span 可直接用于持久化
func (e *SpanEvent) End(status, errorCode, message string, tags map[string]string) *Span {
	if e == nil || e.span == nil {
		return nil
	}
	// 记录结束时间（UTC），用于计算 Duration
	e.span.EndAt = time.Now().UTC()
	// 防御性编程：若 StartAt 未被正确设置（极少见），兜底为结束时间
	if e.span.StartAt.IsZero() {
		e.span.StartAt = e.span.EndAt
	}
	// 计算耗时（毫秒）
	e.span.DurationMs = e.span.EndAt.Sub(e.span.StartAt).Milliseconds()
	// 归一化状态码（ok/error）
	e.span.Status = normalizeStatus(status)
	// 记录错误码与消息（仅在 error 状态下有意义）
	e.span.ErrorCode = strings.TrimSpace(errorCode)
	e.span.Message = strings.TrimSpace(message)
	// 合并额外的 Tags
	e.span.Tags = mergeTags(e.span.Tags, tags)
	return e.span
}

// StartSpan 创建并挂载一个新 Span
// 这是开启链路追踪的核心入口。它会：
// 1. 自动处理 TraceID/SpanID 的生成与继承
// 2. 将新生成的 SpanID 注入返回的 Context，供下游使用
// 3. 返回 SpanEvent 句柄用于后续操作
func StartSpan(ctx context.Context, opt StartOptions) (context.Context, *SpanEvent) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. 确保 Context 中有基础 ID (RequestID, TraceID)
	// 2. 确保 Context 中有 W3C TraceContext
	ctx, ids := contextid.EnsureIDs(ctx)
	ctx, tc := contextid.EnsureTraceContext(ctx)

	// 确定当前 SpanID：优先使用参数，否则生成新 ID
	spanID := strings.TrimSpace(opt.SpanID)
	if !w3c.IsValidSpanID(strings.ToLower(spanID)) {
		spanID = w3c.NewSpanID()
	} else {
		spanID = strings.ToLower(spanID)
	}

	// 确定 ParentSpanID：自动解析父子关系
	parentSpanID := resolveParentSpanID(ctx, opt)

	// 确定 TraceID：优先参数，其次 Context，最后生成
	traceID := strings.TrimSpace(opt.TraceID)
	if !w3c.IsValidTraceID(strings.ToLower(traceID)) {
		traceID = ids.TraceID
	}
	if !w3c.IsValidTraceID(strings.ToLower(traceID)) {
		traceID = tc.TraceID
	}
	if !w3c.IsValidTraceID(strings.ToLower(traceID)) {
		traceID = w3c.NewTraceID()
	}

	// 确定 RequestID：优先参数，其次 Context
	requestID := strings.TrimSpace(opt.RequestID)
	if requestID == "" {
		requestID = ids.RequestID
	}

	// 规范化 ID 格式
	traceID = strings.ToLower(strings.TrimSpace(traceID))
	parentSpanID = strings.ToLower(strings.TrimSpace(parentSpanID))

	// 初始化 Span 对象
	span := &Span{
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		TraceID:      traceID,
		RequestID:    requestID,
		Service:      strings.TrimSpace(opt.Service),
		Stage:        strings.TrimSpace(opt.Stage),
		Name:         strings.TrimSpace(opt.Name),
		Kind:         strings.TrimSpace(opt.Kind),
		Status:       SpanStatusOK, // 默认为成功
		StartAt:      time.Now().UTC(),
		Tags:         cloneTags(opt.Tags),
	}

	// 更新 Context：
	// 1. 将当前 SpanID 设为 "CurrentSpanID"（下一次 StartSpan 会把它当做 Parent）
	ctx = context.WithValue(ctx, ctxCurrentSpanIDKey, spanID)
	// 2. 更新基础 IDs
	ctx = contextid.IntoContext(ctx, contextid.IDs{
		RequestID: requestID,
		TraceID:   traceID,
	})
	// 3. 更新 W3C TraceContext（TraceID 不变，SpanID 更新为当前 SpanID）
	ctx = contextid.IntoTraceContext(ctx, contextid.TraceContext{
		TraceID:    traceID,
		SpanID:     spanID, // 注意：W3C ParentID 字段在这里填的是“当前 Span ID”，供下游作为 Parent
		TraceFlags: w3c.NormalizeTraceFlags(tc.TraceFlags),
		TraceState: tc.TraceState,
	})
	return ctx, &SpanEvent{span: span}
}

// RunSpan 快捷执行一个 span 闭包，返回结束后的 span 与执行错误。
// 这是一个语法糖，用于简化 "Start -> Do -> End" 的样板代码。
// fn: 业务逻辑闭包，接收包含 Span 上下文的 ctx
func RunSpan(
	ctx context.Context,
	opt StartOptions,
	fn func(context.Context) error,
) (*Span, error) {
	// 启动 Span
	spanCtx, spanEvent := StartSpan(ctx, opt)
	if spanEvent == nil {
		// 防御性：如果 StartSpan 失败（极少见），降级执行 fn
		if fn == nil {
			return nil, nil
		}
		return nil, fn(spanCtx)
	}

	var err error
	if fn != nil {
		// 执行业务逻辑
		err = fn(spanCtx)
	}

	// 自动判定状态：有 error 即为失败
	status := SpanStatusOK
	message := ""
	if err != nil {
		status = SpanStatusError
		message = err.Error()
	}
	// 结束 Span 并返回
	return spanEvent.End(status, "", message, nil), err
}

// CurrentSpanID 获取当前上下文 SpanID
// 用于在业务代码中获取当前正在执行的 Span ID（例如打日志）
func CurrentSpanID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx.Value(ctxCurrentSpanIDKey).(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// resolveParentSpanID 决定当前 Span 的父 Span ID
// 优先级：
// 1. 显式参数 opt.ParentSpanID
// 2. Context 中的当前 Span（进程内父子调用）
// 3. 显式参数 opt.ParentTraceSpanID
// 4. Context 中的入站 Span（跨进程父子调用，如 HTTP Header 解析出的）
func resolveParentSpanID(ctx context.Context, opt StartOptions) string {
	if v := strings.TrimSpace(opt.ParentSpanID); v != "" {
		return v
	}
	if v := CurrentSpanID(ctx); v != "" {
		return v
	}
	if v := strings.TrimSpace(opt.ParentTraceSpanID); v != "" {
		return v
	}
	return contextid.IncomingParentSpanIDFromContext(ctx)
}

// normalizeStatus 归一化 Span 状态
// 仅保留 "ok" 和 "error"，其余均视为 "ok"
func normalizeStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == SpanStatusError {
		return SpanStatusError
	}
	return SpanStatusOK
}

// cloneTags 深拷贝 Tags map，防止并发读写冲突
func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

// mergeTags 合并两个 Tags map，返回新 map
// 如果 key 冲突，以 extra 为准
func mergeTags(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := cloneTags(base)
	if out == nil {
		out = make(map[string]string, len(extra))
	}
	for k, v := range extra {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = strings.TrimSpace(v)
	}
	return out
}
