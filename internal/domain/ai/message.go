package ai

const (
	// RoleUser 表示用户消息角色。
	RoleUser = "user"
	// RoleAssistant 表示 AI assistant 消息角色。
	RoleAssistant = "assistant"

	// MessageStatusLoading 表示 assistant 消息仍在生成中。
	MessageStatusLoading = "loading"
	// MessageStatusSuccess 表示 assistant 消息已成功完成。
	MessageStatusSuccess = "success"
	// MessageStatusError 表示 assistant 消息生成失败。
	MessageStatusError = "error"
	// MessageStatusStopped 表示 assistant 消息因取消或超时停止。
	MessageStatusStopped = "stopped"
)

// Message 表示 runtime 可读取的最小历史消息结构。
// 它用于向模型提供上下文，不暴露数据库实体或前端响应 DTO。
type Message struct {
	ID      string
	Role    string
	Content string
}
