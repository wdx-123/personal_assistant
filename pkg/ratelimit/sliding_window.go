package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// luaSlidingWindow 通过 ZSET 实现滑动窗口限流。
// KEYS[1]: 限流键
// ARGV[1]: 窗口内最大请求数
// ARGV[2]: 窗口大小（毫秒）
// ARGV[3]: 当前时间戳（毫秒）
// ARGV[4]: 本次请求唯一 member
// 返回: {是否放行(0/1), 当前计数, retry_after_ms}
const luaSlidingWindow = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local member = ARGV[4]
local min_score = now_ms - window_ms

redis.call('ZREMRANGEBYSCORE', key, '-inf', min_score)

local current = redis.call('ZCARD', key)
if current >= limit then
	local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
	local retry_after_ms = 0
	if oldest[2] then
		retry_after_ms = math.max(0, tonumber(oldest[2]) + window_ms - now_ms)
	end
	redis.call('PEXPIRE', key, window_ms)
	return {0, current, retry_after_ms}
end

redis.call('ZADD', key, now_ms, member)
redis.call('PEXPIRE', key, window_ms)
return {1, current + 1, 0}
`

// SlidingWindowLimiter 基于 Redis ZSET 的滑动窗口限流器。
type SlidingWindowLimiter struct {
	rdb       *redis.Client
	keyPrefix string
	limit     int
	window    time.Duration
	now       func() time.Time
}

// NewSlidingWindowLimiter 创建滑动窗口限流器。
func NewSlidingWindowLimiter(
	rdb *redis.Client,
	keyPrefix string,
	limit int,
	window time.Duration,
) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		limit:     limit,
		window:    window,
		now:       time.Now,
	}
}

// Allow 判断指定标识在滑动窗口内是否放行。
func (l *SlidingWindowLimiter) Allow(ctx context.Context, identifier string) (*Result, error) {
	if l == nil {
		return nil, fmt.Errorf("sliding window limiter is nil")
	}
	if l.rdb == nil {
		return nil, fmt.Errorf("redis client is nil")
	}

	key := fmt.Sprintf("%s:%s", l.keyPrefix, identifier)
	windowMs := l.window.Milliseconds()
	if windowMs <= 0 {
		windowMs = int64((60 * time.Second).Milliseconds())
	}

	now := l.now()
	member := fmt.Sprintf("%d-%s", now.UnixNano(), uuid.NewString())
	vals, err := l.rdb.Eval(
		ctx,
		luaSlidingWindow,
		[]string{key},
		l.limit,
		windowMs,
		now.UnixMilli(),
		member,
	).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("sliding window rate limit eval failed: %w", err)
	}
	if len(vals) < 3 {
		return nil, fmt.Errorf("sliding window lua returned unexpected result")
	}

	allowed := vals[0] == 1
	current := vals[1]
	retryAfter := time.Duration(vals[2]) * time.Millisecond
	remaining := l.limit - int(current)
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:    allowed,
		Current:    current,
		Limit:      l.limit,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}
