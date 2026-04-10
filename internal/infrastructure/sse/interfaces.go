package sse
/*
1. Authorizer 授权者
  管“谁能连、谁能订阅、谁能看到什么”

2. ConnectionBroker 连接经纪人
  管“本机有哪些连接，消息怎么发给它们”

3. StreamWriter
  管“怎么把消息写成 SSE 响应发给客户端”

4. ReplayStore
  管“历史消息存哪、怎么补发”

5. Backplane5. 背板
  管“多机之间怎么同步消息和踢线命令”
*/ 
import "context"

// Authorizer 定义 SSE 接入链路的授权与事件过滤能力。
// 它把“能否连”“能否订阅”“能看到什么事件”拆成三个阶段，方便按需替换实现。
type Authorizer interface {
	// AuthorizeConnect 在连接建立前生成主体身份快照。
	AuthorizeConnect(ctx context.Context, req ConnectRequest) (*Principal, error) // 连接前

	// AuthorizeSubscribe 在主体身份已知后校验其是否允许订阅目标 channel。
	AuthorizeSubscribe(ctx context.Context, principal *Principal, channel string) error // 订阅前

	// FilterEvent 在事件写出前做最后一层过滤或裁剪。
	FilterEvent(ctx context.Context, principal *Principal, evt *StreamEvent) (*StreamEvent, error) // 写出前
}

// ConnectionBroker 定义本地连接注册、广播和踢线的最小能力集合。
// 业务层依赖接口而不是具体 Broker，有利于测试替身和未来实现替换。
type ConnectionBroker interface {
	Register(conn *Connection) error
	Unregister(connID string)
	PublishToSubject(subjectID uint64, evt *StreamEvent) int
	PublishToChannel(channel string, evt *StreamEvent) int
	RevokeSubject(subjectID uint64, reason string) int
	Stats() BrokerStats
}

// StreamWriter 抽象“如何把事件写到客户端”。
// 这样 Connection 可以只关注生命周期与节流，而不直接依赖具体 HTTP 实现。
type StreamWriter interface {
	WriteEvent(ctx context.Context, evt *StreamEvent) error
	WriteHeartbeat(ctx context.Context) error
	WriteTerminal(ctx context.Context, evt *StreamEvent) error
}

// ReplayStore 抽象 durable 事件的补发能力。
// 只有实现了 Append 与 ReplayAfter，客户端断线重连后才有机会基于 Last-Event-ID 补齐消息。
type ReplayStore interface {
	Append(ctx context.Context, evt *StreamEvent) error // 追加
	ReplayAfter(ctx context.Context, channel string, lastEventID string, limit int) ([]*StreamEvent, error) // 补发limit条
}

// Backplane 抽象多实例之间的广播和撤销命令同步能力。
// 它把数据面事件和控制面 revoke 命令分成两套接口，避免不同语义的消息混用。
type Backplane interface {
	Publish(ctx context.Context, evt *StreamEvent) error
	Subscribe(ctx context.Context, handler func(context.Context, *StreamEvent) error) error
	PublishRevoke(ctx context.Context, revoke RevokeCommand) error
	SubscribeRevoke(ctx context.Context, handler func(context.Context, RevokeCommand) error) error
}
