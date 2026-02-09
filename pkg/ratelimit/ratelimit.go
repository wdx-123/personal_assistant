/**
 * @description: 基于 Redis 固定窗口计数器的通用限流器
 *               使用 Lua 脚本保证 INCR + EXPIRE 的原子性，单次 Redis 调用完成判断
 *               适用于中低 QPS 场景（如图片上传），窗口边界的短暂突发可接受
 */
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// luaFixedWindow 固定窗口计数器 Lua 脚本（原子操作）
// KEYS[1]: 限流键
// ARGV[1]: 窗口内最大请求数
// ARGV[2]: 窗口大小（秒）
// 返回: {是否放行(0/1), 当前计数}
const luaFixedWindow = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local current = redis.call('INCR', key)
if current == 1 then
    redis.call('EXPIRE', key, window)
end
if current > limit then
    return {0, current}
end
return {1, current}
`

// Limiter 基于 Redis 固定窗口计数器的限流器
type Limiter struct {
	rdb       *redis.Client
	keyPrefix string        // Redis 键前缀，如 "ratelimit:upload:global"
	limit     int           // 窗口内最大请求数
	window    time.Duration // 窗口大小
}

// NewLimiter 创建限流器实例
// keyPrefix: Redis 键前缀（会拼接 identifier 组成完整键）
// limit: 窗口内最大请求数
// window: 窗口大小
func NewLimiter(
	rdb *redis.Client,
	keyPrefix string,
	limit int,
	window time.Duration,
) *Limiter {
	return &Limiter{
		rdb:       rdb,
		keyPrefix: keyPrefix,
		limit:     limit,
		window:    window,
	}
}

// Result 限流判断结果
type Result struct {
	Allowed   bool  // 是否放行
	Current   int64 // 当前窗口内已请求次数
	Limit     int   // 窗口内最大请求数
	Remaining int   // 剩余可用次数
}

// Allow 判断指定标识的请求是否放行
// identifier: 限流维度标识（如 "global"、用户ID 等）
func (l *Limiter) Allow(ctx context.Context, identifier string) (*Result, error) {
	key := fmt.Sprintf("%s:%s", l.keyPrefix, identifier)
	windowSec := int(l.window.Seconds())
	if windowSec <= 0 {
		windowSec = 60 // 兜底 60 秒
	}

	vals, err := l.rdb.Eval(ctx, luaFixedWindow, []string{key}, l.limit, windowSec).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limit eval failed: %w", err)
	}
	if len(vals) < 2 {
		return nil, fmt.Errorf("rate limit lua returned unexpected result")
	}

	allowed := vals[0] == 1
	current := vals[1]
	remaining := l.limit - int(current)
	if remaining < 0 {
		remaining = 0
	}

	return &Result{
		Allowed:   allowed,
		Current:   current,
		Limit:     l.limit,
		Remaining: remaining,
	}, nil
}
