package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sony/gobreaker/v2"
)

// BreakerDriver 为存储驱动添加熔断保护的装饰器。
// 当底层驱动连续失败达到阈值时，自动进入 Open 状态快速拒绝请求，
// 避免大量 goroutine 因等待超时而堆积，防止级联故障。
// Upload 和 Delete 共享同一个熔断器状态（失败均指向同一下游不可用）。
type BreakerDriver struct {
	inner   Driver
	breaker *gobreaker.CircuitBreaker[StorageObject]
}

// WrapWithBreaker 用熔断器包装驱动实例，返回一个新的 Driver。
// 本地驱动通常不需要包装（本地 I/O 不会导致级联故障），仅用于远程存储驱动。
func WrapWithBreaker(drv Driver, settings gobreaker.Settings) Driver {
	return &BreakerDriver{
		inner:   drv,
		breaker: gobreaker.NewCircuitBreaker[StorageObject](settings),
	}
}

// Name 返回原始驱动名称（熔断器对外透明）
func (d *BreakerDriver) Name() string {
	return d.inner.Name()
}

// Upload 通过熔断器执行上传操作。
// 熔断中（Open 状态）时直接返回友好错误，不调用底层驱动。
func (d *BreakerDriver) Upload(ctx context.Context, r io.Reader, filename string) (StorageObject, error) {
	obj, err := d.breaker.Execute(func() (StorageObject, error) {
		return d.inner.Upload(ctx, r, filename)
	})
	if err != nil {
		return StorageObject{}, d.wrapBreakerError(err)
	}
	return obj, nil
}

// Delete 通过熔断器执行删除操作。
// 复用上传的熔断器状态：删除失败同样说明下游不可用。
func (d *BreakerDriver) Delete(ctx context.Context, key string) error {
	_, err := d.breaker.Execute(func() (StorageObject, error) {
		return StorageObject{}, d.inner.Delete(ctx, key)
	})
	if err != nil {
		return d.wrapBreakerError(err)
	}
	return nil
}

// wrapBreakerError 将熔断器特有错误转换为业务友好的提示
func (d *BreakerDriver) wrapBreakerError(err error) error {
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("存储服务暂时不可用（熔断中），请稍后重试: %w", err)
	}
	return err
}
