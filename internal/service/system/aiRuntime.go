package system

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
)

// AIToolBlueprint 描述一次“工具调用计划”的静态信息。
// 它不是工具执行结果本身，而是 runtime 预先规划好的工具蓝图。
type AIToolBlueprint struct {
	Kind string // 工具类型，如 task / progress / doc

	Key string // 工具唯一键，用于 trace、事件和 interrupt 关联

	Title string // 前端展示用标题

	Description string // 工具用途或执行说明

	DurationMS int64 // 模拟或记录的执行耗时（毫秒）

	Content string // 工具执行后的摘要结果

	DetailMarkdown string // 更完整的工具结果详情，通常用于展开查看

	RequiresConfirmation bool // 调用该工具前是否需要用户确认

	ConfirmationTitle string // 等待确认时展示的标题

	ConfirmationDescription string // 等待确认时展示的说明文案
}

// AIRuntimePlan 表示一次 AI 会话在执行前生成的“运行计划”。
// runtime 会先产出 plan，再按 plan 执行。
type AIRuntimePlan struct {
	Title string // 本次会话标题

	Preview string // 会话预览文本

	Lightweight bool // 是否为轻量对话（如问候、感谢）

	LightweightReply string // 轻量对话时可直接返回的回复

	Scope *resp.AssistantScopeInfo // 本次回答的作用域信息

	ShowThinkingSummary bool // 是否展示“思考摘要”区块

	TaskTool *AIToolBlueprint // 任务快照工具计划

	ProgressTool *AIToolBlueprint // 进度分析工具计划

	DocTool *AIToolBlueprint // 文档摘要工具计划（可能需要确认）

	FinalReplyConfirm string // 用户确认执行文档工具后的最终回复

	FinalReplySkip string // 用户跳过文档工具后的最终回复
}

// AIRuntimePlanInput 是 Plan 阶段的输入参数。
type AIRuntimePlanInput struct {
	Conversation *entity.AIConversation // 当前会话，可为空（新建会话）

	Request *request.StreamAssistantMessageReq // 本次用户请求
}

// AIRuntimeExecutionInput 是 Execute 阶段的输入参数。
type AIRuntimeExecutionInput struct {
	UserID uint // 当前用户 ID

	Conversation *entity.AIConversation // 当前会话实体

	Request *request.StreamAssistantMessageReq // 本次请求参数

	Plan *AIRuntimePlan // 预先生成好的执行计划

	Interrupt *entity.AIInterrupt // 本次执行关联的 interrupt，可为空

	AssistantMsgID string // 当前 assistant 消息 ID
}

// AIRuntimeDecisionCommand 表示一次 interrupt 决策提交命令。
type AIRuntimeDecisionCommand struct {
	UserID uint // 提交决策的用户 ID

	ConversationID string // 所属会话 ID

	InterruptID string // 目标 interrupt ID

	Decision string // 用户决策，如 confirm / skip

	Reason string // 决策附带说明，可为空
}

// AIRuntimeSink 是 runtime 输出事件的承接端。
// runtime 不直接操作 SSE/数据库，而是统一写给 sink。
type AIRuntimeSink interface {
	Emit(ctx context.Context, eventName string, payload any) error // 发出一个运行时事件

	Heartbeat(ctx context.Context) error // 发送保活心跳
}

// AIRuntime 定义 AI 运行时需要实现的核心能力。
type AIRuntime interface {
	Plan(ctx context.Context, input AIRuntimePlanInput) (*AIRuntimePlan, error) // 生成执行计划

	Execute(ctx context.Context, input AIRuntimeExecutionInput, sink AIRuntimeSink) error // 按计划执行并输出事件

	SubmitDecision(ctx context.Context, cmd AIRuntimeDecisionCommand) (bool, error) // 提交 interrupt 决策

	RevokeUser(ctx context.Context, userID uint, reason string) int // 撤销某用户当前等待中的会话

	NodeID() string // 返回当前 runtime 所属节点 ID
}