package inits

import (
	"context"
)

// InitSubscribers 初始化所有事件订阅器
// 目前仅保留空实现，等待后续业务模块接入
func InitSubscribers(ctx context.Context) error {
	// TODO: 在这里初始化各个模块的订阅者
	// 例如:
	// if err := initOrderSubscriber(ctx); err != nil { return err }
	return nil
}
