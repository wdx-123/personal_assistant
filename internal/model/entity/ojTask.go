package entity

import "time"

// OJTask 表示一个可冻结、可追溯的 OJ 任务版本实体。
// 每次编辑性变更都会产生新的任务版本，同一版本链通过 RootTaskID 串联。
type OJTask struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// RootTaskID 是版本链根任务 ID；根版本初始创建后会回填为自身 ID。
	RootTaskID *uint `json:"root_task_id,omitempty" gorm:"index;comment:'版本链根任务ID'"`
	// ParentTaskID 是直接父版本任务 ID；根版本为空，派生版本指向来源版本。
	ParentTaskID *uint `json:"parent_task_id,omitempty" gorm:"index;comment:'父版本任务ID'"`
	// VersionNo 是版本号；根版本为 1，后续派生版本按链路递增。
	VersionNo int `json:"version_no" gorm:"not null;default:1;comment:'版本号'"`
	// Title 是任务标题。
	Title string `json:"title" gorm:"type:varchar(200);not null;comment:'任务标题'"`
	// Description 是任务描述。
	Description string `json:"description" gorm:"type:text;comment:'任务描述'"`
	// Mode 是任务创建模式，取值来自 consts.OJTaskMode。
	Mode string `json:"mode" gorm:"type:varchar(16);not null;index;comment:'创建模式 immediate|scheduled'"`
	// Status 是任务版本状态，取值来自 consts.OJTaskStatus。
	Status string `json:"status" gorm:"type:varchar(16);not null;index;comment:'任务状态'"`
	// ExecuteAt 是定时任务计划执行时间；立即任务通常为空。
	ExecuteAt *time.Time `json:"execute_at,omitempty" gorm:"type:datetime;index;comment:'计划执行时间'"`
	// CreatedBy 是创建该任务版本的用户 ID。
	CreatedBy uint `json:"created_by" gorm:"not null;index;comment:'创建人ID'"`
	// UpdatedBy 是最后一次修改该任务版本的用户 ID。
	UpdatedBy uint `json:"updated_by" gorm:"not null;index;comment:'更新人ID'"`
}

// OJTaskOrg 表示任务版本与组织之间的关联实体。
// 一条记录表示该任务版本会作用于一个组织范围。
type OJTaskOrg struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// TaskID 是任务版本 ID。
	TaskID uint `json:"task_id" gorm:"not null;index;comment:'任务ID'"`
	// OrgID 是被该任务版本覆盖的组织 ID。
	OrgID uint `json:"org_id" gorm:"not null;index;comment:'组织ID'"`
}

// OJTaskItem 表示任务版本中的单道题目配置。
// 该表保存任务题单快照，后续执行会基于这些配置生成执行明细。
type OJTaskItem struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// TaskID 是所属任务版本 ID。
	TaskID uint `json:"task_id" gorm:"not null;index;comment:'任务ID'"`
	// SortNo 是题目在任务题单中的排序号。
	SortNo int `json:"sort_no" gorm:"not null;default:1;comment:'题目顺序'"`
	// Platform 是 OJ 平台标识，取值来自 OJ 平台常量。
	Platform string `json:"platform" gorm:"type:varchar(16);not null;index;comment:'OJ 平台'"`
	// InputTitle 是用户为该任务项输入的原始题目标题。
	InputTitle string `json:"input_title" gorm:"type:varchar(255);not null;default:'';comment:'用户输入题目标题'"`
	// InputMode 是该任务项的输入模式，目前固定为 title。
	InputMode string `json:"input_mode" gorm:"type:varchar(32);not null;default:'title';comment:'任务题目输入模式'"`
	// ResolvedQuestionID 是当前绑定的本地权威题目 ID，未解析时为 0。
	ResolvedQuestionID uint `json:"resolved_question_id" gorm:"default:0;index;comment:'已解析本地题库ID'"`
	// ResolvedQuestionCode 是当前绑定的题目编码快照，未解析时为空。
	ResolvedQuestionCode string `json:"resolved_question_code" gorm:"type:varchar(64);not null;default:'';comment:'已解析题目编码快照'"`
	// ResolvedTitleSnapshot 是创建或回填时冻结的权威题目标题。
	ResolvedTitleSnapshot string `json:"resolved_title_snapshot" gorm:"type:varchar(255);not null;default:'';comment:'已解析题目标题快照'"`
	// ResolutionStatus 表示题目在任务侧的解析状态。
	ResolutionStatus string `json:"resolution_status" gorm:"type:varchar(32);not null;default:'pending_resolution';index;comment:'任务题目解析状态'"`
	// ResolutionNote 记录题目解析或预检备注。
	ResolutionNote string `json:"resolution_note" gorm:"type:varchar(255);not null;default:'';comment:'任务题目解析备注'"`
}

// OJQuestionIntake 表示任务项待解析/待确认事实。
// 该表只承接任务侧 pending_resolution 状态，不反向污染题库真相。
type OJQuestionIntake struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// TaskID 是所属任务版本 ID。
	TaskID uint `json:"task_id" gorm:"not null;index;comment:'任务ID'"`
	// TaskItemID 是所属任务题目 ID，一对一承接待解析状态。
	TaskItemID uint `json:"task_item_id" gorm:"not null;uniqueIndex;comment:'任务题目ID'"`
	// Platform 是 OJ 平台标识。
	Platform string `json:"platform" gorm:"type:varchar(16);not null;index;comment:'OJ 平台'"`
	// InputTitle 是待解析时冻结的原始输入标题。
	InputTitle string `json:"input_title" gorm:"type:varchar(255);not null;default:'';comment:'待解析输入标题'"`
	// Status 表示当前 intake 的解析状态。
	Status string `json:"status" gorm:"type:varchar(32);not null;default:'pending_resolution';index;comment:'待解析状态'"`
	// ResolvedQuestionID 是已回填的本地权威题目 ID，未解析时为 0。
	ResolvedQuestionID uint `json:"resolved_question_id" gorm:"default:0;index;comment:'已回填本地题库ID'"`
	// ResolutionNote 记录 intake 的解析备注。
	ResolutionNote string `json:"resolution_note" gorm:"type:varchar(255);not null;default:'';comment:'待解析备注'"`
}

// OJTaskExecution 表示任务版本对应的唯一执行记录实体。
// 当前实现约束一个任务版本只对应一条执行记录，状态流转由调度器和 Service 共同维护。
type OJTaskExecution struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// TaskID 是任务版本 ID，并通过唯一索引保证一对一执行关系。
	TaskID uint `json:"task_id" gorm:"not null;uniqueIndex;comment:'任务ID'"`
	// TriggerType 是本次执行的触发来源，取值来自 consts.OJTaskExecutionTriggerType。
	TriggerType string `json:"trigger_type" gorm:"type:varchar(32);not null;comment:'触发类型'"`
	// PlannedAt 是本次执行计划开始时间；立即任务通常为创建或触发当时。
	PlannedAt time.Time `json:"planned_at" gorm:"type:datetime;not null;index;comment:'计划执行时间'"`
	// StartedAt 是实际开始执行时间；未开始时为空。
	StartedAt *time.Time `json:"started_at,omitempty" gorm:"type:datetime;comment:'开始执行时间'"`
	// FinishedAt 是实际完成时间；执行中或未执行时为空。
	FinishedAt *time.Time `json:"finished_at,omitempty" gorm:"type:datetime;comment:'结束执行时间'"`
	// RequestedBy 是发起本次执行的用户 ID。
	RequestedBy uint `json:"requested_by" gorm:"not null;index;comment:'请求执行的用户ID'"`
	// Status 是执行状态，取值来自 consts.OJTaskExecutionStatus。
	Status string `json:"status" gorm:"type:varchar(16);not null;index;comment:'执行状态'"`
	// ErrorMessage 保存执行失败时的诊断信息；成功时通常为空。
	ErrorMessage string `json:"error_message" gorm:"type:text;comment:'错误信息'"`
	// TotalUserCount 是本次执行命中的总用户数。
	TotalUserCount int `json:"total_user_count" gorm:"not null;default:0;comment:'总用户数'"`
	// CompletedUserCount 是本次执行中已完成全部题目的用户数。
	CompletedUserCount int `json:"completed_user_count" gorm:"not null;default:0;comment:'完成用户数'"`
	// PendingUserCount 是本次执行中仍有未完成题目的用户数。
	PendingUserCount int `json:"pending_user_count" gorm:"not null;default:0;comment:'未完成用户数'"`
	// TotalItemCount 是本次执行生成的用户题目明细总数。
	TotalItemCount int `json:"total_item_count" gorm:"not null;default:0;comment:'总题目快照数'"`
	// CompletedItemCount 是本次执行中已完成的用户题目明细数。
	CompletedItemCount int `json:"completed_item_count" gorm:"not null;default:0;comment:'完成题目快照数'"`
	// PendingItemCount 是本次执行中未完成的用户题目明细数。
	PendingItemCount int `json:"pending_item_count" gorm:"not null;default:0;comment:'未完成题目快照数'"`
}

// OJTaskExecutionUser 表示执行过程中冻结的用户快照。
// 它将用户展示信息与执行统计固化下来，避免后续用户资料变更影响历史结果。
type OJTaskExecutionUser struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// ExecutionID 是所属执行记录 ID。
	ExecutionID uint `json:"execution_id" gorm:"not null;index;comment:'执行记录ID'"`
	// UserID 是源用户 ID。
	UserID uint `json:"user_id" gorm:"not null;index;comment:'用户ID'"`
	// UserUUIDSnapshot 是执行时冻结的用户 UUID。
	UserUUIDSnapshot string `json:"user_uuid_snapshot" gorm:"type:char(36);not null;default:'';comment:'用户UUID快照'"`
	// UsernameSnapshot 是执行时冻结的用户名。
	UsernameSnapshot string `json:"username_snapshot" gorm:"type:varchar(50);not null;default:'';comment:'用户名快照'"`
	// AvatarSnapshot 是执行时冻结的头像地址。
	AvatarSnapshot string `json:"avatar_snapshot" gorm:"type:varchar(255);not null;default:'';comment:'头像快照'"`
	// UserStatusSnapshot 是执行时冻结的用户状态值。
	UserStatusSnapshot int8 `json:"user_status_snapshot" gorm:"type:tinyint;not null;default:0;comment:'用户状态快照'"`
	// CompletedItemCount 是该用户已完成题目数。
	CompletedItemCount int `json:"completed_item_count" gorm:"not null;default:0;comment:'完成题数'"`
	// PendingItemCount 是该用户待完成题目数。
	PendingItemCount int `json:"pending_item_count" gorm:"not null;default:0;comment:'未完成题数'"`
	// AllCompleted 表示该用户是否已完成本次任务全部题目。
	AllCompleted bool `json:"all_completed" gorm:"type:boolean;not null;default:false;index;comment:'是否全部完成'"`
}

// OJTaskExecutionUserOrg 表示执行时用户命中的组织快照。
// 该表保留任务命中用户时的组织视图，便于后续审计和结果展示。
type OJTaskExecutionUserOrg struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// ExecutionUserID 是所属执行用户快照 ID。
	ExecutionUserID uint `json:"execution_user_id" gorm:"not null;index;comment:'执行用户快照ID'"`
	// OrgID 是命中的组织 ID。
	OrgID uint `json:"org_id" gorm:"not null;index;comment:'组织ID'"`
	// OrgNameSnapshot 是执行时冻结的组织名称。
	OrgNameSnapshot string `json:"org_name_snapshot" gorm:"type:varchar(100);not null;default:'';comment:'组织名称快照'"`
}

// OJTaskExecutionUserItem 表示用户在单道任务题目上的执行结果快照。
// 它是最终统计、列表展示和重试判定的重要事实表之一。
type OJTaskExecutionUserItem struct {
	// MODEL 提供主键、时间戳和软删除字段。
	MODEL
	// ExecutionID 是所属执行记录 ID。
	ExecutionID uint `json:"execution_id" gorm:"not null;index;comment:'执行记录ID'"`
	// UserID 是源用户 ID。
	UserID uint `json:"user_id" gorm:"not null;index;comment:'用户ID'"`
	// ExecutionUserID 是所属执行用户快照 ID。
	ExecutionUserID uint `json:"execution_user_id" gorm:"not null;index;comment:'执行用户快照ID'"`
	// TaskItemID 是对应的任务题单项 ID。
	TaskItemID uint `json:"task_item_id" gorm:"not null;index;comment:'任务题目ID'"`
	// ResultStatus 是该用户题目结果状态，取值来自 consts.OJTaskExecutionUserItemResultStatus。
	ResultStatus string `json:"result_status" gorm:"type:varchar(16);not null;index;comment:'完成状态'"`
	// Reason 是 pending 状态下的原因编码；completed 状态通常为空字符串。
	Reason string `json:"reason" gorm:"type:varchar(32);not null;default:'';index;comment:'pending 原因'"`
}
