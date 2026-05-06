package core

import (
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/sse"
)

// InitSSEInfrastructure 负责根据全局配置初始化 SSE 运行时基础设施。
// 参数：无。
// 返回值：无。
// 核心流程：
//  1. 先确认配置和 Redis 都已就绪，避免在依赖缺失时构造出不可用实例。
//  2. 把配置层的秒级数值映射为 ConnectionPolicy。
//  3. 创建聚合后的 SSE Infrastructure 并挂到全局变量，供控制器和服务层复用。
//
// 注意事项：
//   - 这里不重复创建 Redis 客户端；SSE 只消费外层已经初始化好的全局资源，符合基础设施边界要求。
func InitSSEInfrastructure() {
	// 缺少配置或 Redis 时直接跳过，是为了允许未启用 SSE 的部署形态平稳启动。
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if global.Config == nil || global.Redis == nil {
		return
	}

	cfg := global.Config.SSE
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	policy := sse.ConnectionPolicy{
		HeartbeatInterval:        time.Duration(cfg.HeartbeatIntervalSeconds) * time.Second,
		WriteTimeout:             time.Duration(cfg.WriteTimeoutSeconds) * time.Second,
		QueueCapacity:            cfg.QueueCapacity,
		MaxConnectionsPerSubject: cfg.MaxConnectionsPerSubject,
		ReplayLimit:              cfg.ReplayLimit,
		IdleKickPolicy:           sse.IdleKickDisconnectSlowConsumer,
	}

	// 统一在 core 层组装基础设施，是为了保证全局只存在一套 SSE 运行时实例。
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	global.StreamInfra = sse.NewInfrastructure(
		global.Redis,
		policy,
		cfg.ReplayStreamPrefix,
		cfg.PubSubChannelPrefix,
	)
}
