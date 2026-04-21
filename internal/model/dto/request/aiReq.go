package request

// CreateAssistantConversationReq 定义当前接口使用的请求参数结构。
type CreateAssistantConversationReq struct {
	Title string `json:"title" binding:"omitempty,max=100"` // 会话标题，非必填，如果不填可以由 AI 根据内容自动生成
}

// StreamAssistantMessageReq 定义当前接口使用的请求参数结构。
type StreamAssistantMessageReq struct {
	ConversationID  string `json:"conversation_id" binding:"required,max=64"`     // 会话 ID
	Content         string `json:"content" binding:"required"`                    // 消息内容
	ContextUserName string `json:"context_user_name" binding:"omitempty,max=100"` // 兼容字段；正式上下文由服务端推导
	ContextOrgName  string `json:"context_org_name" binding:"omitempty,max=100"`  // 兼容字段；正式上下文由服务端推导
}
