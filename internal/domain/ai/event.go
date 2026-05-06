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

	// EventToolCallStarted 表示 assistant 已经开始调用某个工具。
	EventToolCallStarted EventName = "tool_call_started"

	// EventToolCallFinished 表示一次工具调用已经结束。
	EventToolCallFinished EventName = "tool_call_finished"

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

// ToolCallStartedPayload 表示工具调用开始事件的载荷。
type ToolCallStartedPayload struct {
	// Key 是本次工具调用在 trace_items 中的稳定主键。
	Key string `json:"key"`
	// ToolName 是被调用的工具名，便于前端展示和排障。
	ToolName string `json:"tool_name,omitempty"`
	// Title 是前端或 trace 卡片展示用的标题。
	Title string `json:"title"`
	// Description 是当前阶段的人类可读状态说明。
	Description string `json:"description"`
}

// ToolCallFinishedPayload 表示工具调用结束事件的载荷。
type ToolCallFinishedPayload struct {
	// Key 是本次工具调用在 trace_items 中的稳定主键。
	Key string `json:"key"`
	// ToolName 是完成执行的工具名。
	ToolName string `json:"tool_name,omitempty"`
	// Description 是完成阶段的人类可读状态说明。
	Description string `json:"description"`
	// DurationMS 记录本次工具执行耗时，单位毫秒。
	DurationMS int64 `json:"duration_ms"`
	// Status 表示本次工具调用最终状态，如 success 或 failed。
	Status string `json:"status"`
	// Content 是用于摘要展示的短结果。
	Content string `json:"content,omitempty"`
	// DetailMarkdown 是给 trace 详情面板展示的完整内容。
	DetailMarkdown string `json:"detail_markdown,omitempty"`
}

// ErrorPayload 表示 AI 流式执行失败时的事件载荷。
type ErrorPayload struct {
	Message string `json:"message"`
}
