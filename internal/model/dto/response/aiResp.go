package response

// AssistantConversationGroup 表示会话列表中的时间分组。
type AssistantConversationGroup string

const (
	AssistantConversationGroupToday  AssistantConversationGroup = "今天"
	AssistantConversationGroupRecent AssistantConversationGroup = "最近"
	AssistantConversationGroupOlder  AssistantConversationGroup = "更早"
)

// AssistantConversationResp 表示会话列表项的响应结构。
type AssistantConversationResp struct {
	ID           string                     `json:"id"`            // 会话唯一标识。
	Title        string                     `json:"title"`         // 会话标题，通常由首轮问题或摘要生成。
	Preview      string                     `json:"preview"`       // 会话预览文本，通常用于列表摘要展示。
	UpdatedAt    string                     `json:"updated_at"`    // 会话最近更新时间的格式化字符串。
	Timestamp    int64                      `json:"timestamp"`     // 会话最近更新时间的时间戳，便于排序或分组。
	Group        AssistantConversationGroup `json:"group"`         // 会话所属时间分组，如今天、最近、更早。
	IsGenerating bool                       `json:"is_generating"` // 当前会话是否仍有消息在生成中。
}

// AssistantTraceAction 表示轨迹节点上的可执行动作。
type AssistantTraceAction struct {
	Key    string `json:"key"`             // 动作唯一标识，用于前端定位或回传。
	Label  string `json:"label"`           // 动作按钮文案，展示给用户。
	Action string `json:"action"`          // 动作类型或动作指令，如 accept / reject / retry。
	Style  string `json:"style,omitempty"` // 动作样式标记，如 primary / danger，供前端渲染使用。
}

// AssistantTraceItem 表示一条执行轨迹节点。
type AssistantTraceItem struct {
	Key                     string                 `json:"key"`                                // 轨迹节点唯一标识。
	Title                   string                 `json:"title"`                              // 轨迹节点标题，概括当前步骤。
	Description             string                 `json:"description"`                        // 轨迹节点简述，说明当前步骤做了什么。
	Status                  string                 `json:"status"`                             // 轨迹状态，如 running / success / failed / waiting。
	InterruptID             string                 `json:"interrupt_id,omitempty"`             // 中断确认场景下的中断 ID，用于后续确认或拒绝。
	DurationMS              int64                  `json:"duration_ms,omitempty"`              // 当前步骤耗时，单位毫秒。
	Content                 string                 `json:"content,omitempty"`                  // 当前步骤的简要结果内容，供前端折叠进普通消息显示。
	DetailMarkdown          string                 `json:"detail_markdown,omitempty"`          // 当前步骤的详细说明，通常为 Markdown 格式，仅在前端需要补充时使用。
	RequiresConfirmation    bool                   `json:"requires_confirmation,omitempty"`    // 当前步骤是否需要用户确认后才能继续。
	ConfirmationTitle       string                 `json:"confirmation_title,omitempty"`       // 确认弹窗或确认区域的标题。
	ConfirmationDescription string                 `json:"confirmation_description,omitempty"` // 对确认事项的补充说明。
	Actions                 []AssistantTraceAction `json:"actions,omitempty"`                  // 当前节点可供用户选择的动作列表。
}

// AssistantScopeInfo 表示当前消息所处的业务上下文范围。
type AssistantScopeInfo struct {
	UserName      string `json:"user_name"`                 // 当前上下文中的用户名。
	OrgName       string `json:"org_name"`                  // 当前上下文中的组织名。
	ScopeLabel    string `json:"scope_label"`               // 当前作用域标签，如个人空间、组织空间等。
	TaskName      string `json:"task_name,omitempty"`       // 当前关联的任务名称。
	DocScopeLabel string `json:"doc_scope_label,omitempty"` // 当前文档范围标签，用于说明文档上下文来源。
}

// AssistantMessageResp 表示单条消息的响应结构。
type AssistantMessageResp struct {
	ID             string               `json:"id"`                   // 消息唯一标识。
	ConversationID string               `json:"conversation_id"`      // 所属会话 ID。
	Role           string               `json:"role"`                 // 消息角色，如 user / assistant / system。
	Content        string               `json:"content"`              // 消息最终正文内容。
	CreatedAt      string               `json:"created_at"`           // 消息创建时间的格式化字符串。
	Status         string               `json:"status"`               // 消息状态，如 loading / success / error / stopped。
	TraceItems     []AssistantTraceItem `json:"trace_items"`          // 当前消息关联的执行轨迹列表，前端会把这些轨迹折叠进同一条消息显示。
	Scope          *AssistantScopeInfo  `json:"scope,omitempty"`      // 当前消息关联的作用域信息，为空表示无额外上下文。
	ErrorText      string               `json:"error_text,omitempty"` // 错误信息文本，通常在失败场景下返回。
}

// AssistantConversationStartedPayload 表示会话开始事件的载荷。
type AssistantConversationStartedPayload struct {
	Title string `json:"title"` // 新会话生成后的标题。
}

// AssistantTokenPayload 表示流式输出 token 事件的载荷。
type AssistantTokenPayload struct {
	Token string `json:"token"` // 本次流式追加的 token 内容。
}

// AssistantToolCallStartedPayload 表示工具调用开始事件的载荷。
type AssistantToolCallStartedPayload struct {
	Key         string `json:"key"`                 // 工具调用步骤唯一标识。
	ToolName    string `json:"tool_name,omitempty"` // 被调用的工具名，便于前端展示和排障。
	Title       string `json:"title"`               // 工具调用步骤标题。
	Description string `json:"description"`         // 工具调用开始时的说明文本。
}

// AssistantToolCallFinishedPayload 表示工具调用结束事件的载荷。
type AssistantToolCallFinishedPayload struct {
	Key            string `json:"key"`                       // 工具调用步骤唯一标识。
	ToolName       string `json:"tool_name,omitempty"`       // 执行完成的工具名，便于前端展示和排障。
	Description    string `json:"description"`               // 工具调用结束后的简述。
	DurationMS     int64  `json:"duration_ms"`               // 工具调用耗时，单位毫秒。
	Status         string `json:"status"`                    // 工具调用结果状态，如 success / failed。
	Content        string `json:"content,omitempty"`         // 工具调用返回的简要结果。
	DetailMarkdown string `json:"detail_markdown,omitempty"` // 工具调用详细结果，通常为 Markdown。
}

// AssistantToolCallWaitingConfirmationPayload 表示工具调用进入待确认状态时的载荷。
type AssistantToolCallWaitingConfirmationPayload struct {
	InterruptID             string                 `json:"interrupt_id"`              // 本次待确认中断的唯一标识。
	Key                     string                 `json:"key"`                       // 当前工具调用步骤唯一标识。
	Title                   string                 `json:"title"`                     // 待确认步骤标题。
	Description             string                 `json:"description"`               // 待确认步骤说明。
	DetailMarkdown          string                 `json:"detail_markdown,omitempty"` // 待确认的详细说明，通常为 Markdown。
	ConfirmationTitle       string                 `json:"confirmation_title"`        // 确认区域标题。
	ConfirmationDescription string                 `json:"confirmation_description"`  // 确认区域说明文本。
	Actions                 []AssistantTraceAction `json:"actions"`                   // 用户可选的确认动作列表。
}

// AssistantToolCallConfirmationResultPayload 表示用户确认后返回的结果载荷。
type AssistantToolCallConfirmationResultPayload struct {
	InterruptID    string `json:"interrupt_id"`              // 已处理的中断 ID。
	Key            string `json:"key"`                       // 对应的工具调用步骤 ID。
	Decision       string `json:"decision"`                  // 用户做出的决定，如 accept / reject。
	Status         string `json:"status"`                    // 决定生效后的步骤状态。
	Description    string `json:"description"`               // 对本次确认结果的简要说明。
	DetailMarkdown string `json:"detail_markdown,omitempty"` // 对本次确认结果的详细说明。
}

// AssistantStructuredBlockPayload 表示作用域信息事件的载荷。
type AssistantStructuredBlockPayload struct {
	Scope *AssistantScopeInfo `json:"scope,omitempty"` // 本次下发的作用域信息。
}

// AssistantMessageCompletedPayload 表示消息生成完成事件的载荷。
type AssistantMessageCompletedPayload struct {
	Content string `json:"content"` // 消息最终完整内容。
}

// AssistantErrorPayload 表示错误事件的载荷。
type AssistantErrorPayload struct {
	Message string `json:"message"` // 错误描述信息。
}
