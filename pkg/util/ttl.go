package util

import (
	"math/rand"
	"time"
)

const defaultCacheTTL = 10 * time.Minute

// ApplyTTLJitter 返回基础 TTL 加随机抖动后的过期时间，避免同类缓存同时失效。
func ApplyTTLJitter(base, maxJitter time.Duration) time.Duration {
	if base <= 0 {
		base = defaultCacheTTL
	}
	if maxJitter <= 0 {
		return base
	}

	// Int64n 上界为开区间，这里 +1 让抖动范围覆盖 [0, maxJitter]。
	jitter := time.Duration(rand.Int63n(int64(maxJitter) + 1))
	return base + jitter
}
