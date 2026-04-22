package ai

// EventName 表示 AI runtime 输出给 Service Sink 的稳定事件名。
// 它是 domain 层的最小事件协议，不依赖 SSE、HTTP 或数据库实现。
type EventName string

const (
	// EventConversationStarted 表示一次 AI 流式生成已经开始。
	EventConversationStarted EventName = "conversation_started"

	// EventAssistantToken 表示 assistant 本次追加输出的一段文本。
	EventAssistantToken EventName = "assistant_token"

	// EventMessageCompleted 表示 assistant 消息已经生成完整内容。
	EventMessageCompleted EventName = "message_completed"

	// EventError 表示 runtime 执行过程中出现可下发给前端的错误。
	EventError EventName = "error"

	// EventDone 表示当前 SSE 流已经进入最终结束态。
	EventDone EventName = "done"
)

// Event 表示 runtime 发给 Service 的最小事件对象。
// 参数：
//   - Name：事件名，必须来自本文件定义的稳定事件集。
//   - Payload：事件载荷，由具体事件名决定其结构。
//
// 注意事项：
//   - domain 层只定义事件语义，不负责 JSON 编码、SSE 写出或 DB 投影。
type Event struct {
	Name    EventName
	Payload any
}

// ConversationStartedPayload 表示会话开始事件的载荷。
type ConversationStartedPayload struct {
	Title string `json:"title"`
}

// AssistantTokenPayload 表示 assistant token 追加事件的载荷。
type AssistantTokenPayload struct {
	Token string `json:"token"`
}

// MessageCompletedPayload 表示 assistant 消息完成事件的载荷。
type MessageCompletedPayload struct {
	Content string `json:"content"`
}

// ErrorPayload 表示 AI 流式执行失败时的事件载荷。
type ErrorPayload struct {
	Message string `json:"message"`
}
