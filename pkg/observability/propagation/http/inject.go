package httppropagation

import (
	"context"
	"strings"

	"personal_assistant/pkg/observability/contextid"
	"personal_assistant/pkg/observability/w3c"
)

// InjectOptions 控制“出站请求”注入哪些头，以及使用哪些头名。
type InjectOptions struct {
	// RequestIDHeader 指定 request_id 写入的 Header 名。
	// 为空时回退为 contextid.DefaultRequestIDHeader（默认 X-Request-ID）。
	RequestIDHeader string

	// InjectW3C 控制是否注入 W3C Trace Context 头（traceparent / tracestate）。
	// 为 false 时仅注入 request_id，不注入 trace 相关头。
	InjectW3C bool
}

// InjectHeaders 将 request_id +（可选）W3C trace 头注入到出站请求。
//
// 参数说明：
// - ctx：上游传入的 context，用于提取/生成 request_id 与 trace context。
// - setHeader：由调用方提供的“设置 Header”回调（适配 net/http、grpc-gateway、自研 client 等）。
// - opt：注入行为配置。
//
// 可靠性约束：
// - 若 setHeader 为 nil，则无法写 header，直接返回。
// - 若 ctx 为 nil，会使用 context.Background() 兜底，保证函数不 panic。
// - 若 ctx 中不存在 IDs/TraceContext，会通过 Ensure* 自动生成，确保出站请求具备可关联信息。
func InjectHeaders(ctx context.Context, setHeader func(key, value string), opt InjectOptions) {
	// 调用方未提供写 Header 的方法，则无注入意义，直接返回。
	if setHeader == nil {
		return
	}

	// ctx 允许为 nil；用 Background 兜底，避免后续调用链崩溃。
	if ctx == nil {
		ctx = context.Background()
	}

	// 优先使用调用方指定的 request_id header；为空则回退默认值。
	requestIDHeader := strings.TrimSpace(opt.RequestIDHeader)
	if requestIDHeader == "" {
		requestIDHeader = contextid.DefaultRequestIDHeader
	}

	// 确保 ctx 中存在 request_id + trace_id（缺失会自动生成）。
	// 注意：EnsureIDs 会返回“写入后的新 ctx”，用于后续 EnsureTraceContext 继续复用同一套 trace_id。
	ctx, ids := contextid.EnsureIDs(ctx)

	// 如果 request_id 可用，则写入到出站请求头，用于日志/调用链关联（更偏“人类检索”）。
	if ids.RequestID != "" {
		setHeader(requestIDHeader, ids.RequestID)
	}

	// 如未开启 W3C 注入，则到此为止（只注入 request_id）。
	if !opt.InjectW3C {
		return
	}

	// 确保 ctx 中存在一个合法的 W3C TraceContext（trace_id/span_id/flags/state）。
	// 若上游未带入 traceparent，这里会生成新的 trace_id/span_id，避免出站链路断裂。
	_, tc := contextid.EnsureTraceContext(ctx)

	// TraceID 为空表示无法形成合法 trace；不写 trace 相关 header。
	//（理论上 EnsureTraceContext 会保证 TraceID 合法，但保留防御性判断更稳健）
	if tc.TraceID == "" {
		return
	}

	// 构造标准 W3C traceparent 头。
	// 注意：traceparent 的第三段是 parent-id；实际应传“当前 span id”还是“入站 parent span id”，
	// 取决于你们的 span 生成策略。本函数假设 tc.SpanID 已由上游逻辑设置为“应透传的 span id”。
	traceparent := w3c.BuildTraceparent(tc)
	if traceparent != "" {
		// 写入 traceparent，用于下游继续沿用同一条 trace 并建立父子 span 关系。
		setHeader(w3c.HeaderTraceparent, traceparent)
	}

	// tracestate 是可选头：用于携带厂商/系统扩展状态（例如采样策略、租户信息等）。
	// 为空则不写，避免无意义 header。
	tracestate := strings.TrimSpace(tc.TraceState)
	if tracestate != "" {
		setHeader(w3c.HeaderTracestate, tracestate)
	}
}
