package sse

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-redis/redis/v8"
)

// PubSubBackplane 封装 Redis Pub/Sub 作为多实例之间的广播背板。
// 它只负责跨进程转发事件，不参与连接生命周期管理；真正的连接分发仍由本地 Broker 负责。
type PubSubBackplane struct {
	client        *redis.Client
	eventChannel  string
	revokeChannel string
}

// NewPubSubBackplane 创建一个基于 Redis Pub/Sub 的背板实现。
// 参数：
//   - client：Redis 客户端；为空时对象仍可创建，但发布/订阅调用会直接降级为空操作。
//   - prefix：频道名前缀，用于不同环境或业务域隔离消息空间。
//
// 返回值：
//   - *PubSubBackplane：可选启用的背板实例。
//
// 核心流程：
//  1. 先清洗 prefix，避免首尾空白导致频道命名不一致。
//  2. 若未显式配置前缀，则回退到统一的 `sse` 默认值。
//
// 注意事项：
//   - 频道命名在这里统一收口，是为了避免发布端和订阅端各自拼接造成不兼容。
func NewPubSubBackplane(client *redis.Client, prefix string) *PubSubBackplane {
	base := strings.TrimSpace(prefix)
	if base == "" {
		base = "sse"
	}
	return &PubSubBackplane{
		client:        client,
		eventChannel:  base + ":events",
		revokeChannel: base + ":revoke",
	}
}

// Publish 负责把普通流事件广播到 Redis 背板。
// 参数：
//   - ctx：发布阶段上下文，用于超时、取消和链路透传。
//   - evt：待广播的流事件。
//
// 返回值：
//   - error：序列化或 Redis 发布失败时返回错误。
//
// 核心流程：
//  1. 先做空指针和空客户端兜底，让未启用背板的环境保持无害降级。
//  2. 再把事件序列化成 JSON，保证跨实例传输结构稳定。
//  3. 最后通过 Redis Pub/Sub 发布到统一事件频道。
//
// 注意事项：
//   - 这里返回 nil 而不是报错终止，是因为“未配置背板”属于可接受部署形态，不应放大成业务异常。
func (p *PubSubBackplane) Publish(ctx context.Context, evt *StreamEvent) error {
	if p == nil || p.client == nil || evt == nil {
		return nil
	}

	// 统一序列化为 JSON，便于不同实例按同一结构反序列化，不依赖内存共享。
	raw, err := json.Marshal(evt)
	if err != nil {
		return err
	}

	// Redis 的 Publish 是一次性操作，返回错误时交由上层决定是否重试或降级。
	return p.client.Publish(ctx, p.eventChannel, raw).Err()
}

// Subscribe 负责持续订阅普通流事件并交给调用方处理。
// 参数：
//   - ctx：订阅生命周期上下文；取消时会主动退出循环并释放 Redis 订阅。
//   - handler：每条事件的处理函数。
//
// 返回值：
//   - error：上下文取消或 handler 返回错误时结束并返回对应错误。
//
// 核心流程：
//  1. 先校验依赖是否齐全，未启用背板时直接空返回。
//  2. 建立 Redis 订阅并在函数退出时关闭，避免连接泄露。
//  3. 循环读取消息、反序列化事件并交给 handler。
//
// 注意事项：
//   - 反序列化失败的消息会被跳过而不是终止订阅，因为单条脏消息不应拖垮整条广播链路。
func (p *PubSubBackplane) Subscribe(
	ctx context.Context,
	handler func(context.Context, *StreamEvent) error,
) error {
	if p == nil || p.client == nil || handler == nil {
		return nil
	}

	// 订阅对象必须在退出时关闭，否则 Redis 客户端会一直保留服务器侧订阅状态。
	sub := p.client.Subscribe(ctx, p.eventChannel)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	for {
		select {
		// 上下文结束时立即退出，确保外部停机、重载或请求取消能及时生效。
		case <-ctx.Done():
			return ctx.Err()

		// 持续消费背板消息；nil 消息直接跳过，避免意外值触发空指针。
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// 对单条异常消息做隔离处理，避免整个订阅 goroutine 因脏数据退出。
			var evt StreamEvent
			if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
				continue
			}

			// handler 返回错误时直接上抛，让调用方决定是否重建订阅或整体 fail-fast。
			if err := handler(ctx, &evt); err != nil {
				return err
			}
		}
	}
}

// PublishRevoke 负责广播“撤销某主体连接”的控制命令。
// 参数：
//   - ctx：发布阶段上下文。
//   - revoke：撤销指令，通常用于跨实例同步踢线。
//
// 返回值：
//   - error：序列化或发布失败时返回错误。
//
// 核心流程：
//  1. 未启用背板时直接降级为无操作。
//  2. 序列化撤销命令后发布到独立频道，避免与普通事件混流。
//
// 注意事项：
//   - 普通业务事件与 revoke 指令拆频道，是为了让消费端按语义使用不同处理路径。
func (p *PubSubBackplane) PublishRevoke(ctx context.Context, revoke RevokeCommand) error {
	if p == nil || p.client == nil {
		return nil
	}

	raw, err := json.Marshal(revoke)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, p.revokeChannel, raw).Err()
}

// SubscribeRevoke 负责持续监听撤销命令并交给调用方执行。
// 参数：
//   - ctx：订阅生命周期上下文。
//   - handler：收到撤销命令后的处理函数。
//
// 返回值：
//   - error：上下文取消或 handler 返回错误时结束。
//
// 核心流程：
//  1. 建立 revoke 专用订阅。
//  2. 循环反序列化命令并调用 handler。
//  3. 在退出前关闭订阅对象，释放资源。
//
// 注意事项：
//   - 撤销命令是控制面消息，处理失败通常意味着跨实例一致性受影响，因此这里不会吞掉 handler 错误。
func (p *PubSubBackplane) SubscribeRevoke(
	ctx context.Context,
	handler func(context.Context, RevokeCommand) error,
) error {
	if p == nil || p.client == nil || handler == nil {
		return nil
	}

	sub := p.client.Subscribe(ctx, p.revokeChannel)
	defer func() { _ = sub.Close() }()

	ch := sub.Channel()
	for {
		select {
		// 统一尊重外部生命周期，避免后台订阅在服务停止后继续运行。
		case <-ctx.Done():
			return ctx.Err()

		// 逐条消费 revoke 命令；消息为空时直接跳过，保持循环健壮性。
		case msg := <-ch:
			if msg == nil {
				continue
			}

			// 单条命令反序列化失败只影响当前消息，不让整个控制链路提前退出。
			var revoke RevokeCommand
			if err := json.Unmarshal([]byte(msg.Payload), &revoke); err != nil {
				continue
			}

			// 控制命令处理失败需要尽快暴露给上层，否则撤销行为可能悄悄失效。
			if err := handler(ctx, revoke); err != nil {
				return err
			}
		}
	}
}
