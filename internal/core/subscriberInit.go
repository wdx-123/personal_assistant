package core

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/messaging"
	"personal_assistant/internal/service/system"

	"go.uber.org/zap"
)

// InitSubscribers 初始化所有事件订阅器
// 目前仅保留空实现，等待后续业务模块接入
func InitSubscribers(ctx context.Context, ojSvc *system.OJService) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ojSvc == nil || global.Redis == nil || global.Log == nil || global.Config == nil {
		return nil
	}

	cfg := global.Config.Messaging
	topic := strings.TrimSpace(cfg.LuoguBindTopic)
	group := strings.TrimSpace(cfg.LuoguBindGroup)
	consumer := strings.TrimSpace(cfg.LuoguBindConsumer)
	if topic == "" || group == "" || consumer == "" {
		return errors.New("luogu bind messaging config missing")
	}

	// 订阅
	subscriber := messaging.NewRedisStreamSubscriber(global.Redis, global.Log, group, consumer)
	go func() {
		err := subscriber.Subscribe(ctx, topic, func(ctx context.Context, msg *messaging.Message) error {
			// 存的用户id
			aggregateID := strings.TrimSpace(msg.Metadata["aggregate_id"])
			if aggregateID == "" {
				return errors.New("luogu bind aggregate id missing")
			}
			userID, err := strconv.ParseUint(aggregateID, 10, 64)
			if err != nil || userID == 0 {
				return errors.New("invalid luogu bind aggregate id")
			}
			// 绑定数据
			var payload system.LuoguBindPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return err
			}
			return ojSvc.HandleLuoguBindPayload(ctx, uint(userID), &payload)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			global.Log.Error("luogu bind subscriber stopped", zap.Error(err))
		}
	}()

	leetcodeTopic := strings.TrimSpace(cfg.LeetcodeBindTopic)                 // 获取 LeetCode 绑定主题
	leetcodeGroup := strings.TrimSpace(cfg.LeetcodeBindGroup)                 // 获取 LeetCode 消费组
	leetcodeConsumer := strings.TrimSpace(cfg.LeetcodeBindConsumer)           // 获取 LeetCode 消费者名
	if leetcodeTopic == "" || leetcodeGroup == "" || leetcodeConsumer == "" { // 校验配置完整性
		return errors.New("leetcode bind messaging config missing") // 配置缺失直接返回
	}

	leetcodeSubscriber := messaging.NewRedisStreamSubscriber(global.Redis, global.Log, leetcodeGroup, leetcodeConsumer) // 创建订阅器
	go func() {                                                                                                         // 启动异步订阅
		err := leetcodeSubscriber.Subscribe(
			ctx,
			leetcodeTopic,
			func(ctx context.Context, msg *messaging.Message) error { // 注册回调
				aggregateID := strings.TrimSpace(msg.Metadata["aggregate_id"]) // 读取用户 ID
				if aggregateID == "" {                                         // 校验用户 ID
					return errors.New("leetcode bind aggregate id missing") // 缺失则报错
				}
				userID, err := strconv.ParseUint(aggregateID, 10, 64) // 解析用户 ID
				if err != nil || userID == 0 {                        // 校验解析结果
					return errors.New("invalid leetcode bind aggregate id") // 非法则报错
				}
				return ojSvc.HandleLeetcodeBindSignal(ctx, uint(userID)) // 触发 LeetCode 异步刷新
			})
		if err != nil && !errors.Is(err, context.Canceled) { // 订阅异常且非取消
			global.Log.Error("leetcode bind subscriber stopped", zap.Error(err)) // 记录错误日志
		}
	}()
	return nil
}
