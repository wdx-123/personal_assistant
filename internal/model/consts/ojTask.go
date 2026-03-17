package consts

// OJTaskMode 任务创建模式。
type OJTaskMode string

const (
	// OJTaskModeImmediate 表示任务创建后立即进入待执行队列，不要求提供 execute_at。
	OJTaskModeImmediate OJTaskMode = "immediate"
	// OJTaskModeScheduled 表示任务按指定时间调度执行，要求提供未来时间的 execute_at。
	OJTaskModeScheduled OJTaskMode = "scheduled"
)

// OJTaskStatus 任务版本状态。
type OJTaskStatus string

const (
	// OJTaskStatusScheduled 表示任务已创建但尚未到达执行时间。
	OJTaskStatusScheduled OJTaskStatus = "scheduled"
	// OJTaskStatusQueued 表示任务已经进入执行队列，等待调度器消费。
	OJTaskStatusQueued OJTaskStatus = "queued"
	// OJTaskStatusExecuting 表示任务正在生成快照或执行判题结果聚合。
	OJTaskStatusExecuting OJTaskStatus = "executing"
	// OJTaskStatusSucceeded 表示任务执行完成且流程成功结束。
	OJTaskStatusSucceeded OJTaskStatus = "succeeded"
	// OJTaskStatusFailed 表示任务执行结束但流程出现不可恢复错误。
	OJTaskStatusFailed OJTaskStatus = "failed"
	// OJTaskStatusDeleted 表示任务版本已被业务删除，不再允许继续编辑或执行。
	OJTaskStatusDeleted OJTaskStatus = "deleted"
)

// OJTaskExecutionStatus 执行状态。
type OJTaskExecutionStatus string

const (
	// OJTaskExecutionStatusScheduled 表示执行记录已生成，等待到达计划时间。
	OJTaskExecutionStatusScheduled OJTaskExecutionStatus = "scheduled"
	// OJTaskExecutionStatusQueued 表示执行记录已进入调度队列，等待 worker 抢占。
	OJTaskExecutionStatusQueued OJTaskExecutionStatus = "queued"
	// OJTaskExecutionStatusExecuting 表示当前执行记录正在被 worker 处理。
	OJTaskExecutionStatusExecuting OJTaskExecutionStatus = "executing"
	// OJTaskExecutionStatusSucceeded 表示执行记录处理完成且结果已落库。
	OJTaskExecutionStatusSucceeded OJTaskExecutionStatus = "succeeded"
	// OJTaskExecutionStatusFailed 表示执行记录处理失败，通常伴随 error_message。
	OJTaskExecutionStatusFailed OJTaskExecutionStatus = "failed"
	// OJTaskExecutionStatusCancelled 表示执行被主动取消，例如任务删除后终止执行。
	OJTaskExecutionStatusCancelled OJTaskExecutionStatus = "cancelled"
)

// OJTaskExecutionTriggerType 执行触发来源。
type OJTaskExecutionTriggerType string

const (
	// OJTaskExecutionTriggerCreateImmediate 表示任务创建时以立即模式触发执行。
	OJTaskExecutionTriggerCreateImmediate OJTaskExecutionTriggerType = "create_immediate"
	// OJTaskExecutionTriggerScheduleDue 表示定时任务到点后由调度器触发执行。
	OJTaskExecutionTriggerScheduleDue OJTaskExecutionTriggerType = "schedule_due"
	// OJTaskExecutionTriggerExecuteNow 表示用户显式调用“立即执行”触发执行。
	OJTaskExecutionTriggerExecuteNow OJTaskExecutionTriggerType = "execute_now"
	// OJTaskExecutionTriggerRetry 表示基于历史执行结果重新发起一次执行。
	OJTaskExecutionTriggerRetry OJTaskExecutionTriggerType = "retry"
)

// OJTaskExecutionUserItemResultStatus 执行用户题目结果状态。
type OJTaskExecutionUserItemResultStatus string

const (
	// OJTaskExecutionUserItemResultCompleted 表示用户在该题目上已满足完成条件。
	OJTaskExecutionUserItemResultCompleted OJTaskExecutionUserItemResultStatus = "completed"
	// OJTaskExecutionUserItemResultPending 表示用户在该题目上尚未满足完成条件。
	OJTaskExecutionUserItemResultPending OJTaskExecutionUserItemResultStatus = "pending"
)

// OJTaskExecutionUserItemPendingReason pending 原因。
type OJTaskExecutionUserItemPendingReason string

const (
	// OJTaskExecutionUserItemReasonAccountUnbound 表示用户未绑定对应 OJ 平台账号，无法判定完成。
	OJTaskExecutionUserItemReasonAccountUnbound OJTaskExecutionUserItemPendingReason = "account_unbound"
	// OJTaskExecutionUserItemReasonUnsolved 表示用户已具备判题条件，但当前题目尚未完成。
	OJTaskExecutionUserItemReasonUnsolved OJTaskExecutionUserItemPendingReason = "unsolved"
	// OJTaskExecutionUserItemReasonQuestionNotFound 表示题目在执行前未能转正为有效真题。
	OJTaskExecutionUserItemReasonQuestionNotFound OJTaskExecutionUserItemPendingReason = "question_not_found"
)

// OJQuestionSourceStatus 题库来源状态。
type OJQuestionSourceStatus int8

const (
	// OJQuestionSourceStatusVerified 表示题目已验证。
	OJQuestionSourceStatusVerified OJQuestionSourceStatus = 1
	// OJQuestionSourceStatusPending 表示题目为影子题，待后续校验。
	OJQuestionSourceStatusPending OJQuestionSourceStatus = 2
	// OJQuestionSourceStatusInvalid 表示题目经预检后确认为无效。
	OJQuestionSourceStatusInvalid OJQuestionSourceStatus = 3
)

// OJQuestionSourceType 题库来源类型。
type OJQuestionSourceType string

const (
	// OJQuestionSourceTypeSync 表示题目来自同步链路。
	OJQuestionSourceTypeSync OJQuestionSourceType = "sync"
	// OJQuestionSourceTypeManual 表示题目来自手工录入。
	OJQuestionSourceTypeManual OJQuestionSourceType = "manual"
)

// OJTaskItemResolutionStatus 任务题目的解析状态。
type OJTaskItemResolutionStatus string

const (
	// OJTaskItemResolutionStatusResolved 表示任务项已绑定到已验证真题。
	OJTaskItemResolutionStatusResolved OJTaskItemResolutionStatus = "resolved"
	// OJTaskItemResolutionStatusPendingResolution 表示任务项仍待题库解析。
	OJTaskItemResolutionStatusPendingResolution OJTaskItemResolutionStatus = "pending_resolution"
	// OJTaskItemResolutionStatusInvalid 表示任务项已确认无法解析为有效题目。
	OJTaskItemResolutionStatusInvalid OJTaskItemResolutionStatus = "invalid"
)

// OJTaskItemInputMode 任务题目输入模式。
type OJTaskItemInputMode string

const (
	// OJTaskItemInputModeTitle 表示任务项通过 platform + title 输入。
	OJTaskItemInputModeTitle OJTaskItemInputMode = "title"
)

const (
	// OJPlatformLuogu 表示洛谷平台标识。
	OJPlatformLuogu = "luogu"
	// OJPlatformLeetcode 表示 LeetCode 平台标识。
	OJPlatformLeetcode = "leetcode"
	// OJPlatformLanqiao 表示蓝桥杯平台标识。
	OJPlatformLanqiao = "lanqiao"
)

const (
	// CapabilityDomainOJTask 是 OJ 任务能力所属的权限域编码。
	CapabilityDomainOJTask = "oj_task"
	// CapabilityGroupCodeOJTaskManagement 是 OJ 任务管理能力组编码。
	CapabilityGroupCodeOJTaskManagement = "oj_task_management"
	// CapabilityGroupNameOJTaskManagement 是 OJ 任务管理能力组显示名。
	CapabilityGroupNameOJTaskManagement = "OJ任务管理"
	// CapabilityCodeOJTaskManage 是 OJ 任务管理能力编码。
	CapabilityCodeOJTaskManage = "oj.task.manage"
)

// OJTaskCapabilitySeeds 返回 OJ 任务相关 capability 定义。
func OJTaskCapabilitySeeds() []CapabilitySeed {
	return []CapabilitySeed{
		{
			Code:      CapabilityCodeOJTaskManage,
			Name:      "管理 OJ 任务",
			Domain:    CapabilityDomainOJTask,
			GroupCode: CapabilityGroupCodeOJTaskManagement,
			GroupName: CapabilityGroupNameOJTaskManagement,
			Desc:      "允许创建、编辑、删除、提前执行、派生与重试 OJ 任务",
		},
	}
}

// OJTaskCapabilityCodes 返回 OJ 任务 capability code 列表副本，避免调用方持有共享底层切片。
func OJTaskCapabilityCodes() []string {
	return []string{CapabilityCodeOJTaskManage}
}

// IsValidOJTaskMode 判断任务创建模式是否属于当前系统允许的枚举值。
func IsValidOJTaskMode(mode string) bool {
	switch OJTaskMode(mode) {
	case OJTaskModeImmediate, OJTaskModeScheduled:
		return true
	default:
		return false
	}
}

// IsValidOJTaskStatus 判断任务版本状态是否属于当前系统允许的枚举值。
func IsValidOJTaskStatus(status string) bool {
	switch OJTaskStatus(status) {
	case OJTaskStatusScheduled,
		OJTaskStatusQueued,
		OJTaskStatusExecuting,
		OJTaskStatusSucceeded,
		OJTaskStatusFailed,
		OJTaskStatusDeleted:
		return true
	default:
		return false
	}
}

// IsValidOJTaskExecutionStatus 判断执行状态是否属于当前系统允许的枚举值。
func IsValidOJTaskExecutionStatus(status string) bool {
	switch OJTaskExecutionStatus(status) {
	case OJTaskExecutionStatusScheduled,
		OJTaskExecutionStatusQueued,
		OJTaskExecutionStatusExecuting,
		OJTaskExecutionStatusSucceeded,
		OJTaskExecutionStatusFailed,
		OJTaskExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

// OJQuestionSourceStatusLabel 将题库来源状态转换成接口侧可读标签。
func OJQuestionSourceStatusLabel(status int8) string {
	switch OJQuestionSourceStatus(status) {
	case OJQuestionSourceStatusVerified:
		return "verified"
	case OJQuestionSourceStatusPending:
		return "pending"
	case OJQuestionSourceStatusInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}
