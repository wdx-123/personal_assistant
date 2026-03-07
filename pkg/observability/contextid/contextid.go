package contextid

import (
	"context"
	"strings"

	"personal_assistant/pkg/observability/w3c"

	"github.com/google/uuid"
)

/*
Package contextid 负责在 context.Context 中保存/读取“请求级 ID”与 W3C Trace Context。

它解决两个核心问题：
1) 跨函数/跨层传递 request_id、trace_id（用于日志关联、问题定位）。
2) 兼容 W3C Trace Context（traceparent / tracestate），用于链路追踪的父子 span 关联。

注意：
- 本包仅做“上下文携带与规范化/校验”，不负责实际采集/上报 spans。
- 所有写入 context 的 key 都使用私有类型，避免与业务代码发生 key 冲突。
*/

const (
	// DefaultRequestIDHeader 是常见的 request id HTTP Header 名。
	// 如果网关/上游没传，通常在入口处自行生成。
	DefaultRequestIDHeader = "X-Request-ID"

	// GinKey* 是建议在 gin.Context 中使用的 key（用于中间件传递、统一记录日志/trace）。
	// 这些 key 通常对应：
	// - RequestID：每个 HTTP 请求一个（uuid）
	// - TraceID：分布式链路的 trace id（W3C 16-byte hex）
	// - Traceparent / Tracestate：W3C Trace Context 原始头
	// - ErrorCode / PanicStack：用于统一错误/异常上报
	GinKeyRequestID   = "obs_request_id"
	GinKeyTraceID     = "obs_trace_id"
	GinKeyTraceparent = "obs_traceparent"
	GinKeyTracestate  = "obs_tracestate"
	GinKeyErrorCode   = "obs_error_code"
	GinKeyPanicStack  = "obs_panic_stack"
)

// idsContextKey 是 context key 的私有类型，用于避免与其他包的 key 字符串冲突。
type idsContextKey string

const (
	// ctxRequestIDKey / ctxTraceIDKey 保存业务侧常用的 request_id 与 trace_id。
	ctxRequestIDKey idsContextKey = "observability_request_id"
	ctxTraceIDKey   idsContextKey = "observability_trace_id"

	// ctxTraceContextKey 保存 W3C Trace Context（TraceID/SpanID/Flags/State）。
	// 用于链路追踪的父子关系衔接（traceparent/tracestate）。
	ctxTraceContextKey idsContextKey = "observability_trace_context"

	// ctxIncomingParentSpanKey 保存“入站请求携带的父 span id”。
	// 常见场景：从 traceparent 中解析出 parent span id，用于当前服务创建 root/span 时挂接。
	ctxIncomingParentSpanKey idsContextKey = "observability_incoming_parent_span_id"
)

// IDs 是本服务内部常用的两个关联 ID：
// - RequestID：请求维度（通常 uuid），适合日志检索与定位（更“人类友好”）
// - TraceID：链路维度（W3C trace id），用于分布式追踪串联
type IDs struct {
	RequestID string
	TraceID   string
}

// TraceContext 表示 W3C Trace Context 在上下文中的副本。
// 直接复用 pkg/observability/w3c 的定义，避免重复结构体。
type TraceContext = w3c.TraceContext

// IntoContext 将 IDs 写入 ctx。
// - ctx 为 nil 时使用 context.Background()
// - 字段会进行 TrimSpace，空值不写入（保持上下文干净）
func IntoContext(ctx context.Context, ids IDs) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if v := strings.TrimSpace(ids.RequestID); v != "" {
		ctx = context.WithValue(ctx, ctxRequestIDKey, v)
	}
	if v := strings.TrimSpace(ids.TraceID); v != "" {
		ctx = context.WithValue(ctx, ctxTraceIDKey, v)
	}
	return ctx
}

// FromContext 从 ctx 读取 IDs（不存在则返回空结构）。
// 返回值会 TrimSpace，避免上下游带入意外空白字符。
func FromContext(ctx context.Context) IDs {
	if ctx == nil {
		return IDs{}
	}
	ids := IDs{}
	if v, ok := ctx.Value(ctxRequestIDKey).(string); ok {
		ids.RequestID = strings.TrimSpace(v)
	}
	if v, ok := ctx.Value(ctxTraceIDKey).(string); ok {
		ids.TraceID = strings.TrimSpace(v)
	}
	return ids
}

// IntoTraceContext 将 W3C TraceContext 写入 ctx，并做规范化：
// - trace_id / span_id：小写 + TrimSpace
// - trace_flags：归一化
// - tracestate：TrimSpace
//
// 注意：这里不强制校验 trace_id/span_id 的合法性；
// 校验逻辑在 TraceContextFromContext / EnsureTraceContext 中完成。
func IntoTraceContext(ctx context.Context, tc TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	tc.TraceID = strings.ToLower(strings.TrimSpace(tc.TraceID))
	tc.SpanID = strings.ToLower(strings.TrimSpace(tc.SpanID))
	tc.TraceFlags = w3c.NormalizeTraceFlags(tc.TraceFlags)
	tc.TraceState = strings.TrimSpace(tc.TraceState)
	return context.WithValue(ctx, ctxTraceContextKey, tc)
}

// TraceContextFromContext 从 ctx 读取 TraceContext，并进行规范化 + 基础校验。
// 仅当 trace_id 合法时返回 ok=true。
// （trace_id 不合法的 TraceContext 对后续追踪没有意义，直接视为不存在）
func TraceContextFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	tc, ok := ctx.Value(ctxTraceContextKey).(TraceContext)
	if !ok {
		return TraceContext{}, false
	}

	// 规范化，避免不同来源的大小写/空白造成链路断裂
	tc.TraceID = strings.ToLower(strings.TrimSpace(tc.TraceID))
	tc.SpanID = strings.ToLower(strings.TrimSpace(tc.SpanID))
	tc.TraceFlags = w3c.NormalizeTraceFlags(tc.TraceFlags)
	tc.TraceState = strings.TrimSpace(tc.TraceState)

	// 至少要保证 TraceID 合法，否则整个 TraceContext 无意义
	if !w3c.IsValidTraceID(tc.TraceID) {
		return TraceContext{}, false
	}
	return tc, true
}

// WithIncomingParentSpanID 写入“入站父 span id”，并保证合法性：
// - 非法 span id 会被清空为 ""，避免污染上下文
func WithIncomingParentSpanID(ctx context.Context, spanID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	spanID = strings.ToLower(strings.TrimSpace(spanID))
	if !w3c.IsValidSpanID(spanID) {
		spanID = ""
	}
	return context.WithValue(ctx, ctxIncomingParentSpanKey, spanID)
}

// IncomingParentSpanIDFromContext 读取并校验入站父 span id。
// 如果不存在或非法，返回空字符串。
func IncomingParentSpanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, ok := ctx.Value(ctxIncomingParentSpanKey).(string)
	if !ok {
		return ""
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if !w3c.IsValidSpanID(v) {
		return ""
	}
	return v
}

// EnsureTraceContext 确保 ctx 中存在一个“可用的” TraceContext，并返回（ctx, tc）。
//
// 规则：
// - trace_id：优先复用 ctx 中已有 TraceContext；否则尝试复用 IDs.TraceID；再否则新生成
// - span_id：若缺失/非法则新生成（用于后续创建 span 时作为起点）
// - flags/state：做归一化
//
// 适用场景：
// - 中间件/入口：确保每个请求都有 trace context，避免链路断裂
// - 下游调用前：保证 trace_id/span_id 合法
func EnsureTraceContext(ctx context.Context) (context.Context, TraceContext) {
	if ctx == nil {
		ctx = context.Background()
	}
	ids := FromContext(ctx)

	tc, ok := TraceContextFromContext(ctx)
	if !ok {
		tc = TraceContext{}
	}

	// 确保 TraceID：优先复用已有，其次复用 IDs.TraceID，最后生成新的
	if !w3c.IsValidTraceID(tc.TraceID) {
		if w3c.IsValidTraceID(ids.TraceID) {
			tc.TraceID = ids.TraceID
		} else {
			tc.TraceID = w3c.NewTraceID()
		}
	}

	// 确保 SpanID：缺失/非法就生成新的
	if !w3c.IsValidSpanID(tc.SpanID) {
		tc.SpanID = w3c.NewSpanID()
	}

	tc.TraceFlags = w3c.NormalizeTraceFlags(tc.TraceFlags)
	ctx = IntoTraceContext(ctx, tc)
	return ctx, tc
}

// EnsureIDs 确保 request_id + trace_id 可用，并自动补齐 TraceContext。
// 返回（ctx, ids）。
//
// 规则：
// - request_id：若缺失则生成 uuid（更适合人肉排查/日志检索）
// - trace_id：若缺失则优先复用 TraceContext.TraceID，否则生成新的 W3C TraceID
// - TraceContext：确保存在且与 ids.TraceID 一致（避免出现“两套 trace_id”导致链路断裂）
//
// 典型使用：
// - HTTP/RPC 入口中间件：先解析入站 header 写入 ctx，然后调用 EnsureIDs 做兜底补齐
func EnsureIDs(ctx context.Context) (context.Context, IDs) {
	if ctx == nil {
		ctx = context.Background()
	}
	ids := FromContext(ctx)

	// 保证 request_id：空则生成
	if ids.RequestID == "" {
		ids.RequestID = uuid.NewString()
	}

	// 保证 trace_id：空则尝试从 TraceContext 复用，否则生成
	if ids.TraceID == "" {
		if tc, ok := TraceContextFromContext(ctx); ok && w3c.IsValidTraceID(tc.TraceID) {
			ids.TraceID = tc.TraceID
		} else {
			ids.TraceID = w3c.NewTraceID()
		}
	}

	// 写入 IDs，并确保 TraceContext 存在
	ctx = IntoContext(ctx, ids)
	ctx, tc := EnsureTraceContext(ctx)

	// 强制 TraceContext.TraceID 与 ids.TraceID 一致，避免上游/中间件写入不一致造成链路分叉
	if tc.TraceID != ids.TraceID {
		tc.TraceID = ids.TraceID
		ctx = IntoTraceContext(ctx, tc)
	}
	return ctx, ids
}
