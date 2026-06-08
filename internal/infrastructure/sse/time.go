package sse

import "time"

// ticker 抽象最小定时器接口，方便在测试中替换真实 time.Ticker。
// 这样 Connection.loop 不必直接依赖标准库具体类型，降低测试控制时间流逝的成本。
type ticker interface {
	C() <-chan time.Time
	Stop()
}

// stdTicker 是对标准库 time.Ticker 的轻量适配。
// 它存在的意义是把标准库类型包一层，让业务逻辑只依赖接口。
type stdTicker struct {
	t *time.Ticker
}

// C 返回定时事件通道。
// 这里单独包一层，是为了让 Connection.loop 在测试时可替换为伪造实现。
func (s *stdTicker) C() <-chan time.Time {
	return s.t.C
}

// Stop 停止底层定时器，避免 goroutine 与计时资源泄露。
func (s *stdTicker) Stop() {
	s.t.Stop()
}

// timeTicker 创建一个标准库 ticker 适配器。
// 参数：
//   - d：触发间隔。
//
// 返回值：
//   - ticker：满足最小接口的实现。
//
// 核心流程：
//  1. 包装 `time.NewTicker` 结果并返回接口类型。
//
// 注意事项：
//   - 之所以不直接在调用点创建 `time.Ticker`，是为了给测试预留替换位点。
func timeTicker(d time.Duration) ticker {
	return &stdTicker{t: time.NewTicker(d)}
}
