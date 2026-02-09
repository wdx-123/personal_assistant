package core

import (
	"time"

	"personal_assistant/global"
	"personal_assistant/pkg/storage"
	"personal_assistant/pkg/storage/local"
	"personal_assistant/pkg/storage/qiniu"

	"github.com/sony/gobreaker/v2"
	"go.uber.org/zap"
)

// InitStorage 初始化存储驱动，注册本地与七牛驱动并设置当前驱动。
// 七牛驱动自动包装熔断器：连续失败 5 次后进入 Open 状态，快速拒绝请求，15s 后尝试恢复。
// 本地驱动不加熔断——本地 I/O 故障不会导致 goroutine 堆积。
// 调用时机：在配置和日志初始化之后、Server 启动之前。
func InitStorage() {
	// 本地驱动：不需要熔断器
	localDrv := local.New()

	// 七牛驱动：包装熔断器保护
	qiniuDrv := storage.WrapWithBreaker(qiniu.New(), gobreaker.Settings{
		Name:        "qiniu-storage",
		MaxRequests: 3,                // Half-Open 状态最多放 3 个探测请求
		Interval:    30 * time.Second, // Closed 状态下每 30s 重置失败计数
		Timeout:     15 * time.Second, // Open → Half-Open 的等待时间
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// 连续失败 5 次触发熔断
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			global.Log.Warn("存储驱动熔断器状态变更",
				zap.String("breaker", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
		},
	})

	// 注册所有可用驱动
	storage.RegisterDriver("local", localDrv)
	storage.RegisterDriver("qiniu", qiniuDrv)
	storage.InitAll()

	// 根据配置选择当前驱动
	current := global.Config.Storage.Current
	if current != "" {
		if !storage.SetCurrent(current) {
			global.Log.Warn("存储驱动切换失败，驱动不存在，回退为 local",
				zap.String("requested", current),
				zap.Strings("available", storage.DriverNames()))
		}
	}

	global.Log.Info("存储驱动初始化完成",
		zap.String("current", storage.CurrentDriverName()),
		zap.Strings("registered", storage.DriverNames()))
}
