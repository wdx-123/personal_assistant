package ai

import "context"

// ToolParameterType 表示工具参数的 JSON 类型。
type ToolParameterType string

// ToolParameterType 枚举了工具参数可使用的 JSON 基础类型。
const (
	ToolParameterTypeObject  ToolParameterType = "object"
	ToolParameterTypeString  ToolParameterType = "string"
	ToolParameterTypeInteger ToolParameterType = "integer"
	ToolParameterTypeNumber  ToolParameterType = "number"
	ToolParameterTypeBoolean ToolParameterType = "boolean"
	ToolParameterTypeArray   ToolParameterType = "array"
)

const (
	// ToolParameterFormatRFC3339 表示字符串必须符合 RFC3339 时间格式。
	ToolParameterFormatRFC3339 = "rfc3339"
)

// ToolParameter 描述单个工具参数。
// 对 object / array 参数，使用 Properties / Items 继续描述子结构。
type ToolParameter struct {
	// Name 是参数名，会直接暴露给模型作为调用参数键名。
	Name string
	// Type 表示参数的 JSON 基础类型。
	Type ToolParameterType
	// Description 描述参数含义和使用限制，帮助模型正确构造调用参数。
	Description string
	// Required 标记该参数是否必须由模型显式提供。
	Required bool
	// Enum 给出允许的枚举值范围，便于模型约束输入。
	Enum []string
	// Format 描述字符串参数的格式约束，如 RFC3339。
	Format string
	// Pattern 描述字符串参数的正则模式约束。
	Pattern string
	// MinLength 描述字符串最小长度约束。
	MinLength *int
	// MaxLength 描述字符串最大长度约束。
	MaxLength *int
	// Minimum 描述 number/integer 参数的最小值约束。
	Minimum *float64
	// Maximum 描述 number/integer 参数的最大值约束。
	Maximum *float64
	// MinItems 描述数组最少元素个数。
	MinItems *int
	// MaxItems 描述数组最多元素个数。
	MaxItems *int
	// Examples 给出推荐示例值，帮助模型修正参数。
	Examples []string
	// DefaultValue 仅用于提示默认值，不代表 runtime 会静默注入。
	DefaultValue string
	// Properties 描述 object 参数的子字段结构。
	Properties []ToolParameter
	// Items 描述 array 参数的元素结构。
	Items *ToolParameter
}

// ToolSpec 描述一个稳定的 AI tool 协议。
type ToolSpec struct {
	// Name 是工具的稳定标识，供模型发起 tool call 时引用。
	Name string
	// Description 说明工具用途和适用边界。
	Description string
	// Parameters 描述该工具接受的结构化参数列表。
	Parameters []ToolParameter
}

// ToolCall 表示模型发起的一次工具调用。
type ToolCall struct {
	// ID 是本次调用在一轮对话内的稳定标识，用于 trace 关联。
	ID string
	// Name 是模型请求执行的工具名。
	Name string
	// ArgumentsJSON 是模型传入的 JSON 参数原文。
	ArgumentsJSON string
}

// ToolResult 表示工具执行结果。
// Output 会回传给模型；Summary / DetailMarkdown 用于 trace 投影和前端展示。
type ToolResult struct {
	// Output 是返回给模型继续推理使用的结构化结果正文。
	Output string
	// Summary 是给 trace 摘要和前端卡片使用的短说明。
	Summary string
	// DetailMarkdown 是给 trace 详情展示使用的可读内容。
	DetailMarkdown string
}

// AIToolPrincipal 表示本轮 AI tool 调用可使用的最小授权事实。
// 它只承载事实，不把用户强行映射成固定 AI 身份分类。
type AIToolPrincipal struct {
	// UserID 是当前发起会话的用户 ID。
	UserID uint
	// CurrentOrgID 是用户当前选中的组织上下文，可为空。
	CurrentOrgID *uint
	// IsSuperAdmin 标记当前用户是否具备超级管理员全局授权事实。
	IsSuperAdmin bool
}

// ToolCallContext 表示本轮工具调用共享的最小上下文。
type ToolCallContext struct {
	// ConversationID 是当前 AI 会话 ID。
	ConversationID string
	// UserMessageID 是触发本轮回答的用户消息 ID。
	UserMessageID string
	// AssistantMessageID 是当前 assistant 消息 ID。
	AssistantMessageID string
	// Principal 是本轮调用共享的最小授权事实。
	Principal AIToolPrincipal
}

// Tool 表示可供 runtime 暴露给模型的稳定工具协议。
// 具体工具实现只负责声明 spec 和执行业务，不负责决定是否可见。
type Tool interface {
	Spec() ToolSpec
	Call(ctx context.Context, call ToolCall, callCtx ToolCallContext) (ToolResult, error)
}
