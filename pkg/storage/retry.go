package storage

import (
	"context"
	"math/rand"
	"time"
)

// DoWithBackoff 使用指数退避和抖动执行函数 fn。
//   - maxRetries: 最大重试次数（0 表示只尝试一次）
//   - baseDelay: 第一次重试前的初始延迟
//   - jitter: 添加到延迟中的随机抖动
//
// 该函数会尊重上下文的取消信号，在 Context 被取消时立即返回。
func DoWithBackoff(
	ctx context.Context,
	maxRetries int,
	baseDelay,
	jitter time.Duration,
	fn func() error,
) error {
	var err error
	delay := baseDelay
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			j := time.Duration(rand.Int63n(int64(jitter)))
			select {
			case <-time.After(delay + j):
			case <-ctx.Done():
				return ctx.Err()
			}
			delay *= 2 // 指数增长
		}
		if err = fn(); err == nil {
			return nil
		}
	}
	return err
}
