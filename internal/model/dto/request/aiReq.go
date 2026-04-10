package request

// CreateAssistantConversationReq 定义当前接口使用的请求参数结构。
type CreateAssistantConversationReq struct {
	Title string `json:"title" binding:"omitempty,max=100"` // 会话标题，非必填，如果不填可以由 AI 根据内容自动生成
}

// StreamAssistantMessageReq 定义当前接口使用的请求参数结构。
type StreamAssistantMessageReq struct {
	ConversationID  string `json:"conversation_id" binding:"required,max=64"` // 会话 ID
	Content         string `json:"content" binding:"required"` // 消息内容
	ContextUserName string `json:"context_user_name" binding:"required,max=100"` // 上下文中的用户名称，便于 AI 生成更自然的回复
	ContextOrgName  string `json:"context_org_name" binding:"required,max=100"` // 上下文中的组织名称，便于 AI 生成更自然的回复
}

// SubmitAssistantDecisionReq 定义当前接口使用的请求参数结构。
type SubmitAssistantDecisionReq struct {
	ConversationID string `json:"conversation_id" binding:"required,max=64"` // 会话 ID
	InterruptID    string `json:"interrupt_id" binding:"required,max=64"` // 中断 ID
	Decision       string `json:"decision" binding:"required,oneof=confirm skip"` // 决策
	Reason         string `json:"reason" binding:"omitempty,max=500"` // 原因
}
