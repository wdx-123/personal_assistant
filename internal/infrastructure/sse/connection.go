package sse

import (
	"context"
	"sync"
)

// Connection 表示一条已经通过鉴权并准备进入 Broker 管理的流连接。
// 它内部通过独立 goroutine 负责心跳与事件写出，从而把慢客户端与上游广播逻辑解耦。
type Connection struct {
	// 身份和归属信息
	ID        string
	Principal *Principal
	Channel   string

	// 运行依赖
	writer StreamWriter
	policy ConnectionPolicy
	queue  chan *StreamEvent

	// 生命周期控制
	ctx    context.Context
	cancel context.CancelFunc
	closed chan struct{}

	// 关闭状态与回调
	mu          sync.RWMutex
	closeReason string
	closeOnce   sync.Once
	onClose     func(*Connection)
}

// NewConnection 创建一条带独立队列和取消能力的连接对象。
// 参数：
//   - parent：上游生命周期上下文；为空时会自动回退到 Background，避免后续调用 panic。
//   - id：连接唯一标识。
//   - principal：连接所属主体快照。
//   - channel：订阅频道。
//   - writer：真正负责把事件写到客户端的输出器。
//   - policy：连接策略，决定队列长度、心跳等行为。
//
// 返回值：
//   - *Connection：尚未启动事件循环的连接对象。
//
// 核心流程：
//  1. 兜底 parent，确保任何来源都能安全构造连接。
//  2. 归一化策略，统一默认值。
//  3. 派生可取消上下文与固定容量队列。
//
// 注意事项：
//   - 这里只创建对象，不启动 goroutine；启动时机交给 Broker 控制，避免未注册就开始收发事件。
func NewConnection(
	parent context.Context,
	id string,
	principal *Principal,
	channel string,
	writer StreamWriter,
	policy ConnectionPolicy,
) *Connection {
	if parent == nil {
		parent = context.Background()
	}

	policy = policy.Normalize()
	ctx, cancel := context.WithCancel(parent)
	return &Connection{
		ID:        id,
		Principal: principal,
		Channel:   channel,
		writer:    writer,
		policy:    policy,
		queue:     make(chan *StreamEvent, policy.QueueCapacity),
		ctx:       ctx,
		cancel:    cancel,
		closed:    make(chan struct{}),
	}
}

// Start 启动连接内部的事件循环 goroutine。
// 参数：无。
// 返回值：无。
// 核心流程：
//  1. 异步进入 loop，开始消费队列和发送心跳。
//
// 注意事项：
//   - 这里不做幂等保护；根据上下文推测，调用方约定每个连接只会在注册成功后启动一次。
func (c *Connection) Start() {
	go c.loop()
}

// Done 返回连接关闭通知通道。
// 参数：无。
// 返回值：
//   - <-chan struct{}：连接彻底关闭时会被关闭的信号通道。
//
// 核心流程：
//  1. 直接暴露只读通道给外部等待。
//
// 注意事项：
//   - 统一使用关闭 channel 表示结束，比发送单次值更适合多个等待者同时监听。
func (c *Connection) Done() <-chan struct{} {
	return c.closed
}

// Enqueue 尝试把事件放入连接的待发送队列。
// 参数：
//   - evt：待发送事件。
//
// 返回值：
//   - bool：true 表示成功入队；false 表示连接已关闭或队列已满。
//
// 核心流程：
//  1. 先快速检查连接是否已经关闭，避免向已结束连接继续写入。
//  2. 再进行非阻塞发送，保证慢消费者不会反向阻塞 Broker 广播线程。
//
// 注意事项：
//   - 非阻塞入队是保护整体吞吐的关键；队列满时由 Broker 决定是否踢线。
func (c *Connection) Enqueue(evt *StreamEvent) bool {
	select {
	case <-c.closed:
		return false
	default:
	}

	select {
	case c.queue <- evt:
		return true
	default:
		return false
	}
}

// Close 负责以幂等方式关闭连接并记录关闭原因。
// 参数：
//   - reason：关闭原因，用于观测和排障。
//
// 返回值：无。
// 核心流程：
//  1. 通过 sync.Once 保证只执行一次真正关闭。
//  2. 记录原因并取消上下文，唤醒 loop 退出。
//  3. 关闭 done 通道并通知 Broker 清理索引。
//
// 注意事项：
//   - 关闭顺序先 cancel 再 close(closed)，是为了让 loop 中依赖 ctx 的写操作尽快感知退出。
func (c *Connection) Close(reason string) {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closeReason = reason
		c.mu.Unlock()

		c.cancel()
		close(c.closed)

		// 通过回调把索引清理收口到 Broker，避免 Connection 自己依赖 Broker 具体实现。
		if c.onClose != nil {
			c.onClose(c)
		}
	})
}

// CloseReason 返回连接最近一次关闭时记录的原因。
// 参数：无。
// 返回值：
//   - string：关闭原因；未关闭时可能为空字符串。
//
// 核心流程：
//  1. 通过读锁读取共享字段，避免与 Close 并发写产生数据竞争。
//
// 注意事项：
//   - 返回空串不一定表示异常，也可能只是连接仍处于活动状态。
func (c *Connection) CloseReason() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.closeReason
}

// loop 负责持续消费事件队列并按策略发送心跳。
// 参数：无。
// 返回值：无。
// 核心流程：
//  1. 先校验 writer，缺失时立即关闭连接，避免 goroutine 空转。
//  2. 启动心跳 ticker，定期向客户端发送 keepalive 防止中间链路超时。
//  3. 在同一个 select 中统一处理上下文取消、事件发送和心跳发送。
//
// 注意事项：
//   - 任何一次写事件或写心跳失败都会关闭连接，因为这通常意味着底层 HTTP 流已不可用。
func (c *Connection) loop() {
	if c.writer == nil {
		c.Close("writer_missing")
		return
	}

	ticker := timeTicker(c.policy.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		// 上下文结束说明上游已要求断开连接，此时统一收口关闭原因。
		case <-c.ctx.Done():
			c.Close("context_done")
			return

		// 事件队列中的消息优先走正式写出路径；空事件直接跳过，避免无意义写操作。
		case evt := <-c.queue:
			if evt == nil {
				continue
			}
			if err := c.writer.WriteEvent(c.ctx, evt); err != nil {
				c.Close("write_failed")
				return
			}

		// 心跳用于维持长连接活性；失败通常说明客户端已断开或代理不再接受写入。
		case <-ticker.C():
			if err := c.writer.WriteHeartbeat(c.ctx); err != nil {
				c.Close("heartbeat_failed")
				return
			}
		}
	}
}
