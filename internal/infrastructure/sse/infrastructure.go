package sse

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// Infrastructure 聚合 SSE 所需的运行时基础设施。
// 它把 Broker、回放存储和跨实例背板收口为一个对象，便于 core/init 统一初始化和关闭。
type Infrastructure struct {
	Broker      *Broker // 本地连接
	ReplayStore ReplayStore
	Backplane   *PubSubBackplane // 多实例
	Policy      ConnectionPolicy
}

// NewInfrastructure 创建一套完整的 SSE 基础设施实例。
// 参数：
//   - client：Redis 客户端，用于回放与 Pub/Sub；为空时对应能力会自动降级。
//   - policy：连接策略。
//   - replayStreamPrefix：Redis Stream 前缀。
//   - pubSubPrefix：Pub/Sub 频道前缀。
//
// 返回值：
//   - *Infrastructure：聚合后的基础设施对象。
//
// 核心流程：
//  1. 先归一化策略，确保所有子组件拿到同一套默认值。
//  2. 创建本地 Broker。
//  3. 创建回放存储与 Pub/Sub 背板。
//
// 注意事项：
//   - 这里统一装配而不是让各模块自行 new，是为了避免同一进程里出现多套 SSE 运行时实例。
func NewInfrastructure(
	client *redis.Client,
	policy ConnectionPolicy,
	replayStreamPrefix string,
	pubSubPrefix string,
) *Infrastructure {
	policy = policy.Normalize()
	return &Infrastructure{
		Broker:      NewBroker(policy),
		ReplayStore: NewRedisReplayStore(client, replayStreamPrefix),
		Backplane:   NewPubSubBackplane(client, pubSubPrefix),
		Policy:      policy,
	}
}

// Close 负责关闭 SSE 基础设施当前进程内的活动连接。
// 参数：
//   - ctx：预留的关闭上下文；当前实现尚未消费该值。
//
// 返回值：无。
// 核心流程：
//  1. 判空保护，允许在未启用 SSE 的环境里安全调用。
//  2. 通知 Broker 进入排空模式并关闭所有连接。
//
// 注意事项：
//   - 当前仅收口本地连接，不额外关闭 Redis 客户端；Redis 生命周期由更外层基础设施统一管理。
func (i *Infrastructure) Close(ctx context.Context) {
	_ = ctx
	if i == nil || i.Broker == nil {
		return
	}
	i.Broker.BeginDrain()
}
