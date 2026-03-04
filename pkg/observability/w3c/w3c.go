package w3c

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const (
	// HeaderTraceparent / HeaderTracestate 是 W3C Trace Context 规范定义的 HTTP Header 名。
	// - traceparent: 负责在分布式系统中传递 trace_id / parent_span_id / flags
	// - tracestate: 负责携带供应商自定义的状态（可选）
	HeaderTraceparent = "traceparent"
	HeaderTracestate  = "tracestate"

	// defaultVersion 是本实现生成 traceparent 时使用的版本号。
	// W3C Trace Context v1 推荐使用 "00"。
	defaultVersion = "00"
)

// TraceContext 表示 W3C Trace Context 的核心字段集合。
// 典型来源：HTTP 入站请求头 traceparent / tracestate。
// 典型用途：
// - 用于续接上游链路（trace_id）
// - 用于创建当前服务的 span，并把 parent span 关联到上游 span_id
//
// 字段说明：
// - TraceID: 16 bytes（32 hex），全 0 不允许
// - SpanID:  8 bytes（16 hex），全 0 不允许（在 traceparent 中表示“父 span id”）
// - TraceFlags: 1 byte（2 hex），通常用于 sampled 等标志
// - TraceState: W3C tracestate（可选），通常直接透传
type TraceContext struct {
	TraceID    string
	SpanID     string
	TraceFlags string
	TraceState string
}

// ParseTraceparent 解析 traceparent 头，返回 (TraceContext, ok)。
// 该函数严格按照 W3C traceparent 的基本格式进行校验：
//   - 结构必须为 4 段，以 "-" 分隔：version-trace_id-span_id-flags
//   - version 必须是 2 位 hex，且不能为 "ff"（规范保留值）
//   - trace_id 必须为 32 位 hex 且不能全 0
//   - span_id 必须为 16 位 hex 且不能全 0
//   - flags 必须为 2 位 hex
//
// 注意：
// - 本函数不会解析 tracestate（由调用方从 HeaderTracestate 读取后自行填充 tc.TraceState）。
// - 本函数会对输入做 TrimSpace + ToLower，保证输出规范化。
func ParseTraceparent(header string) (TraceContext, bool) {
	header = strings.TrimSpace(header)
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return TraceContext{}, false
	}

	version := strings.ToLower(strings.TrimSpace(parts[0]))
	traceID := strings.ToLower(strings.TrimSpace(parts[1]))
	spanID := strings.ToLower(strings.TrimSpace(parts[2]))
	flags := strings.ToLower(strings.TrimSpace(parts[3]))

	// version=ff 为保留值，必须拒绝
	if !isHex(version, 2) || version == "ff" {
		return TraceContext{}, false
	}
	if !IsValidTraceID(traceID) {
		return TraceContext{}, false
	}
	if !IsValidSpanID(spanID) {
		return TraceContext{}, false
	}
	if !isHex(flags, 2) {
		return TraceContext{}, false
	}

	return TraceContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: flags,
	}, true
}

// BuildTraceparent 根据 TraceContext 构造 traceparent header 字符串。
// 返回值为空字符串表示 tc 不合法或无法构造（调用方应视为“不写 header”）。
//
// 行为约束：
// - TraceID/SpanID 会被规范化为小写，并且必须通过合法性校验
// - TraceFlags 会通过 NormalizeTraceFlags 归一化（非法则回退为默认值）
// - version 固定使用 defaultVersion（"00"）
//
// 备注：
//   - tc.SpanID 在入站语义里代表 parent span id；在出站透传时通常应设置为“当前 span id”。
//     具体取决于你们 tracing 实现如何生成子 span。
func BuildTraceparent(tc TraceContext) string {
	traceID := strings.ToLower(strings.TrimSpace(tc.TraceID))
	spanID := strings.ToLower(strings.TrimSpace(tc.SpanID))
	flags := NormalizeTraceFlags(tc.TraceFlags)

	if !IsValidTraceID(traceID) || !IsValidSpanID(spanID) {
		return ""
	}
	return defaultVersion + "-" + traceID + "-" + spanID + "-" + flags
}

// NewTraceID 生成一个新的 W3C trace_id：16 bytes 随机数（32 hex），且不允许全 0。
func NewTraceID() string {
	return newRandomID(16)
}

// NewSpanID 生成一个新的 W3C span_id：8 bytes 随机数（16 hex），且不允许全 0。
func NewSpanID() string {
	return newRandomID(8)
}

// IsValidTraceID 校验 trace_id：
// - 必须是 32 位小写 hex（0-9a-f）
// - 不允许全 0
//
// 调用方不需要预先 ToLower/TrimSpace，本函数内部会做规范化。
func IsValidTraceID(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return isHex(v, 32) && !isAllZero(v)
}

// IsValidSpanID 校验 span_id：
// - 必须是 16 位小写 hex（0-9a-f）
// - 不允许全 0
//
// 调用方不需要预先 ToLower/TrimSpace，本函数内部会做规范化。
func IsValidSpanID(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return isHex(v, 16) && !isAllZero(v)
}

// NormalizeTraceFlags 归一化 trace-flags。
// trace-flags 在规范中是 1 byte（2 hex）。
// - 如果 flags 非法（不是 2 位 hex），则回退到 "01"（本项目默认值）。
//
// 注意：严格来说不同系统可能更偏向默认 "00"（unsampled）。
// 本项目选择 "01" 作为默认含义，请在接入采样策略时保持一致。
func NormalizeTraceFlags(flags string) string {
	flags = strings.ToLower(strings.TrimSpace(flags))
	if !isHex(flags, 2) {
		return "01"
	}
	return flags
}

// newRandomID 生成指定字节长度的随机 ID，并以 hex 编码输出。
// - bytesLen=16 -> 32 hex（trace_id）
// - bytesLen=8  -> 16 hex（span_id）
//
// 可靠性策略：
// - 如果 rand.Read 失败则重试（极少见，通常只会发生在系统熵源异常时）
// - 如果结果全 0 则重试（W3C 规范不允许全 0）
func newRandomID(bytesLen int) string {
	buf := make([]byte, bytesLen)
	for {
		_, err := rand.Read(buf)
		if err != nil {
			continue
		}
		out := hex.EncodeToString(buf)
		if !isAllZero(out) {
			return out
		}
	}
}

// isHex 判断字符串是否为指定长度的 hex（仅允许 0-9a-f）。
// 注意：这里不接受大写 A-F；调用方应先做 ToLower。
func isHex(v string, length int) bool {
	if len(v) != length {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isAllZero 判断字符串是否全部由字符 '0' 组成。
// W3C 规范要求 trace_id/span_id 不能全 0。
func isAllZero(v string) bool {
	for i := 0; i < len(v); i++ {
		if v[i] != '0' {
			return false
		}
	}
	return true
}
