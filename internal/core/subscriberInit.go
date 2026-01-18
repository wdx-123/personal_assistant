package inits

import (
	"context"
	"fmt"
	"manpao-service/internal/infrastructure/interfaces/eventhandlers"
	"manpao-service/internal/infrastructure/messaging"
	"manpao-service/pkg/globals"
	"manpao-service/pkg/logger"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// SubscriberInit 初始化所有事件订阅器
// 架构说明：
// 本项目采用多 Consumer Group 模式，确保每个业务域独立消费同一事件（发布-订阅模式）。
// 每个 Group 内的消费者竞争消费（负载均衡）。
//
// 目前划分的业务域：
// 1. Order (订单): 处理支付成功、退款
// 2. Ledger (账本): 记录财务流水
// 3. Merchant (商家): 处理入驻、分佣
// 4. Timeout (超时): 处理订单支付超时
// 5. Carousel (轮播): 处理轮播图状态流转
// 6. Test (测试): 测试事件与延迟队列
// 7. Notification (通知): 发送站内信/短信
//
// nolint
func SubscriberInit(ctx context.Context) error {
	// 创建 Watermill Logger
	wmLogger := logger.NewWatermillLogger(globals.Log)

	// 1. 订单域 (Order Domain)
	// Group: order-processing-group
	// Events: payment.succeeded, payment.refunded
	if err := initOrderProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 2. 账本域 (Ledger Domain)
	// Group: ledger-processing-group
	// Events: payment.succeeded, merchant_subscription.paid, payment.refunded
	if err := initLedgerProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 3. 商家域 (Merchant Domain)
	// Group: merchant-processing-group
	// Events: merchant_subscription.paid
	if err := initMerchantProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 4. 超时域 (Timeout Domain)
	// Group: timeout-processing-group
	// Events: payment_timeout
	if err := initTimeoutProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 5. 轮播域 (Carousel Domain)
	// Group: carousel-processing-group
	// Events: carousel.start, carousel.end
	if err := initCarouselProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 6. 测试域 (Test Domain)
	// Group: test-processing-group
	// Events: test.event, test_delay_task
	if err := initTestProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 7. 通知域 (Notification Domain)
	// Group: notification-processing-group
	// Events: payment.succeeded, payment.refunded
	if err := initNotificationProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	if err := initMessageCenterProcessingGroup(ctx, wmLogger); err != nil {
		return err
	}

	// 初始化完成日志
	logInitializedGroups()

	return nil
}

// logInitializedGroups 打印已初始化的消费者组信息
func logInitializedGroups() {
	globals.Log.Info("Event Subscriber 初始化完成 - 已启动 8 个业务域 Consumer Group:")
	globals.Log.Info("  [1] Order        (订单处理)")
	globals.Log.Info("  [2] Ledger       (账本记录)")
	globals.Log.Info("  [3] Merchant     (商家入驻)")
	globals.Log.Info("  [4] Timeout      (支付超时)")
	globals.Log.Info("  [5] Carousel     (轮播调度)")
	globals.Log.Info("  [6] Test         (测试/延迟)")
	globals.Log.Info("  [7] Notification (消息通知)")
	globals.Log.Info("  [8] MessageCenter (消息中心落库)")
}

func initMessageCenterProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	messageCenterSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "message-center-processing-group",
		ConsumerName:  fmt.Sprintf("message-center-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create message center subscriber: %w", err)
	}

	handler := eventhandlers.NewMessageCenterHandler(
		messageCenterSubscriber.(*messaging.RedisStreamsSubscriber),
		globals.DB,
		wmLogger,
	)

	if err := subscribeAndConsume(
		ctx,
		messageCenterSubscriber,
		"message.write_requested",
		"[消息中心-消息落库域] 开始监听 message.write_requested 事件",
		handler.HandleWriteRequested,
		"[消息中心] 处理消息写入失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to message.write_requested for message center: %w", err)
	}

	return nil
}

func initNotificationProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	notificationSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "notification-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("notification-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create notification subscriber: %w", err)
	}

	// 初始化 Handler
	handler := eventhandlers.NewNotificationHandler(
		notificationSubscriber.(*messaging.RedisStreamsSubscriber),
		globals.DB,
		wmLogger,
	)

	// 1. 订阅支付成功
	if err := subscribeAndConsume(
		ctx,
		notificationSubscriber,
		"payment.succeeded",
		"[通知中心-通知处理域] 开始监听 payment.succeeded 事件",
		handler.HandlePaymentSucceeded,
		"[通知中心] 处理支付成功通知失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to payment.succeeded for notification: %w", err)
	}

	// 2. 订阅退款成功
	if err := subscribeAndConsume(
		ctx,
		notificationSubscriber,
		"payment.refunded",
		"[通知中心-通知处理域] 开始监听 payment.refunded 事件",
		handler.HandlePaymentRefunded,
		"[通知中心] 处理退款成功通知失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to payment.refunded for notification: %w", err)
	}

	return nil
}

func initOrderProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	orderSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "order-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("order-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create order subscriber: %w", err)
	}

	paymentSucceededMessages, err := orderSubscriber.Subscribe(ctx, "payment.succeeded")
	if err != nil {
		return fmt.Errorf("failed to subscribe to payment.succeeded: %w", err)
	}

	paymentSucceededHandler := eventhandlers.NewOrderPaymentHandler(
		orderSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[订单模块-订单处理域] 开始监听 payment.succeeded 事件")
		for msg := range paymentSucceededMessages {
			if err := paymentSucceededHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[订单模块] 处理支付成功事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	paymentRefundedMessages, err := orderSubscriber.Subscribe(ctx, "payment.refunded")
	if err != nil {
		return fmt.Errorf("failed to subscribe to payment.refunded: %w", err)
	}

	paymentRefundedHandler := eventhandlers.NewOrderRefundHandler(
		orderSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[订单模块-订单处理域] 开始监听 payment.refunded 事件")
		for msg := range paymentRefundedMessages {
			if err := paymentRefundedHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[订单模块] 处理退款成功事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	return nil
}

func initLedgerProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	ledgerSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "ledger-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("ledger-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create ledger subscriber: %w", err)
	}

	ledgerHandler := eventhandlers.NewLedgerHandler(
		ledgerSubscriber.(*messaging.RedisStreamsSubscriber),
		globals.LedgerService,
		wmLogger,
	)

	if err := subscribeAndConsume(
		ctx,
		ledgerSubscriber,
		"payment.succeeded",
		"[账本模块-账本处理域] 开始监听 payment.succeeded 事件",
		ledgerHandler.HandlePaymentSucceeded,
		"[账本模块] 创建订单支付账本失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to payment.succeeded for ledger: %w", err)
	}

	if err := subscribeAndConsume(
		ctx,
		ledgerSubscriber,
		"merchant_subscription.paid",
		"[账本模块-账本处理域] 开始监听 merchant_subscription.paid 事件",
		ledgerHandler.HandleMerchantSubscriptionPaid,
		"[账本模块] 创建商家入驻账本失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to merchant_subscription.paid for ledger: %w", err)
	}

	if err := subscribeAndConsume(
		ctx,
		ledgerSubscriber,
		"payment.refunded",
		"[账本模块-账本处理域] 开始监听 payment.refunded 事件",
		ledgerHandler.HandlePaymentRefunded,
		"[账本模块] 创建退款账本失败",
	); err != nil {
		return fmt.Errorf("failed to subscribe to payment.refunded for ledger: %w", err)
	}

	return nil
}

func subscribeAndConsume(
	ctx context.Context,
	subscriber message.Subscriber,
	topic string,
	startLog string,
	handle func(*message.Message) error,
	errPrefix string,
) error {
	msgs, err := subscriber.Subscribe(ctx, topic)
	if err != nil {
		return err
	}

	go func() {
		globals.Log.Info(startLog)
		for msg := range msgs {
			if err := handle(msg); err != nil {
				globals.Log.Errorf("%s %s: %v", errPrefix, msg.UUID, err)
			}
		}
	}()

	return nil
}

func initMerchantProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	merchantSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "merchant-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("merchant-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create merchant subscriber: %w", err)
	}

	merchantSubMessages, err := merchantSubscriber.Subscribe(ctx, "merchant_subscription.paid")
	if err != nil {
		return fmt.Errorf("failed to subscribe to merchant_subscription.paid: %w", err)
	}

	merchantSubHandler := eventhandlers.NewMerchantSubscriptionHandler(
		merchantSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[商家入驻模块-商家处理域] 开始监听 merchant_subscription.paid 事件")
		for msg := range merchantSubMessages {
			if err := merchantSubHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[商家入驻模块] 处理商家入驻支付成功事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	return nil
}

func initTimeoutProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	timeoutSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "timeout-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("timeout-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create timeout subscriber: %w", err)
	}

	paymentTimeoutMessages, err := timeoutSubscriber.Subscribe(ctx, "payment_timeout")
	if err != nil {
		return fmt.Errorf("failed to subscribe to payment_timeout: %w", err)
	}

	paymentTimeoutHandler := eventhandlers.NewPaymentTimeoutHandler(
		timeoutSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[支付超时模块-超时处理域] 开始监听 payment_timeout 事件")
		for msg := range paymentTimeoutMessages {
			if err := paymentTimeoutHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[支付超时模块] 处理支付超时事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	return nil
}

func initCarouselProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	carouselSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB, // Redis 客户端
		ConsumerGroup: "carousel-processing-group",
		ConsumerName:  fmt.Sprintf("carousel-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create carousel subscriber: %w", err)
	}

	carouselStartMessages, err := carouselSubscriber.Subscribe(ctx, "carousel.start")
	if err != nil {
		return fmt.Errorf("failed to subscribe to carousel.start: %w", err)
	}

	carouselStartHandler := eventhandlers.NewCarouselStartHandler(
		carouselSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[轮播模块-轮播处理域] 开始监听 carousel.start 事件")
		for msg := range carouselStartMessages {
			if err := carouselStartHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[轮播模块] 处理 carousel.start 事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	carouselEndMessages, err := carouselSubscriber.Subscribe(ctx, "carousel.end")
	if err != nil {
		return fmt.Errorf("failed to subscribe to carousel.end: %w", err)
	}

	carouselEndHandler := eventhandlers.NewCarouselEndHandler(
		carouselSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[轮播模块-轮播处理域] 开始监听 carousel.end 事件")
		for msg := range carouselEndMessages {
			if err := carouselEndHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[轮播模块] 处理 carousel.end 事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	return nil
}

func initTestProcessingGroup(ctx context.Context, wmLogger watermill.LoggerAdapter) error {
	testSubscriber, err := messaging.NewRedisStreamsSubscriber(messaging.SubscriberConfig{
		Client:        globals.RDB,
		ConsumerGroup: "test-processing-group", // 独立的消费者组
		ConsumerName:  fmt.Sprintf("test-consumer-%d", time.Now().Unix()),
		Logger:        wmLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create test subscriber: %w", err)
	}

	testEventMessages, err := testSubscriber.Subscribe(ctx, "test.event")
	if err != nil {
		return fmt.Errorf("failed to subscribe to test.event: %w", err)
	}

	testEventHandler := eventhandlers.NewTestEventHandler(
		testSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[测试模块-测试处理域] Event consumer started, listening on test.event")
		for msg := range testEventMessages {
			if err := testEventHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[测试模块] Failed to handle message %s: %v", msg.UUID, err)
			}
		}
		globals.Log.Info("[测试模块] Event consumer stopped")
	}()

	delayTaskMessages, err := testSubscriber.Subscribe(ctx, "test_delay_task")
	if err != nil {
		return fmt.Errorf("failed to subscribe to test_delay_task: %w", err)
	}

	delayTaskHandler := eventhandlers.NewDelayTaskTestHandler(
		testSubscriber.(*messaging.RedisStreamsSubscriber),
		wmLogger,
	)

	go func() {
		globals.Log.Info("[延迟队列测试-测试处理域] 开始监听 test_delay_task 事件")
		for msg := range delayTaskMessages {
			if err := delayTaskHandler.Handle(msg); err != nil {
				globals.Log.Errorf("[延迟队列测试] 处理延迟任务事件失败 %s: %v", msg.UUID, err)
			}
		}
	}()

	return nil
}
