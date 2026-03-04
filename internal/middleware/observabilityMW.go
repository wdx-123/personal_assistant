package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/observability/contextid"
	obsmetrics "personal_assistant/pkg/observability/metrics"
	obstrace "personal_assistant/pkg/observability/trace"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ObservabilityMiddleware 在每个 HTTP 请求结束后采集“HTTP 指标（metrics）”与“全链路 span”。
//
// 设计目标：
//   - 尽量轻量：采集发生在请求完成之后（c.Next() 之后），只做必要字段拼装与一次调用。
//   - 不影响业务：当后端未初始化（global.ObservabilityMetrics/Traces 为 nil）或写入失败时，仅记录日志，不阻断响应。
//   - 统一维度：尽量使用路由模板（FullPath）作为聚合维度，避免实际 path 导致高基数。
//   - 关联链路：从 gin.Context 中获取 request_id / trace_id（由 RequestIDMiddleware 注入），用于日志/链路回溯关联。
//
// 采集内容：
//  1. HTTP 指标（obsmetrics.HTTPRecord）
//     - 时间戳、服务名、路由模板、方法、status/statusClass、业务错误码、耗时
//     - 通常用于 QPS/错误率/平均耗时/最大耗时 等看板指标
//  2. 全链路 Span（obstrace.Span）
//     - 覆盖 root/controller 两层，Service 层由 supplier traced wrapper 自动补齐
//
// 中间件顺序建议：
//   - 建议放在 RequestIDMiddleware 之后（保证 GinKeyRequestID/GinKeyTraceID 已写入），并尽量靠后以便拿到最终 status 与 errors。
//   - 若还有统一错误处理/恢复中间件，确保它们会在 c.Errors 中写入错误信息或设置 GinKeyErrorCode。
func ObservabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录请求开始时间，用于计算本次请求耗时。
		// 注意：该 start 是 wall-clock；如需更精确统计可考虑 monotonic time（time.Since 已使用单调时钟）。
		start := time.Now()

		// request_id / trace_id：由 RequestIDMiddleware 注入 gin.Context key。
		requestID := getGinString(c, contextid.GinKeyRequestID)
		traceID := getGinString(c, contextid.GinKeyTraceID)

		// serviceName：默认值兜底为 "personal_assistant"，可通过配置覆盖，保证指标维度稳定。
		serviceName := "personal_assistant"
		if global.Config != nil {
			if v := strings.TrimSpace(global.Config.Observability.ServiceName); v != "" {
				serviceName = v
			}
		}

		// 根 span + 控制器阶段 span。后续 DB/外部HTTP/Service 会复用同一个 request.Context() 继续打点。
		reqCtx, rootSpan := obstrace.StartSpan(c.Request.Context(), obstrace.StartOptions{
			TraceID:   traceID,
			RequestID: requestID,
			Service:   serviceName,
			Stage:     "http.request",
			Name:      c.Request.Method + " " + c.Request.URL.Path,
			Kind:      "server",
		})
		reqCtx, controllerSpan := obstrace.StartSpan(reqCtx, obstrace.StartOptions{
			TraceID:   traceID,
			RequestID: requestID,
			Service:   serviceName,
			Stage:     "controller",
			Name:      buildControllerSpanName(c.Request.Method, c.FullPath(), c.Request.URL.Path),
			Kind:      "internal",
		})
		c.Request = c.Request.WithContext(reqCtx)

		requestSnippet := captureRequestSnippet(c)

		// 继续执行后续中间件与业务逻辑；此处之后开始采集“最终结果”。
		c.Next()

		// 若两类后端都未启用/未初始化，直接返回，避免做无意义的字符串处理与对象构造。
		if global.ObservabilityMetrics == nil && global.ObservabilityTraces == nil {
			return
		}

		// 获取最终 HTTP 状态码（Gin Writer 已在请求结束后确定）。
		status := c.Writer.Status()

		// 路由模板：优先使用 c.FullPath()（例如 "/v1/users/:id"），用于低基数聚合。
		// FullPath 为空的场景（例如 404 未命中路由、或某些特殊 handler）退化为实际 Path。
		routeTemplate := c.FullPath()
		if strings.TrimSpace(routeTemplate) == "" {
			routeTemplate = c.Request.URL.Path
		}

		// HTTP 方法与耗时。
		method := c.Request.Method
		latencyMs := time.Since(start).Milliseconds()

		// 业务错误码：由上游业务逻辑写入 gin.Context（GinKeyErrorCode）。
		// 若未设置则为空字符串。
		errorCode := extractErrorCode(c)

		// 1) 记录 HTTP 指标：通常以“内存聚合 + 定时落库”的方式实现，尽量不影响业务请求延迟。
		if global.ObservabilityMetrics != nil {
			err := global.ObservabilityMetrics.RecordHTTP(c.Request.Context(), &obsmetrics.HTTPRecord{
				// Timestamp 使用 start，表示请求开始时间（也可用 end 时间，需与统计口径一致）。
				Timestamp:     start,
				Service:       serviceName,
				RouteTemplate: routeTemplate,
				Method:        method,
				StatusCode:    status,
				// StatusClass 为百位数（2/3/4/5），便于聚合 2xx/4xx/5xx。
				StatusClass: status / 100,
				ErrorCode:   errorCode,
				LatencyMs:   latencyMs,
			})
			// 可观测性写入失败不应影响业务：仅记录日志。
			if err != nil {
				global.Log.Error("record http metrics failed", zap.Error(err))
			}
		}

		// 2) 记录全链路 Span：根 Span + Controller Span。
		if global.ObservabilityTraces != nil {
			message := strings.TrimSpace(c.Errors.String())
			if message == "" && status >= http.StatusBadRequest {
				message = fmt.Sprintf("http status=%d", status)
			}
			spanStatus := obstrace.SpanStatusOK
			if shouldRecordError(status, errorCode, c.Errors.String()) {
				spanStatus = obstrace.SpanStatusError
			}

			if root := rootSpan.Span(); root != nil {
				root.Name = method + " " + routeTemplate
			}
			rootFinal := rootSpan.End(spanStatus, errorCode, message, map[string]string{
				"method":         method,
				"route_template": routeTemplate,
				"status_code":    fmt.Sprintf("%d", status),
				"status_class":   fmt.Sprintf("%d", status/100),
			})

			controllerFinal := controllerSpan.End(spanStatus, errorCode, message, map[string]string{
				"method":         method,
				"route_template": routeTemplate,
				"status_code":    fmt.Sprintf("%d", status),
			})

			if spanStatus == obstrace.SpanStatusError && shouldCaptureErrorPayload() {
				rootSpan.WithErrorPayload(requestSnippet, cutPayload(message))
				controllerSpan.WithErrorPayload(requestSnippet, cutPayload(message))
			}
			if spanStatus == obstrace.SpanStatusError {
				errorDetail := buildHTTPErrorDetailJSON(routeTemplate, method, status, status/100, errorCode, c.Errors.String())
				rootSpan.WithErrorDetail(errorDetail)
				controllerSpan.WithErrorDetail(errorDetail)

				panicStack := getGinString(c, contextid.GinKeyPanicStack)
				if panicStack != "" {
					rootSpan.WithErrorStack(panicStack)
					controllerSpan.WithErrorStack(panicStack)
				}
			}

			if rootFinal != nil {
				if err := global.ObservabilityTraces.RecordSpan(c.Request.Context(), rootFinal); err != nil {
					global.Log.Error("record root trace span failed", zap.Error(err))
				}
			}
			if controllerFinal != nil {
				if err := global.ObservabilityTraces.RecordSpan(c.Request.Context(), controllerFinal); err != nil {
					global.Log.Error("record controller trace span failed", zap.Error(err))
				}
			}
		}
	}
}

func buildHTTPErrorDetailJSON(
	routeTemplate string,
	method string,
	statusCode int,
	statusClass int,
	errorCode string,
	ginErrors string,
) string {
	payload := map[string]interface{}{
		"route_template": strings.TrimSpace(routeTemplate),
		"method":         strings.TrimSpace(method),
		"status_code":    statusCode,
		"status_class":   statusClass,
		"error_code":     strings.TrimSpace(errorCode),
		"errors":         strings.TrimSpace(ginErrors),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

// shouldRecordError 决定是否需要记录错误链路事件。
// 默认口径：
//   - 存在业务错误码（ErrorCode 非空）
//   - 或 gin.Errors 非空（中间件/handler 往 c.Errors 写入了错误）
//   - 或 HTTP 状态码 >= 400
//
// 注意：
//   - 该口径会把 4xx 也计为错误事件；是否符合你们告警/看板需求需要统一定义。
//   - 若你们只关心 5xx，可将 status >= 500 作为阈值，并明确 4xx 的处理策略。
func shouldRecordError(status int, errorCode string, errMsg string) bool {
	if strings.TrimSpace(errorCode) != "" {
		return true
	}
	if strings.TrimSpace(errMsg) != "" {
		return true
	}
	return status >= 400
}

// extractErrorCode 从 gin.Context 中提取业务错误码。
// 约定：
//   - 业务侧/统一错误处理器可将错误码写入 c.Set(contextid.GinKeyErrorCode, <code>)。
//   - code 类型支持：int/int64/string；其他类型忽略。
//   - 数值型 code <= 0 视为无效；string 会 TrimSpace。
//
// 目的：
//   - 将“业务错误码”作为可观测性维度，便于按错误原因聚合与排查。
//
// 风险：
//   - 错误码高基数会导致指标维度膨胀；建议使用稳定枚举，不要拼接动态信息。
func extractErrorCode(c *gin.Context) string {
	value, exists := c.Get(contextid.GinKeyErrorCode)
	if !exists || value == nil {
		return ""
	}
	switch v := value.(type) {
	case int:
		if v <= 0 {
			return ""
		}
		return fmt.Sprintf("%d", v)
	case int64:
		if v <= 0 {
			return ""
		}
		return fmt.Sprintf("%d", v)
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

// getGinString 从 gin.Context 取 string 类型的值并 TrimSpace。
// 用于读取 RequestIDMiddleware 注入的 GinKeyRequestID/GinKeyTraceID 等字段。
// 若 key 不存在或类型不匹配则返回空字符串。
func getGinString(c *gin.Context, key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func buildControllerSpanName(method, routeTemplate, requestPath string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		method = "unknown"
	}
	route := strings.TrimSpace(routeTemplate)
	if route == "" {
		route = strings.TrimSpace(requestPath)
	}
	route = strings.TrimPrefix(route, "/")
	route = strings.ReplaceAll(route, "/", ".")
	route = strings.ReplaceAll(route, ":", "")
	route = strings.ReplaceAll(route, "-", "_")
	route = strings.Trim(route, ".")
	if route == "" {
		route = "dispatch"
	}
	return "controller." + method + "." + route
}

func shouldCaptureErrorPayload() bool {
	if global.Config == nil {
		return true
	}
	return global.Config.Observability.Traces.CaptureErrorPayload
}

func maxPayloadBytes() int {
	if global.Config == nil {
		return 4096
	}
	if global.Config.Observability.Traces.MaxPayloadBytes <= 0 {
		return 4096
	}
	return global.Config.Observability.Traces.MaxPayloadBytes
}

func cutPayload(raw string) string {
	raw = strings.TrimSpace(raw)
	maxBytes := maxPayloadBytes()
	if raw == "" || len(raw) <= maxBytes {
		return raw
	}
	return raw[:maxBytes]
}

// captureRequestSnippet 尽量无副作用地抓取请求正文片段，仅用于错误场景落 trace。
// 为避免上传等大包场景影响业务，仅在 content-length 已知且不超过阈值时读取并回填 Body。
func captureRequestSnippet(c *gin.Context) string {
	if c == nil || c.Request == nil || !shouldCaptureErrorPayload() {
		return ""
	}
	if c.Request.ContentLength <= 0 {
		return ""
	}
	maxBytes := maxPayloadBytes()
	if c.Request.ContentLength > int64(maxBytes) {
		return ""
	}
	contentType := strings.ToLower(strings.TrimSpace(c.ContentType()))
	if strings.Contains(contentType, "multipart/form-data") || strings.Contains(contentType, "octet-stream") {
		return ""
	}
	if c.Request.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	// 读完后放回去，避免影响后续绑定。
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) == 0 {
		return ""
	}
	return cutPayload(string(raw))
}
