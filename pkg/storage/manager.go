package storage

import (
	"sync"
)

// Manager 负责管理所有存储驱动的单例，提供统一的选择与切换能力。
// 线程安全，供并发请求下的读取与选择。
type Manager struct {
	mu          sync.RWMutex
	drivers     map[string]Driver
	current     string
	initialized bool
}

// 全局驱动管理器单例
var manager = &Manager{
	drivers: make(map[string]Driver),
}

// RegisterDriver 注册存储驱动实例到全局管理器。
// 由 internal/core/storage.go 在应用启动时显式调用，不使用 init() 隐式注册。
func RegisterDriver(name string, drv Driver) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.drivers[name] = drv
}

// InitAll 标记管理器已初始化完成。
// 必须在所有驱动注册完毕后调用。
func InitAll() {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.initialized {
		return
	}
	// 若未设置当前驱动，默认使用 local
	if manager.current == "" {
		manager.current = "local"
	}
	manager.initialized = true
}

// CurrentDriver 返回当前选中的驱动实例，未初始化时返回 nil
func CurrentDriver() Driver {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if !manager.initialized {
		return nil
	}
	return manager.drivers[manager.current]
}

// DriverFromName 按名称获取指定驱动实例，未找到时返回 nil
func DriverFromName(name string) Driver {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if !manager.initialized {
		return nil
	}
	return manager.drivers[name]
}

// SetCurrent 切换当前驱动为指定名称，返回是否切换成功
func SetCurrent(name string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if !manager.initialized {
		return false
	}
	if _, ok := manager.drivers[name]; !ok {
		return false
	}
	manager.current = name
	return true
}

// DriverNames 返回已注册的所有驱动名称
func DriverNames() []string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	names := make([]string, 0, len(manager.drivers))
	for k := range manager.drivers {
		names = append(names, k)
	}
	return names
}

// CurrentDriverName 返回当前选中的驱动名称
func CurrentDriverName() string {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.current
}
