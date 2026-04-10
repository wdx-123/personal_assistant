package sse

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// ChannelStreamHandler 负责把“连接请求”转换成受 Broker 管理的长连接会话。
// 它位于鉴权、历史回放和实时订阅之间的编排层，不承担具体写出和存储实现。
type ChannelStreamHandler struct {
	Broker     ConnectionBroker
	Replay     ReplayStore
	Authorizer Authorizer
	Policy     ConnectionPolicy
}

// Serve 负责处理一次 channel 级 SSE 连接请求。
// 参数：
//   - ctx：本次连接的生命周期上下文。
//   - req：连接请求元信息，包含 channel、Last-Event-ID 等。
//   - writer：流写出器，负责把历史事件与实时事件发送给客户端。
//
// 返回值：
//   - error：鉴权、回放、注册或写出阶段失败时返回错误。
//
// 核心流程：
//  1. 兜底空上下文和空授权器，保证基础链路可运行。
//  2. 完成连接鉴权与订阅授权。
//  3. 根据 Last-Event-ID 回放缺失事件，减少客户端重连后的消息丢失。
//  4. 创建 Connection 并注册到 Broker，随后阻塞等待连接结束。
//
// 注意事项：
//   - 回放发生在注册实时连接之前，是为了尽量缩小“历史补发”和“实时订阅”之间的消息缺口。
func (h *ChannelStreamHandler) Serve(
	ctx context.Context,
	req ConnectRequest,
	writer StreamWriter,
) error {
	if h == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if h.Authorizer == nil {
		h.Authorizer = &AllowAllAuthorizer{}
	}

	// 先建立主体身份，再决定是否允许进入后续订阅和事件过滤阶段。
	principal, err := h.Authorizer.AuthorizeConnect(ctx, req)
	if err != nil {
		return err
	}
	if err := h.Authorizer.AuthorizeSubscribe(ctx, principal, req.Channel); err != nil {
		return err
	}

	// 客户端携带 Last-Event-ID 时先做回放，尽量补齐重连期间错过的 durable 事件。
	if h.Replay != nil && strings.TrimSpace(req.LastEventID) != "" {
		events, err := h.Replay.ReplayAfter(ctx, req.Channel, req.LastEventID, h.Policy.ReplayLimit)
		if err != nil {
			return err
		}
		for _, evt := range events {
			// 过滤器返回 nil 时表示该主体不应再看到该事件，因此这里只跳过当前事件而不是整体报错。
			filtered, err := h.Authorizer.FilterEvent(ctx, principal, evt)
			if err != nil || filtered == nil {
				continue
			}
			if err := writer.WriteEvent(ctx, filtered); err != nil {
				return err
			}
		}
	}

	// 连接 ID 允许外部透传；为空时在这里补默认值，便于后续追踪和注销。
	connID := strings.TrimSpace(req.ConnID)
	if connID == "" {
		connID = uuid.NewString()
	}
	conn := NewConnection(ctx, connID, principal, req.Channel, writer, h.Policy)
	if err := h.Broker.Register(conn); err != nil {
		return err
	}

	// 持续阻塞到连接结束，让调用方可以把 Serve 视为本次连接的同步生命周期。
	<-conn.Done()
	return nil
}
