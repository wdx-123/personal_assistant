package ai

import "context"

// Runtime 定义 AI 子域最小的运行时边界。
// 参数：
//   - ctx：调用链上下文，取消时 runtime 必须尽快停止输出。
//   - input：本次流式对话的输入，包括用户消息、消息 ID 和历史消息。
//   - sink：runtime 的事件输出端，runtime 只能通过它向外发送事件。
//
// 返回值：
//   - StreamResult：本次生成的最终结果摘要。
//   - error：执行失败时返回原始错误，由 Service 决定如何包装为业务错误或 SSE 错误事件。
//
// 核心流程：
//  1. Service 负责准备 StreamInput 和 Sink。
//  2. Runtime 负责调用本地实现或模型实现。
//  3. Runtime 只通过 Sink 输出 Event，不直接操作 HTTP、SSE writer 或数据库。
//
// 注意事项：
//   - domain/ai 只定义协议，不绑定 Eino、Gin、GORM、Redis 等技术实现。
type Runtime interface {
	Name() string
	Stream(ctx context.Context, input StreamInput, sink Sink) (StreamResult, error)
}

// StreamInput 表示一次基础 AI 流式对话的输入。
// 它只包含 runtime 必须知道的信息，不包含 HTTP DTO 或数据库实体。
type StreamInput struct {
	UserID uint

	ConversationID string

	UserMessageID string

	AssistantMessageID string

	Content string

	History []Message
}

// StreamResult 表示 runtime 执行完成后的结果摘要。
// Service 当前主要依赖事件投影落库，Result 用于后续扩展审计、指标或 fallback 判断。
type StreamResult struct {
	Content      string
	FinishReason string
}
