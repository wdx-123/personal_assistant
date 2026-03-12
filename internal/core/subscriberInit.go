package core

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/messaging"
	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/internal/service/contract"

	"go.uber.org/zap"
)

// initOJSubscribers 初始化 OJ 相关订阅器。
func initOJSubscribers(ctx context.Context, ojSvc contract.OJServiceContract) error {
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
			var payload eventdto.LuoguBindPayload
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

// InitSubscribers 初始化所有事件订阅器。
func InitSubscribers(
	ctx context.Context,
	ojSvc contract.OJServiceContract,
	permissionProjectionSvc contract.PermissionProjectionServiceContract,
	cacheProjectionSvc contract.CacheProjectionServiceContract,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if global.Redis == nil || global.Log == nil || global.Config == nil {
		return nil
	}

	if err := initOJSubscribers(ctx, ojSvc); err != nil {
		return err
	}
	if err := initPermissionSubscribers(ctx, permissionProjectionSvc); err != nil {
		return err
	}
	if cacheProjectionSvc == nil {
		return nil
	}

	// 初始化缓存投影订阅器
	cfg := global.Config.Messaging
	topic := strings.TrimSpace(cfg.CacheProjectionTopic)
	group := strings.TrimSpace(cfg.CacheProjectionGroup)
	consumer := strings.TrimSpace(cfg.CacheProjectionConsumer)
	if topic == "" || group == "" || consumer == "" {
		return errors.New("cache projection messaging config missing")
	}

	// 订阅
	subscriber := messaging.NewRedisStreamSubscriber(global.Redis, global.Log, group, consumer)
	go func() {
		err := subscriber.Subscribe(ctx, topic, func(ctx context.Context, msg *messaging.Message) error {
			var payload eventdto.CacheProjectionEvent
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return err
			}
			return cacheProjectionSvc.HandleCacheProjectionEvent(ctx, &payload)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			global.Log.Error("cache projection subscriber stopped", zap.Error(err))
		}
	}()
	return nil
}

func initPermissionSubscribers(
	ctx context.Context,
	permissionProjectionSvc contract.PermissionProjectionServiceContract,
) error {
	if permissionProjectionSvc == nil {
		return nil
	}

	cfg := global.Config.Messaging
	topic := strings.TrimSpace(cfg.PermissionProjectionTopic)
	group := strings.TrimSpace(cfg.PermissionProjectionGroup)
	consumer := strings.TrimSpace(cfg.PermissionProjectionConsumer)
	if topic == "" || group == "" || consumer == "" {
		return errors.New("permission projection messaging config missing")
	}

	subscriber := messaging.NewRedisStreamSubscriber(global.Redis, global.Log, group, consumer)
	go func() {
		err := subscriber.Subscribe(ctx, topic, func(ctx context.Context, msg *messaging.Message) error {
			var payload eventdto.PermissionProjectionEvent
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				return err
			}
			return permissionProjectionSvc.HandlePermissionProjectionEvent(ctx, &payload)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			global.Log.Error("permission projection subscriber stopped", zap.Error(err))
		}
	}()

	channel := strings.TrimSpace(cfg.PermissionPolicyReloadChannel)
	if channel == "" {
		return errors.New("permission policy reload channel config missing")
	}

	pubsub := global.Redis.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return err
	}
	go func() {
		messageCh := pubsub.Channel()
		defer func() {
			if err := pubsub.Close(); err != nil && global.Log != nil {
				global.Log.Warn("close permission reload pubsub failed", zap.Error(err))
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-messageCh:
				if !ok {
					return
				}
				if err := permissionProjectionSvc.ReloadPolicy(ctx); err != nil {
					global.Log.Error("reload permission policy failed", zap.Error(err))
				}
			}
		}
	}()
	return nil
}
