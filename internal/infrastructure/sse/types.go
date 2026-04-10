package sse

import "time"

// StreamKind 描述事件是“会话级”还是“频道级”。
// 通过显式类型区分，可以减少不同流语义之间的误用。
type StreamKind string

const (
	// StreamKindSession 表示事件属于单次会话流，通常只发给特定连接或主体。
	StreamKindSession StreamKind = "session"

	// StreamKindChannel 表示事件属于某个共享频道，通常需要广播给该频道下的多个连接。
	StreamKindChannel StreamKind = "channel"
)

const (
	// IdleKickDisconnectSlowConsumer 表示慢消费者被检测到后直接断开连接。
	// 当前 Broker 的实现走的就是这一策略，因为它最有利于保护整体吞吐。
	IdleKickDisconnectSlowConsumer = "disconnect_slow_consumer"

	// IdleKickDropOldest 预留给未来“丢弃旧消息保留连接”的策略。
	// 当前代码中尚未启用该分支，但保留常量可让配置语义保持完整。
	IdleKickDropOldest = "drop_oldest"
)

// StreamEvent 表示 SSE 链路中的标准事件结构。
// 它同时覆盖实时发送、历史回放和跨实例广播三种场景，因此保留了较完整的上下文字段。
type StreamEvent struct {
	EventID    string            `json:"event_id"` // 唯一标志
	StreamKind StreamKind        `json:"stream_kind"` // 会话级或频道级
	Channel    string            `json:"channel"` // 所属频道
	TenantID   uint64            `json:"tenant_id"` // 租户ID
	SubjectID  uint64            `json:"subject_id"` // 发给谁
	EventName  string            `json:"event_name"`
	Data       []byte            `json:"data"`
	OccurredAt time.Time         `json:"occurred_at"`
	RetryMS    int64             `json:"retry_ms"` // 断线重连时间，毫秒级
	Durable    bool              `json:"durable"`
	RequestID  string            `json:"request_id"`
	TraceID    string            `json:"trace_id"`
	Meta       map[string]string `json:"meta,omitempty"` // 拓展字段
}

// ConnectionPolicy 定义连接管理和写出行为的关键运行参数。
// 这些参数集中在一处，是为了让 Broker、Connection 和 Writer 使用同一套策略。
type ConnectionPolicy struct {
	HeartbeatInterval        time.Duration // 心跳间隔
	WriteTimeout             time.Duration // 写出超时
	QueueCapacity            int // 写出队列容量
	MaxConnectionsPerSubject int // 每个主体的最大连接数
	ReplayLimit              int // 回放限制
	IdleKickPolicy           string // 空闲踢出策略
}

// Normalize 负责把连接策略补齐为可执行配置。
// 参数：无。
// 返回值：
//   - ConnectionPolicy：填充默认值后的策略副本。
//
// 核心流程：
//  1. 逐项检查时间、容量和限额字段是否有效。
//  2. 对缺失项注入系统默认值，确保运行时不必反复判空。
//
// 注意事项：
//   - 返回的是副本而不是原地修改指针，便于调用方在不共享状态的前提下安全复用。
func (p ConnectionPolicy) Normalize() ConnectionPolicy {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if p.HeartbeatInterval <= 0 {
		p.HeartbeatInterval = 20 * time.Second
	}
	if p.WriteTimeout <= 0 {
		p.WriteTimeout = 10 * time.Second
	}
	if p.QueueCapacity <= 0 {
		p.QueueCapacity = 64
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if p.MaxConnectionsPerSubject <= 0 {
		p.MaxConnectionsPerSubject = 3
	}
	if p.ReplayLimit <= 0 {
		p.ReplayLimit = 100
	}
	if p.IdleKickPolicy == "" {
		p.IdleKickPolicy = IdleKickDisconnectSlowConsumer
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return p
}

// Principal 表示 SSE 连接在基础设施层可识别的主体信息。
// 它刻意只保留与流控制相关的通用字段，避免把过多业务模型耦合进基础设施。
type Principal struct {
	UserID    uint
	SubjectID uint64
	TenantID  uint64
	OrgID     uint
	Origin    string
	Scopes    []string
	Meta      map[string]string
}

// ConnectRequest 表示建立连接时所需的元信息。
// 根据上下文推测，该结构既可能来自 HTTP 请求解析，也可能来自更高层的协议适配层。
type ConnectRequest struct {
	ConnID      string // 连接唯一标识
	StreamKind  StreamKind
	Channel     string // 目标频道
	SubjectID   uint64
	TenantID    uint64
	Method      string
	LastEventID string // 用于断线重连时的事件回放
	Origin      string // 连接来源
	QueryToken  string
	Headers     map[string]string
}

// RevokeCommand 表示跨实例同步的踢线命令。
// 当某主体权限变化、被强制下线或会话失效时，可通过它触发各节点同步断开。
type RevokeCommand struct {
	SubjectID uint64 `json:"subject_id"`
	Reason    string `json:"reason"`
}

// BrokerStats 是 Broker 的只读统计视图。
// 这些指标主要用于监控和排障，不参与业务决策。
type BrokerStats struct {
	Connections          int   `json:"connections"`
	Subjects             int   `json:"subjects"`
	Channels             int   `json:"channels"`
	DroppedSlowConsumers int64 `json:"dropped_slow_consumers"`
}
