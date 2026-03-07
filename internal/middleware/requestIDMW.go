package middleware

import (
	"strings"

	"personal_assistant/global"
	"personal_assistant/pkg/observability/contextid"
	"personal_assistant/pkg/observability/w3c"

	"github.com/gin-gonic/gin"
)

// propagationConfig 链路传播配置结构体
// 用于控制是否开启 ID 传播、自定义 Request-ID 头部名称、以及 W3C 标准支持
type propagationConfig struct {
	Enabled         bool   // 总开关：是否启用链路传播
	RequestIDHeader string // Request ID 的 Header 名称（默认 X-Request-ID）
	ParseW3C        bool   // 是否解析传入的 W3C Trace Context 头部 (traceparent)
	InjectW3C       bool   // 是否向响应注入 W3C Trace Context 头部
}

// RequestIDMiddleware 链路追踪上下文注入中间件
//
// 职责：
//  1. 初始化/透传 RequestID：从 Header 读取或生成唯一请求 ID。
//  2. 解析 W3C Trace Context：支持分布式链路追踪标准 (traceparent/tracestate)。
//  3. 上下文注入：将 ID 注入到 Go Context 和 Gin Context 中，供后续链路使用。
//  4. 响应回写：将 RequestID 和 TraceID 写回 Response Header，方便客户端排查。
//
// 建议位置：
//   - 应作为首个或极早期的中间件执行，确保后续所有中间件（如 Logger、Recovery、Observability）都能获取到 ID。
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取当前传播配置（支持热加载或动态配置，取决于 global.Config 实现）
		cfg := resolvePropagationConfig()

		// 2. 尝试从当前 Context 恢复 ID（通常此时为空），并从 Header 提取 RequestID
		ctx := c.Request.Context()
		ids := contextid.FromContext(ctx)
		if ids.RequestID == "" {
			// 优先使用配置的 Header 名称（如 X-Request-ID）获取，去除首尾空格
			ids.RequestID = strings.TrimSpace(c.GetHeader(cfg.RequestIDHeader))
		}

		// 3. W3C Trace Context 解析逻辑
		// 仅在配置启用且 ParseW3C 为 true 时执行
		if cfg.Enabled && cfg.ParseW3C {
			// 读取 W3C 标准头部 traceparent (version-traceid-parentid-flags)
			traceparent := strings.TrimSpace(c.GetHeader(w3c.HeaderTraceparent))
			if parsed, ok := w3c.ParseTraceparent(traceparent); ok {
				// 解析成功：提取 TraceID 并保留 TraceState
				parsed.TraceState = strings.TrimSpace(c.GetHeader(w3c.HeaderTracestate))
				ids.TraceID = parsed.TraceID

				// 将解析出的 Trace Context 注入 ctx
				ctx = contextid.IntoTraceContext(ctx, contextid.TraceContext(parsed))
				// 记录上游传来的 ParentSpanID（即 traceparent 中的 parent-id 部分）
				// 这对于构建父子 Span 关系至关重要
				ctx = contextid.WithIncomingParentSpanID(ctx, parsed.SpanID)
			} else {
				// 解析失败或无 Header：标记无父 Span
				ctx = contextid.WithIncomingParentSpanID(ctx, "")
			}
		} else {
			// 未启用 W3C 解析：标记无父 Span
			ctx = contextid.WithIncomingParentSpanID(ctx, "")
		}

		// 4. ID 兜底与生成
		// 将提取到的 ID 放入 ctx，并调用 EnsureIDs 确保 RequestID/TraceID 均有值
		// 如果 Header 里没有传，这里会自动生成新的 UUID/TraceID
		ctx = contextid.IntoContext(ctx, ids)
		ctx, ensured := contextid.EnsureIDs(ctx)
		// 确保 Trace Context 结构完整
		ctx, traceCtx := contextid.EnsureTraceContext(ctx)

		// 5. 上下文回填
		// 将更新后的 Context 赋值回 Request，后续处理链（Controller/Service）均使用此 Context
		c.Request = c.Request.WithContext(ctx)

		// 同时写入 Gin Context，方便在 Controller 中通过 c.GetString 快速获取
		c.Set(contextid.GinKeyRequestID, ensured.RequestID)
		c.Set(contextid.GinKeyTraceID, ensured.TraceID)

		// 6. 响应头注入
		// 总是返回 Request-ID，方便客户端根据此 ID 向服务端查询日志
		c.Header(cfg.RequestIDHeader, ensured.RequestID)

		// 如果启用了 W3C 注入，将当前的 Trace Context 格式化后写回 Response Header
		// 这允许客户端（如前端 APM 探针）关联后端链路
		if cfg.Enabled && cfg.InjectW3C {
			if traceparent := w3c.BuildTraceparent(traceCtx); traceparent != "" {
				// 存入 Gin Context 供后续中间件（如日志）使用
				c.Set(contextid.GinKeyTraceparent, traceparent)
				// 写入 HTTP 响应头
				c.Header(w3c.HeaderTraceparent, traceparent)
			}
			if tracestate := strings.TrimSpace(traceCtx.TraceState); tracestate != "" {
				c.Set(contextid.GinKeyTracestate, tracestate)
				c.Header(w3c.HeaderTracestate, tracestate)
			}
		}

		// 7. 放行，执行后续业务逻辑
		c.Next()
	}
}

// resolvePropagationConfig 从全局配置解析传播策略
// 提供安全的默认值兜底，防止配置缺失导致 panic 或行为异常
func resolvePropagationConfig() propagationConfig {
	// 默认配置：开启所有特性，使用标准 Request-ID Header
	cfg := propagationConfig{
		Enabled:         true,
		RequestIDHeader: contextid.DefaultRequestIDHeader,
		ParseW3C:        true,
		InjectW3C:       true,
	}

	// 如果全局配置未加载，直接返回默认值
	if global.Config == nil {
		return cfg
	}

	// 从 yaml 配置覆盖默认值
	p := global.Config.Observability.Propagation
	cfg.Enabled = p.Enabled

	// 允许自定义 Request-ID Header（例如 "X-Correlation-ID"）
	if v := strings.TrimSpace(p.RequestIDHeader); v != "" {
		cfg.RequestIDHeader = v
	}
	cfg.ParseW3C = p.ParseW3C
	cfg.InjectW3C = p.InjectW3C

	// 如果总开关关闭，强制关闭 W3C 相关功能
	if !cfg.Enabled {
		cfg.ParseW3C = false
		cfg.InjectW3C = false
	}
	return cfg
}
