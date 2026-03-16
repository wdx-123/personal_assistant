package readmodel

import "time"

// UserOrgPair 表示一个用户与组织之间的有效成员关系投影。
// 该结构通常来自成员关系过滤查询，只保留后续业务编排所需的最小字段。
type UserOrgPair struct {
	// UserID 是成员用户 ID。
	UserID uint `gorm:"column:user_id"`
	// OrgID 是该用户当前有效归属的组织 ID。
	OrgID uint `gorm:"column:org_id"`
}

// OJTaskOrgInfo 表示任务关联组织的只读投影。
// 该结构由任务表与组织表关联查询产生，用于详情页组织列表展示。
type OJTaskOrgInfo struct {
	// TaskID 是任务版本 ID。
	TaskID uint `gorm:"column:task_id"`
	// OrgID 是任务命中的组织 ID。
	OrgID uint `gorm:"column:org_id"`
	// OrgName 是查询时组织名称快照，用于直接展示。
	OrgName string `gorm:"column:org_name"`
}

// OJTaskListItem 是 OJ 任务列表页使用的聚合读模型。
// 它承载任务版本基础信息以及当前执行进度聚合结果，只用于 Repository 查询结果承接。
type OJTaskListItem struct {
	// TaskID 是当前任务版本 ID。
	TaskID uint `gorm:"column:task_id"`
	// RootTaskID 是版本链根任务 ID；同一版本链中的所有任务共享该值。
	RootTaskID uint `gorm:"column:root_task_id"`
	// ParentTaskID 是父版本任务 ID；根版本为空，派生版本指向其来源版本。
	ParentTaskID *uint `gorm:"column:parent_task_id"`
	// VersionNo 是版本号；根版本为 1，后续每次派生递增。
	VersionNo int `gorm:"column:version_no"`
	// Title 是任务标题。
	Title string `gorm:"column:title"`
	// Description 是任务描述。
	Description string `gorm:"column:description"`
	// Mode 是任务创建模式，取值来自 consts.OJTaskMode。
	Mode string `gorm:"column:mode"`
	// Status 是任务版本当前状态，取值来自 consts.OJTaskStatus。
	Status string `gorm:"column:status"`
	// ExecuteAt 是定时任务计划执行时间；立即任务通常为空。
	ExecuteAt *time.Time `gorm:"column:execute_at"`
	// CreatedBy 是任务版本创建人 ID。
	CreatedBy uint `gorm:"column:created_by"`
	// UpdatedBy 是任务版本最后更新人 ID。
	UpdatedBy uint `gorm:"column:updated_by"`
	// CreatedAt 是任务版本创建时间。
	CreatedAt time.Time `gorm:"column:created_at"`
	// UpdatedAt 是任务版本最后更新时间。
	UpdatedAt time.Time `gorm:"column:updated_at"`
	// ExecutionID 是当前任务版本绑定的唯一执行记录 ID。
	ExecutionID uint `gorm:"column:execution_id"`
	// ExecutionStatus 是当前执行记录状态，取值来自 consts.OJTaskExecutionStatus。
	ExecutionStatus string `gorm:"column:execution_status"`
	// TotalUserCount 是执行快照覆盖的总用户数。
	TotalUserCount int `gorm:"column:total_user_count"`
	// CompletedUserCount 是已完成全部题目的用户数。
	CompletedUserCount int `gorm:"column:completed_user_count"`
	// PendingUserCount 是仍存在未完成题目的用户数。
	PendingUserCount int `gorm:"column:pending_user_count"`
	// TotalItemCount 是执行快照中的总题目数乘用户数后的明细条数。
	TotalItemCount int `gorm:"column:total_item_count"`
	// CompletedItemCount 是已完成的用户题目明细条数。
	CompletedItemCount int `gorm:"column:completed_item_count"`
	// PendingItemCount 是未完成的用户题目明细条数。
	PendingItemCount int `gorm:"column:pending_item_count"`
	// OrgCount 是任务当前关联的组织数量。
	OrgCount int `gorm:"column:org_count"`
	// ItemCount 是任务题单中的题目数量。
	ItemCount int `gorm:"column:item_count"`
}

// OJTaskVisibleTask 是任务详情和执行详情共用的可见任务读模型。
// 它在列表聚合字段的基础上补充执行维度的时间线与错误信息。
type OJTaskVisibleTask struct {
	// OJTaskListItem 复用列表页已查询出的任务基础信息与执行统计字段。
	OJTaskListItem
	// TriggerType 是执行触发来源，取值来自 consts.OJTaskExecutionTriggerType。
	TriggerType string `gorm:"column:trigger_type"`
	// PlannedAt 是执行记录计划触发时间；立即任务通常为创建当时或重试触发时间。
	PlannedAt *time.Time `gorm:"column:planned_at"`
	// StartedAt 是执行实际开始时间；未开始时为空。
	StartedAt *time.Time `gorm:"column:started_at"`
	// FinishedAt 是执行实际结束时间；未结束时为空。
	FinishedAt *time.Time `gorm:"column:finished_at"`
	// ErrorMessage 是执行失败时记录的错误信息；成功或未执行时通常为空。
	ErrorMessage string `gorm:"column:error_message"`
	// RequestedBy 是本次执行的触发用户 ID。
	RequestedBy uint `gorm:"column:requested_by"`
}

// OJTaskVersionItem 表示任务版本链中的单个版本摘要。
// 该结构用于“版本历史”查询，不包含详情页所需的组织与题目明细。
type OJTaskVersionItem struct {
	// TaskID 是当前版本任务 ID。
	TaskID uint `gorm:"column:task_id"`
	// RootTaskID 是所属版本链根任务 ID。
	RootTaskID uint `gorm:"column:root_task_id"`
	// ParentTaskID 是当前版本的直接来源版本 ID。
	ParentTaskID *uint `gorm:"column:parent_task_id"`
	// VersionNo 是当前版本号。
	VersionNo int `gorm:"column:version_no"`
	// Title 是该版本任务标题。
	Title string `gorm:"column:title"`
	// Mode 是该版本任务模式。
	Mode string `gorm:"column:mode"`
	// Status 是该版本任务状态。
	Status string `gorm:"column:status"`
	// ExecuteAt 是该版本的计划执行时间。
	ExecuteAt *time.Time `gorm:"column:execute_at"`
	// CreatedAt 是该版本创建时间。
	CreatedAt time.Time `gorm:"column:created_at"`
	// ExecutionID 是该版本关联的执行记录 ID。
	ExecutionID uint `gorm:"column:execution_id"`
	// ExecutionStatus 是该版本关联执行记录的状态。
	ExecutionStatus string `gorm:"column:execution_status"`
}

// OJTaskExecutionDispatch 是调度器扫描待执行任务时使用的最小读模型。
// 它只暴露调度锁竞争和执行启动所需字段，避免加载无关明细。
type OJTaskExecutionDispatch struct {
	// ExecutionID 是待消费的执行记录 ID。
	ExecutionID uint `gorm:"column:execution_id"`
	// TaskID 是执行记录对应的任务版本 ID。
	TaskID uint `gorm:"column:task_id"`
	// Status 是调度扫描时看到的执行状态。
	Status string `gorm:"column:status"`
	// PlannedAt 是允许开始执行的时间点。
	PlannedAt time.Time `gorm:"column:planned_at"`
}

// OJTaskExecutionUserListItem 是执行用户列表页的分页投影。
// 它聚合用户基础快照和完成进度，供 Service 映射为响应 DTO。
type OJTaskExecutionUserListItem struct {
	// ExecutionUserID 是执行用户快照主键 ID。
	ExecutionUserID uint `gorm:"column:execution_user_id"`
	// ExecutionID 是所属执行记录 ID。
	ExecutionID uint `gorm:"column:execution_id"`
	// UserID 是源用户 ID。
	UserID uint `gorm:"column:user_id"`
	// UserUUIDSnapshot 是执行时冻结的用户 UUID。
	UserUUIDSnapshot string `gorm:"column:user_uuid_snapshot"`
	// UsernameSnapshot 是执行时冻结的用户名。
	UsernameSnapshot string `gorm:"column:username_snapshot"`
	// AvatarSnapshot 是执行时冻结的头像地址。
	AvatarSnapshot string `gorm:"column:avatar_snapshot"`
	// UserStatusSnapshot 是执行时冻结的用户状态值。
	UserStatusSnapshot int8 `gorm:"column:user_status_snapshot"`
	// CompletedItemCount 是该用户在本次执行中已完成题目数。
	CompletedItemCount int `gorm:"column:completed_item_count"`
	// PendingItemCount 是该用户在本次执行中待完成题目数。
	PendingItemCount int `gorm:"column:pending_item_count"`
	// AllCompleted 表示该用户在本次执行中是否已完成全部题目。
	AllCompleted bool `gorm:"column:all_completed"`
}

// OJTaskExecutionUserOrgItem 表示执行命中的单条用户组织快照。
// 一个执行用户可能命中多个组织，因此通常会返回多条记录。
type OJTaskExecutionUserOrgItem struct {
	// ExecutionUserID 是所属执行用户快照 ID。
	ExecutionUserID uint `gorm:"column:execution_user_id"`
	// OrgID 是命中的组织 ID。
	OrgID uint `gorm:"column:org_id"`
	// OrgNameSnapshot 是执行时冻结的组织名称。
	OrgNameSnapshot string `gorm:"column:org_name_snapshot"`
}

// OJTaskExecutionUserItemDetail 表示单个用户在单个题目上的执行结果详情。
// 该结构用于用户执行详情页，保留排序、题目信息和结果原因。
type OJTaskExecutionUserItemDetail struct {
	// ExecutionUserID 是所属执行用户快照 ID。
	ExecutionUserID uint `gorm:"column:execution_user_id"`
	// ExecutionID 是所属执行记录 ID。
	ExecutionID uint `gorm:"column:execution_id"`
	// UserID 是源用户 ID。
	UserID uint `gorm:"column:user_id"`
	// TaskItemID 是任务题单项 ID。
	TaskItemID uint `gorm:"column:task_item_id"`
	// SortNo 是题目在任务题单中的顺序。
	SortNo int `gorm:"column:sort_no"`
	// Platform 是题目所属 OJ 平台标识。
	Platform string `gorm:"column:platform"`
	// QuestionCode 是平台题目编码。
	QuestionCode string `gorm:"column:question_code"`
	// PlatformQuestionID 是本地题库中的题目主键 ID。
	PlatformQuestionID uint `gorm:"column:platform_question_id"`
	// QuestionTitleSnapshot 是执行时冻结的题目标题。
	QuestionTitleSnapshot string `gorm:"column:question_title_snapshot"`
	// ResultStatus 是该用户题目结果状态，取值来自 consts.OJTaskExecutionUserItemResultStatus。
	ResultStatus string `gorm:"column:result_status"`
	// Reason 是 pending 状态下的原因编码；completed 状态通常为空字符串。
	Reason string `gorm:"column:reason"`
}
