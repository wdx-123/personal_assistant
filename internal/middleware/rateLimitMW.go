/**
 * @description: 上传接口限流中间件
 *               支持双层限流：全局 QPS 限制 + 用户级频率限制
 *               需在 JWT 中间件之后使用（依赖 claims 提取 userID）
 */
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/ratelimit"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// UploadRateLimitMiddleware 上传接口限流中间件
// 执行顺序：先检查全局限流 → 再检查用户级限流
// globalLimiter: 全局限流器（保护整个上传接口 QPS）
// userLimiter: 用户级限流器（防单用户滥用）
func UploadRateLimitMiddleware(
	globalLimiter, userLimiter *ratelimit.Limiter,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// 1. 全局限流检查
		globalResult, err := globalLimiter.Allow(ctx, "global")
		if err != nil {
			// Redis 异常时放行（降级策略：限流器故障不应阻断正常业务）
			global.Log.Warn("全局限流器异常，降级放行", zap.Error(err))
			c.Next()
			return
		}
		if !globalResult.Allowed {
			setRateLimitHeaders(c, globalResult)
			global.Log.Warn("全局上传限流触发",
				zap.Int64("current", globalResult.Current),
				zap.Int("limit", globalResult.Limit))
			response.BizResultWithStatus(
				http.StatusTooManyRequests,
				errors.CodeTooManyRequests,
				nil,
				"系统繁忙，请稍后再试",
				c,
			)
			c.Abort()
			return
		}

		// 2. 用户级限流检查（复用 pkg/jwt 统一提取 userID）
		userID := jwt.GetUserID(c)
		if userID > 0 {
			identifier := fmt.Sprintf("%d", userID)
			userResult, err := userLimiter.Allow(ctx, identifier)
			if err != nil {
				global.Log.Warn("用户限流器异常，降级放行", zap.Uint("userID", userID), zap.Error(err))
				c.Next()
				return
			}
			if !userResult.Allowed {
				setRateLimitHeaders(c, userResult)
				global.Log.Warn("用户上传限流触发",
					zap.Uint("userID", userID),
					zap.Int64("current", userResult.Current),
					zap.Int("limit", userResult.Limit))
				response.BizResultWithStatus(
					http.StatusTooManyRequests,
					errors.CodeTooManyRequests,
					nil,
					"上传过于频繁，请稍后再试",
					c,
				)
				c.Abort()
				return
			}
			// 设置用户级剩余次数响应头
			setRateLimitHeaders(c, userResult)
		}

		c.Next()
	}
}

// OJBindRateLimitMiddleware OJ 绑定接口限流中间件。
// 仅对蓝桥、力扣、洛谷三个绑定动作生效，非法 JSON 或不支持的平台直接透传给后续 controller 校验。
func OJBindRateLimitMiddleware(
	limiters map[string]*ratelimit.SlidingWindowLimiter,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		platform, ok := resolveOJBindPlatform(c)
		if !ok {
			c.Next()
			return
		}

		limiter := limiters[platform]
		if limiter == nil {
			c.Next()
			return
		}

		result, err := limiter.Allow(c.Request.Context(), "global")
		if err != nil {
			global.Log.Warn("OJ 绑定限流器异常，降级放行",
				zap.String("platform", platform),
				zap.Error(err))
			c.Next()
			return
		}

		setRateLimitHeaders(c, result)
		if result.Allowed {
			c.Next()
			return
		}

		global.Log.Warn("OJ 绑定限流触发",
			zap.String("platform", platform),
			zap.Int64("current", result.Current),
			zap.Int("limit", result.Limit),
			zap.Duration("retry_after", result.RetryAfter))
		response.BizResultWithStatus(
			http.StatusTooManyRequests,
			errors.CodeTooManyRequests,
			nil,
			"当前绑定人数过多，请稍后再试。",
			c,
		)
		c.Abort()
	}
}

// setRateLimitHeaders 设置限流相关响应头，便于客户端感知
func setRateLimitHeaders(c *gin.Context, result *ratelimit.Result) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	if !result.Allowed {
		c.Header("Retry-After", retryAfterHeaderValue(result.RetryAfter))
	}
}

func retryAfterHeaderValue(retryAfter time.Duration) string {
	if retryAfter <= 0 {
		return "1"
	}
	seconds := int((retryAfter + time.Second - 1) / time.Second)
	if seconds <= 0 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func resolveOJBindPlatform(c *gin.Context) (string, bool) {
	if c == nil || c.Request == nil {
		return "", false
	}

	routePath := strings.TrimSpace(c.FullPath())
	requestPath := strings.TrimSpace(c.Request.URL.Path)
	body, err := snapshotRequestBody(c)
	if err != nil {
		return "", false
	}
	if strings.HasSuffix(routePath, "/lanqiao/bind") || strings.HasSuffix(requestPath, "/lanqiao/bind") {
		if len(body) == 0 || !json.Valid(body) {
			return "", false
		}
		return "lanqiao", true
	}
	if !(strings.HasSuffix(routePath, "/bind") || strings.HasSuffix(requestPath, "/bind")) {
		return "", false
	}
	if len(body) == 0 {
		return "", false
	}

	var payload struct {
		Platform string `json:"platform"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}

	platform := strings.ToLower(strings.TrimSpace(payload.Platform))
	switch platform {
	case "leetcode", "luogu":
		return platform, true
	default:
		return "", false
	}
}

func snapshotRequestBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
