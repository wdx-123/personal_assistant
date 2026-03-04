package luogu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/config"
	obsprophttp "personal_assistant/pkg/observability/propagation/http"
	obstrace "personal_assistant/pkg/observability/trace"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const (
	defaultTimeout     = 8 * time.Second
	defaultMaxBodySize = 4 << 20 // 4MB
)

// Client 封装 Luogu 外部服务调用的客户端
// 架构设计与 LeetCode 客户端保持一致
type Client struct {
	baseURL           *url.URL
	restyClient       *resty.Client
	logger            *zap.Logger
	responseBodyLimit int64
}

// Option 定义客户端的可选配置函数
type Option func(*Client)

// WithLogger 配置自定义 Logger
func WithLogger(logger *zap.Logger) Option {
	return func(c *Client) {
		c.logger = logger
	}
}

// NewFromConfig 基于全局配置创建客户端实例
func NewFromConfig(cfg config.LuoguCrawler, opts ...Option) (*Client, error) {
	// 参数校验与默认值处理
	if cfg.BaseURL == "" {
		return nil, errors.New("luogu base_url is required")
	}
	if cfg.TimeoutMs <= 0 {
		cfg.TimeoutMs = 8000
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 100
	}
	if cfg.ResponseBodyLimitBytes <= 0 {
		cfg.ResponseBodyLimitBytes = defaultMaxBodySize
	}

	// 1. 创建基础 Client
	c, err := NewClient(cfg.BaseURL, opts...)
	if err != nil {
		return nil, err
	}
	c.responseBodyLimit = cfg.ResponseBodyLimitBytes

	// 2. 配置 resty
	r := c.restyClient
	r.SetTimeout(time.Duration(cfg.TimeoutMs) * time.Millisecond)

	// 3. 配置 HTTP 连接池
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(cfg.IdleConnTimeoutSec) * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	r.SetTransport(transport)

	// 4. 配置自动重试
	if cfg.RetryCount > 0 {
		r.SetRetryCount(cfg.RetryCount)
		r.SetRetryWaitTime(time.Duration(cfg.RetryWaitMs) * time.Millisecond)
		r.SetRetryMaxWaitTime(time.Duration(cfg.RetryMaxWaitMs) * time.Millisecond)
		r.AddRetryCondition(func(r *resty.Response, err error) bool {
			return err != nil || r.StatusCode() >= 500
		})
	}

	return c, nil
}

// NewClient 创建基础客户端实例
func NewClient(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, errors.New("baseURL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse baseURL failed: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid baseURL: %s", baseURL)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	c := &Client{
		baseURL:           parsed,
		restyClient:       resty.New(),
		logger:            zap.NewNop(),
		responseBodyLimit: defaultMaxBodySize,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.restyClient.SetBaseURL(c.baseURL.String())

	return c, nil
}

// GetPractice 获取用户练习记录
// 对应接口: POST /luogu/practice
func (c *Client) GetPractice(
	ctx context.Context,
	uid int, sleepSec float64,
) (*GetPracticeResponse, error) {
	req := getPracticeRequest{
		UID:      uid,
		SleepSec: sleepSec,
	}
	var out GetPracticeResponse
	if err := c.post(ctx, "/luogu/practice", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// post 统一 POST 请求
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	if c == nil || c.baseURL == nil || c.restyClient == nil {
		return errors.New("luogu client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	spanCtx, spanEvent := obstrace.StartSpan(ctx, obstrace.StartOptions{
		Service: traceServiceName(),
		Stage:   "outbound.http",
		Name:    "luogu" + path,
		Kind:    "client",
		Tags: map[string]string{
			"provider": "luogu",
			"path":     path,
		},
	})
	ctx = spanCtx
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: c.baseURL.Path + path}).String()

	r := c.restyClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
		SetResult(out)

	obsprophttp.InjectHeaders(ctx, func(key, value string) {
		r.SetHeader(key, value)
	}, outboundInjectOptions())

	// r.SetResponseBodyLimit(c.responseBodyLimit) // 根据 resty 版本按需开启

	resp, err := r.Post(path)
	if err != nil {
		c.logError("luogu http request failed", err, endpoint, 0)
		if spanEvent != nil && global.ObservabilityTraces != nil {
			spanEvent.WithErrorPayload(marshalTraceSnippet(body), cutTracePayload(err.Error()))
			spanEvent.WithErrorDetail(buildOutboundErrorDetail(path, endpoint, 0, body, "", err.Error()))
			span := spanEvent.End(obstrace.SpanStatusError, "outbound_http_error", err.Error(), map[string]string{
				"endpoint": endpoint,
				"path":     path,
			})
			_ = global.ObservabilityTraces.RecordSpan(ctx, span)
		}
		return fmt.Errorf("luogu http request failed: %w", err)
	}
	if resp == nil {
		if spanEvent != nil && global.ObservabilityTraces != nil {
			spanEvent.WithErrorDetail(buildOutboundErrorDetail(path, endpoint, 0, body, "", "luogu empty response"))
			span := spanEvent.End(obstrace.SpanStatusError, "outbound_empty_response", "luogu empty response", map[string]string{
				"endpoint": endpoint,
				"path":     path,
			})
			_ = global.ObservabilityTraces.RecordSpan(ctx, span)
		}
		return errors.New("luogu empty response")
	}
	if resp.IsError() {
		httpErr := &RemoteHTTPError{
			URL:        endpoint,
			Path:       path,
			StatusCode: resp.StatusCode(),
			Body:       resp.String(),
		}

		// 尝试解析结构化错误
		var errResp struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(resp.Body(), &errResp); jsonErr == nil && errResp.Error != "" {
			httpErr.Message = errResp.Error
		}

		c.logError("luogu remote http error", httpErr, endpoint, resp.StatusCode())
		if spanEvent != nil && global.ObservabilityTraces != nil {
			spanEvent.WithErrorPayload(marshalTraceSnippet(body), cutTracePayload(resp.String()))
			spanEvent.WithErrorDetail(buildOutboundErrorDetail(path, endpoint, resp.StatusCode(), body, resp.String(), httpErr.Error()))
			span := spanEvent.End(obstrace.SpanStatusError, "outbound_http_status_error", httpErr.Error(), map[string]string{
				"endpoint":    endpoint,
				"path":        path,
				"status_code": fmt.Sprintf("%d", resp.StatusCode()),
			})
			_ = global.ObservabilityTraces.RecordSpan(ctx, span)
		}
		return httpErr
	}
	if spanEvent != nil && global.ObservabilityTraces != nil {
		span := spanEvent.End(obstrace.SpanStatusOK, "", "", map[string]string{
			"endpoint":    endpoint,
			"path":        path,
			"status_code": fmt.Sprintf("%d", resp.StatusCode()),
		})
		_ = global.ObservabilityTraces.RecordSpan(ctx, span)
	}
	return nil
}

func marshalTraceSnippet(body any) string {
	if body == nil {
		return ""
	}
	data, err := json.Marshal(body)
	if err != nil {
		return ""
	}
	return cutTracePayload(string(data))
}

func cutTracePayload(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	maxBytes := 4096
	if global.Config != nil && global.Config.Observability.Traces.MaxPayloadBytes > 0 {
		maxBytes = global.Config.Observability.Traces.MaxPayloadBytes
	}
	if len(raw) <= maxBytes {
		return raw
	}
	return raw[:maxBytes]
}

func buildOutboundErrorDetail(
	path string,
	endpoint string,
	statusCode int,
	requestBody any,
	responseBody string,
	errMsg string,
) string {
	payload := map[string]interface{}{
		"provider":       "luogu",
		"path":           strings.TrimSpace(path),
		"endpoint":       strings.TrimSpace(endpoint),
		"status_code":    statusCode,
		"error":          strings.TrimSpace(errMsg),
		"request":        marshalTraceSnippet(requestBody),
		"response":       cutTracePayload(responseBody),
		"occurred_stage": "outbound.http",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func traceServiceName() string {
	if global.Config == nil {
		return "personal_assistant"
	}
	if v := strings.TrimSpace(global.Config.Observability.ServiceName); v != "" {
		return v
	}
	return "personal_assistant"
}

func outboundInjectOptions() obsprophttp.InjectOptions {
	opt := obsprophttp.InjectOptions{
		InjectW3C: true,
	}
	if global.Config == nil {
		return opt
	}
	opt.RequestIDHeader = strings.TrimSpace(global.Config.Observability.Propagation.RequestIDHeader)
	opt.InjectW3C = global.Config.Observability.Propagation.Enabled &&
		global.Config.Observability.Propagation.InjectW3C
	return opt
}

func (c *Client) logError(msg string, err error, endpoint string, statusCode int) {
	if c.logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("endpoint", endpoint),
	}
	if statusCode != 0 {
		fields = append(fields, zap.Int("status_code", statusCode))
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	c.logger.Error(msg, fields...)
}
