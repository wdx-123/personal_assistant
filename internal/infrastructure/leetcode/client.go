package leetcode

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"personal_assistant/internal/model/config"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const (
	defaultTimeout     = 5 * time.Second
	defaultMaxBodySize = 2 << 20 // 2MB
)

// Client 封装 LeetCode 外部服务调用的客户端
// 基于 go-resty 实现，提供连接池管理、自动重试和统一的错误处理
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
// 这是生产环境初始化的推荐方式，自动从 configs.yaml 加载所有参数
func NewFromConfig(cfg config.LeetCodeCrawler, opts ...Option) (*Client, error) {
	// 参数校验与默认值处理
	if cfg.BaseURL == "" {
		return nil, errors.New("leetcode base_url is required")
	}
	if cfg.TimeoutMs <= 0 {
		cfg.TimeoutMs = 5000
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

	// 1. 创建基础 Client 结构
	c, err := NewClient(cfg.BaseURL, opts...)
	if err != nil {
		return nil, err
	}
	c.responseBodyLimit = cfg.ResponseBodyLimitBytes

	// 2. 配置 resty 客户端 (核心调优部分)
	r := c.restyClient
	r.SetTimeout(time.Duration(cfg.TimeoutMs) * time.Millisecond)

	// 3. 配置 HTTP 连接池 (Transport)
	// 高并发场景下，复用 TCP 连接至关重要
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 建连超时
			KeepAlive: 30 * time.Second, // TCP KeepAlive 探测间隔
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost, // 关键：决定了对单一目标服务的并发复用能力
		IdleConnTimeout:       time.Duration(cfg.IdleConnTimeoutSec) * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	r.SetTransport(transport)

	// 4. 配置自动重试策略
	// 仅在网络错误或特定状态码时重试，增强服务的韧性
	if cfg.RetryCount > 0 {
		r.SetRetryCount(cfg.RetryCount)
		r.SetRetryWaitTime(time.Duration(cfg.RetryWaitMs) * time.Millisecond)
		r.SetRetryMaxWaitTime(time.Duration(cfg.RetryMaxWaitMs) * time.Millisecond)
		// 添加重试条件：状态码 > 500 (服务端错误) 或 网络层错误
		r.AddRetryCondition(func(r *resty.Response, err error) bool {
			return err != nil || r.StatusCode() >= 500
		})
	}

	return c, nil
}

// NewClient 创建基础客户端实例
// 适用于测试或简单初始化场景
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
	// 规范化 Path，移除末尾斜杠
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	c := &Client{
		baseURL:           parsed,
		restyClient:       resty.New(),
		logger:            zap.NewNop(), // 默认空 Logger，防止 nil panic
		responseBodyLimit: defaultMaxBodySize,
	}
	// 应用 Option
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	// 设置 resty 的 BaseURL
	c.restyClient.SetBaseURL(c.baseURL.String())

	return c, nil
}

// PublicProfile 获取用户公开资料
// 对应接口: POST /leetcode/public_profile
func (c *Client) PublicProfile(
	ctx context.Context,
	username string,
	sleepSec float64,
) (*PublicProfileResponse, error) {
	req := publicProfileRequest{
		Username: username,
		SleepSec: sleepSec,
	}
	var out PublicProfileResponse
	if err := c.post(ctx, "/leetcode/public_profile", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SubmitStats 获取用户提交统计
// 对应接口: POST /leetcode/submit_stats
func (c *Client) SubmitStats(
	ctx context.Context,
	username string,
	sleepSec float64,
) (*SubmitStatsResponse, error) {
	req := submitStatsRequest{
		Username: username,
		SleepSec: sleepSec,
	}
	var out SubmitStatsResponse
	if err := c.post(ctx, "/leetcode/submit_stats", req, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// RecentAC 获取用户最近 AC 记录
// 对应接口: POST /leetcode/recent_ac
func (c *Client) RecentAC(
	ctx context.Context,
	username string,
	sleepSec int,
) (*RecentACResponse, error) {
	req := recentACRequest{
		Username: username,
		SleepSec: sleepSec,
	}
	var out RecentACResponse
	if err := c.post(ctx, "/leetcode/recent_ac", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// post 统一的 POST 请求处理逻辑
// 封装了请求构建、Context 传递、JSON 编解码、错误检查和日志记录
func (c *Client) post(
	ctx context.Context,
	path string,
	body any, out any,
) error {
	if c == nil || c.baseURL == nil || c.restyClient == nil {
		return errors.New("leetcode client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// 拼接完整 URL 用于日志记录 (resty 内部也会处理，但这里为了日志方便)
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: c.baseURL.Path + path}).String()

	// 构建请求
	r := c.restyClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBody(body).
		SetResult(out)

	// 如果设置了响应体限制，应用该限制 (防止内存溢出)
	// 注意：go-resty 的 SetResponseBodyLimit 方法可能随版本不同而不同，这里假设 v2.16+ 支持
	// 如果编译报错，可暂时移除此行或检查 resty 版本
	// r.SetResponseBodyLimit(c.responseBodyLimit)

	// 发起请求
	resp, err := r.Post(path) // 这里直接传 path 即可，resty 会自动拼 BaseURL
	if err != nil {
		// 网络层错误 (如超时、DNS 解析失败)
		c.logError("leetcode http request failed", err, endpoint, 0)
		return fmt.Errorf("leetcode http request failed: %w", err)
	}
	if resp == nil {
		return errors.New("leetcode empty response")
	}

	// 检查业务层错误 (非 2xx 状态码)
	if resp.IsError() {
		httpErr := &RemoteHTTPError{
			URL:        endpoint,
			Path:       path,
			StatusCode: resp.StatusCode(),
			Body:       resp.String(),
		}
		c.logError("leetcode remote http error", httpErr, endpoint, resp.StatusCode())
		return httpErr
	}
	return nil
}

// logError 统一错误日志记录
// 记录请求地址、状态码和具体的错误堆栈
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
