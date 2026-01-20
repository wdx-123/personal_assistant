package redislock

import (
	"context"
	"errors"
	"fmt"
	"personal_assistant/global"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	// ErrLockFailed 获取锁失败错误
	ErrLockFailed = errors.New("获取锁失败")
	// ErrUnlockFailed 释放锁失败错误
	ErrUnlockFailed = errors.New("释放锁失败")
	// ErrRenewFailed 续期失败错误
	ErrRenewFailed = errors.New("续期失败")
	// ErrLockNotHeld 锁未持有错误
	ErrLockNotHeld = errors.New("锁未持有")
)

// RedisLock Redis并发锁
type RedisLock struct {
	key          string        // 锁在Redis里的名字
	value        string        // 锁的值, 随机字符串,标识谁加的锁
	expiration   time.Duration // 锁的过期时间
	rdb          *redis.Client // Redis客户端
	ctx          context.Context
	stopRenew    chan struct{} // 一个用来通知"停止续期"的信号
	renewStarted bool          // 标记自动续期是否已启动
	mutex        sync.Mutex    // 保护renewStarted状态
}

// NewRedisLock 创建Redis锁
func NewRedisLock(
	ctx context.Context,
	key string,
	expiration time.Duration,
) *RedisLock {
	if expiration < 5*time.Second {
		expiration = 5 * time.Second // 最小过期时间5秒 (给续期留足够时间)
	}

	return &RedisLock{
		key:        fmt.Sprintf("lock:%s", key),
		value:      uuid.New().String(),
		expiration: expiration,
		rdb:        global.Redis,
		ctx:        ctx,
		stopRenew:  make(chan struct{}),
	}
}

// Lock 获取锁
func (l *RedisLock) Lock() error {
	return l.LockWithRetry(3, 100*time.Millisecond)
}

// LockWithRetry 带重试的获取锁
func (l *RedisLock) LockWithRetry(
	maxRetries int,
	retryInterval time.Duration,
) error {
	for i := 0; i < maxRetries; i++ {
		if err := l.tryLock(); err == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryInterval)
		}
	}
	return ErrLockFailed
}

// tryLock 尝试获取锁
func (l *RedisLock) tryLock() error {
	// 使用SET NX EX原子操作
	success, err := l.rdb.SetNX(l.ctx, l.key, l.value, l.expiration).Result()
	if err != nil {
		return err
	}
	/*
		SET <key> <value> NX EX <seconds>（如果 expiration 是秒级）
		SET <key> <value> NX PX <milliseconds>（如果是毫秒级）
	*/

	if !success {
		return ErrLockFailed
	}

	// 获取锁成功, 启动自动续期
	if err := l.startAutoRenewIfNeeded(); err != nil {
		return err
	}

	global.Log.Debug("获取锁成功", zap.String("key", l.key))
	return nil
}

// startAutoRenewIfNeeded 启动自动续期
func (l *RedisLock) startAutoRenewIfNeeded() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if !l.renewStarted {
		l.stopRenew = make(chan struct{})
		l.renewStarted = true
		go l.autoRenew()
	}

	return nil
}

// Unlock 释放锁
func (l *RedisLock) Unlock() error {
	// 停止续期
	l.mutex.Lock()
	if l.renewStarted {
		close(l.stopRenew)
		l.renewStarted = false
	}
	l.mutex.Unlock()

	// Lua脚本保证原子性: 只有持有锁的客户端才能释放锁
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.rdb.Eval(l.ctx, luaScript, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("执行lua脚本失败: %w", err)
	}

	resultInt, ok := result.(int64)
	if !ok || resultInt == 0 {
		return ErrUnlockFailed
	}

	global.Log.Debug("释放锁成功", zap.String("key", l.key))
	return nil
}

// autoRenew 自动续期
func (l *RedisLock) autoRenew() {
	ticker := time.NewTicker(l.expiration * 2 / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := l.renew(); err != nil {
				global.Log.Error("自动续期失败", zap.String("key", l.key), zap.Error(err))
				return
			}
			global.Log.Debug("自动续期成功", zap.String("key", l.key))
		case <-l.stopRenew:
			global.Log.Debug("自动续期已停止", zap.String("key", l.key))
			return
		case <-l.ctx.Done():
			global.Log.Debug("上下文取消, 停止锁续期", zap.String("key", l.key))
			return
		}
	}
}

// renew 续期
func (l *RedisLock) renew() error {
	// Lua脚本保证原子性: 只有持有锁的客户端才能续期
	luaScript := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.rdb.Eval(l.ctx, luaScript, []string{l.key},
		l.value, l.expiration.Seconds()).Result()

	if err != nil {
		return fmt.Errorf("执行lua脚本失败: %w", err)
	}

	resultInt, ok := result.(int64)
	if !ok || resultInt == 0 {
		return ErrRenewFailed
	}

	return nil
}

// WithLock 使用锁执行函数
func WithLock(ctx context.Context, key string, expiration time.Duration, fn func() error) error {
	lock := NewRedisLock(ctx, key, expiration)

	if err := lock.Lock(); err != nil {
		return fmt.Errorf("获取锁失败: %w", err)
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			global.Log.Error("释放锁失败", zap.String("key", key), zap.Error(err))
		}
	}()

	return fn()
}
