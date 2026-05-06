package sse

import (
	"errors"
	"sync"
	"sync/atomic"
)

// ErrBrokerDraining 表示 Broker 已进入排空阶段，不再接受新连接。
// 停机时显式返回该错误，比继续注册后又立刻断开更利于调用方判断系统状态。
var ErrBrokerDraining = errors.New("sse broker is draining")

// Broker 负责维护本进程内所有 SSE 连接的注册表和分发索引。
// 它同时按连接 ID、主体 ID 和频道维持多组索引，以便快速完成广播、踢线和统计。
type Broker struct {
	policy ConnectionPolicy

	mu        sync.RWMutex
	conns     map[string]*Connection
	bySubject map[uint64]map[string]*Connection
	byChannel map[string]map[string]*Connection

	draining             atomic.Bool
	droppedSlowConsumers atomic.Int64
}

// NewBroker 创建一个带连接策略的 Broker。
// 参数：
//   - policy：连接和分发策略；会在这里统一归一化，避免调用方重复处理默认值。
//
// 返回值：
//   - *Broker：可直接用于注册和分发连接的实例。
//
// 核心流程：
//  1. 归一化策略，确保队列容量、心跳和连接上限都有稳定默认值。
//  2. 初始化多组索引 map，为后续 O(1) 级别查找做准备。
//
// 注意事项：
//   - 策略归一化放在构造函数里，是为了保证 Broker 内部始终只面对一套确定配置。
func NewBroker(policy ConnectionPolicy) *Broker {
	return &Broker{
		policy:    policy.Normalize(),
		conns:     make(map[string]*Connection),
		bySubject: make(map[uint64]map[string]*Connection),
		byChannel: make(map[string]map[string]*Connection),
	}
}

// Register 负责把连接纳入 Broker 管理并启动其事件循环。
// 参数：
//   - conn：待注册连接。
//
// 返回值：
//   - error：连接为空、Broker 排空中，或主体连接数超限时返回错误。
//
// 核心流程：
//  1. 先做无锁快速失败，减少排空阶段的锁竞争。
//  2. 加写锁后二次确认状态，避免在竞争窗口内接入新连接。
//  3. 更新总索引、主体索引和频道索引，并注册 onClose 回调。
//  4. 最后启动连接 goroutine，让连接真正开始消费事件。
//
// 注意事项：
//   - `conn.Start()` 放在索引写入之后，是为了确保 goroutine 一旦启动，外部就能通过 Broker 找到它。
func (b *Broker) Register(conn *Connection) error {
	// 确保有问题时，快速失败
	if conn == nil {
		return errors.New("connection is nil")
	}
	if b.draining.Load() {
		return ErrBrokerDraining
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 再次检查 draining 状态，是为了覆盖拿锁前后之间的竞态窗口。
	if b.draining.Load() {
		return ErrBrokerDraining
	}

	// 主体连接上限在注册阶段统一拦截，避免单个用户创建过多空闲连接拖垮节点资源。
	if conn.Principal != nil && conn.Principal.SubjectID > 0 {
		if existing := len(b.bySubject[conn.Principal.SubjectID]); existing >= b.policy.MaxConnectionsPerSubject {
			return errors.New("too many active stream connections for subject")
		}
	}

	// 先写主索引，再写辅助索引，保证后续注销时总能从 conns 找到连接对象。
	b.conns[conn.ID] = conn
	if conn.Principal != nil && conn.Principal.SubjectID > 0 {
		if b.bySubject[conn.Principal.SubjectID] == nil {
			b.bySubject[conn.Principal.SubjectID] = make(map[string]*Connection)
		}
		b.bySubject[conn.Principal.SubjectID][conn.ID] = conn
	}
	if conn.Channel != "" {
		if b.byChannel[conn.Channel] == nil {
			b.byChannel[conn.Channel] = make(map[string]*Connection)
		}
		b.byChannel[conn.Channel][conn.ID] = conn
	}

	// 关闭回调收口到 Broker，是为了保证连接自发关闭时索引也能同步清理。
	conn.onClose = func(c *Connection) {
		b.unregisterInternal(c.ID, false)
	}

	// 注册完成后再启动 goroutine，避免连接启动后还未被索引收录的短暂不可见状态。
	conn.Start()
	return nil
}

// Unregister 主动从 Broker 中移除连接。
// 参数：
//   - connID：待移除连接的唯一 ID。
//
// 返回值：
//   - 无。
//
// 核心流程：
//  1. 委托给统一的内部注销逻辑。
//  2. 明确要求关闭连接，避免只删索引不关底层 writer。
//
// 注意事项：
//   - 对外入口统一走这里，是为了确保“移除连接”和“关闭连接”语义保持一致。
func (b *Broker) Unregister(connID string) {
	b.unregisterInternal(connID, true)
}

// unregisterInternal 负责执行真正的索引清理与可选关闭动作。
// 参数：
//   - connID：待移除连接 ID。
//   - closeConn：为 true 时在移除索引后主动关闭连接。
//
// 返回值：
//   - 无。
//
// 核心流程：
//  1. 在锁内删除主索引和所有辅助索引。
//  2. 在锁外执行连接关闭，避免关闭回调或 writer 逻辑放大锁持有时间。
//
// 注意事项：
//   - 锁外关闭是关键设计点，否则连接关闭链路若再次回调 Broker，会形成死锁风险。
func (b *Broker) unregisterInternal(connID string, closeConn bool) {
	var conn *Connection

	b.mu.Lock()
	conn = b.conns[connID]
	if conn != nil {
		delete(b.conns, connID)

		// 主体索引为空时及时回收子 map，避免长时间积累空桶。
		if conn.Principal != nil && conn.Principal.SubjectID > 0 {
			if conns := b.bySubject[conn.Principal.SubjectID]; conns != nil {
				delete(conns, connID)
				if len(conns) == 0 {
					delete(b.bySubject, conn.Principal.SubjectID)
				}
			}
		}

		// 频道索引同样在最后一个连接离开时删除，保证统计数据准确。
		if conn.Channel != "" {
			if conns := b.byChannel[conn.Channel]; conns != nil {
				delete(conns, connID)
				if len(conns) == 0 {
					delete(b.byChannel, conn.Channel)
				}
			}
		}
	}
	b.mu.Unlock()

	// 关闭动作放到锁外执行，既减少锁冲突，也避免 onClose 回调产生重入问题。
	if closeConn && conn != nil {
		conn.Close("unregister")
	}
}

// PublishToSubject 负责向某个主体的全部连接广播事件。
// 参数：
//   - subjectID：目标主体 ID。
//   - evt：待发送事件。
//
// 返回值：
//   - int：成功进入连接发送队列的数量。
//
// 核心流程：
//  1. 先基于读锁生成连接快照。
//  2. 再在锁外执行真正分发，降低广播期间的锁持有时间。
//
// 注意事项：
//   - 使用快照而不是直接持锁遍历，是为了避免慢写连接阻塞后续注册与注销。
func (b *Broker) PublishToSubject(subjectID uint64, evt *StreamEvent) int {
	conns := b.snapshotBySubject(subjectID)
	return b.publish(conns, evt)
}

// PublishToChannel 负责向某个频道的全部连接广播事件。
// 参数、返回值和注意事项与 PublishToSubject 相同，只是索引维度换成频道。
func (b *Broker) PublishToChannel(channel string, evt *StreamEvent) int {
	conns := b.snapshotByChannel(channel)
	return b.publish(conns, evt)
}

// publish 负责把事件投递到一组连接的本地队列。
// 参数：
//   - conns：广播目标连接快照。
//   - evt：待发送事件。
//
// 返回值：
//   - int：成功入队的连接数。
//
// 核心流程：
//  1. 空事件或空连接集直接返回，避免无意义工作。
//  2. 尝试非阻塞入队，保持广播路径不会被慢消费者拖住。
//  3. 对入队失败的慢连接统一记数并注销。
//
// 注意事项：
//   - 这里选择“踢掉慢消费者”而不是阻塞等待，是为了优先保护整体广播吞吐和服务可用性。
func (b *Broker) publish(conns []*Connection, evt *StreamEvent) int {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if evt == nil || len(conns) == 0 {
		return 0
	}

	delivered := 0
	var slow []string
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	for _, conn := range conns {
		if conn.Enqueue(evt) {
			delivered++
			continue
		}
		slow = append(slow, conn.ID)
	}

	// 慢消费者统一在第二阶段处理，避免遍历过程中边遍历边修改快照语义不清。
	if len(slow) > 0 {
		for _, connID := range slow {
			b.droppedSlowConsumers.Add(1)
			b.Unregister(connID)
		}
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return delivered
}

// RevokeSubject 负责主动关闭某主体的全部连接。
// 参数：
//   - subjectID：待撤销主体 ID。
//   - reason：关闭原因，会写入连接状态供排障使用。
//
// 返回值：
//   - int：被关闭连接数量。
//
// 核心流程：
//  1. 先拍快照，避免关闭过程中影响遍历稳定性。
//  2. 逐条调用连接关闭，让 onClose 回调自行完成索引清理。
//
// 注意事项：
//   - 关闭原因统一传入，是为了后续区分“用户主动退出”“系统排空”“权限撤销”等场景。
func (b *Broker) RevokeSubject(subjectID uint64, reason string) int {
	conns := b.snapshotBySubject(subjectID)
	for _, conn := range conns {
		conn.Close(reason)
	}
	return len(conns)
}

// Stats 返回当前 Broker 的轻量级统计视图。
// 参数：无。
// 返回值：
//   - BrokerStats：连接数、主体数、频道数以及慢消费者丢弃次数。
//
// 核心流程：
//  1. 在读锁下读取索引尺寸。
//  2. 原子读取累计指标，避免因统计而阻塞写路径。
//
// 注意事项：
//   - 统计值是瞬时快照，不保证跨字段的严格事务一致性，但足以用于观测和告警。
func (b *Broker) Stats() BrokerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BrokerStats{
		Connections:          len(b.conns),
		Subjects:             len(b.bySubject),
		Channels:             len(b.byChannel),
		DroppedSlowConsumers: b.droppedSlowConsumers.Load(),
	}
}

// BeginDrain 负责把 Broker 切换到排空模式并主动关闭所有现存连接。
// 参数：无。
// 返回值：无。
// 核心流程：
//  1. 通过原子状态只允许第一次调用生效，避免重复排空。
//  2. 获取全部连接快照并逐个关闭。
//
// 注意事项：
//   - 先切状态再关连接，是为了让新的 Register 调用立即失败，而不是继续接受连接后再关闭。
func (b *Broker) BeginDrain() {
	if !b.draining.CompareAndSwap(false, true) {
		return
	}
	for _, conn := range b.snapshotAll() {
		conn.Close("drain")
	}
}

// Close 负责对外暴露统一的关闭入口。
// 参数：无。
// 返回值：无。
// 核心流程：
//  1. 委托给 BeginDrain。
//
// 注意事项：
//   - 保留 Close 方法是为了让 Broker 更容易接入通用资源生命周期接口。
func (b *Broker) Close() {
	b.BeginDrain()
}

// snapshotBySubject 负责在读锁下复制某主体的连接列表。
// 之所以返回切片副本，是为了把后续广播逻辑挪到锁外执行，降低锁竞争。
func (b *Broker) snapshotBySubject(subjectID uint64) []*Connection {
	b.mu.RLock()
	defer b.mu.RUnlock()
	conns := b.bySubject[subjectID]
	result := make([]*Connection, 0, len(conns))
	for _, conn := range conns {
		result = append(result, conn)
	}
	return result
}

// snapshotByChannel 负责在读锁下复制某频道的连接列表。
// 它与 snapshotBySubject 的设计目标一致，都是用“快照换低锁占用”。
func (b *Broker) snapshotByChannel(channel string) []*Connection {
	b.mu.RLock()
	defer b.mu.RUnlock()
	conns := b.byChannel[channel]
	result := make([]*Connection, 0, len(conns))
	for _, conn := range conns {
		result = append(result, conn)
	}
	return result
}

// snapshotAll 负责复制当前 Broker 管理的全部连接。
// 停机排空场景用它而不是直接持锁遍历，是为了避免连接关闭回调与 Broker 锁互相等待。
func (b *Broker) snapshotAll() []*Connection {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]*Connection, 0, len(b.conns))
	for _, conn := range b.conns {
		result = append(result, conn)
	}
	return result
}
