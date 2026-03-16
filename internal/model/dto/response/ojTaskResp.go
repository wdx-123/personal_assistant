package response

// OJTaskOrgItemResp 任务关联组织响应项。
type OJTaskOrgItemResp struct {
	OrgID   uint   `json:"org_id"`
	OrgName string `json:"org_name"`
}

// OJTaskItemResp 任务题目响应项。
type OJTaskItemResp struct {
	ID                    uint   `json:"id"`
	SortNo                int    `json:"sort_no"`
	Platform              string `json:"platform"`
	QuestionCode          string `json:"question_code"`
	PlatformQuestionID    uint   `json:"platform_question_id"`
	QuestionTitleSnapshot string `json:"question_title_snapshot"`
}

// OJTaskListItemResp 任务列表项。
type OJTaskListItemResp struct {
	TaskID             uint   `json:"task_id"`
	RootTaskID         uint   `json:"root_task_id"`
	ParentTaskID       *uint  `json:"parent_task_id,omitempty"`
	VersionNo          int    `json:"version_no"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	Mode               string `json:"mode"`
	Status             string `json:"status"`
	ExecuteAt          string `json:"execute_at,omitempty"`
	CreatedBy          uint   `json:"created_by"`
	UpdatedBy          uint   `json:"updated_by"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	ExecutionID        uint   `json:"execution_id"`
	ExecutionStatus    string `json:"execution_status"`
	TotalUserCount     int    `json:"total_user_count"`
	CompletedUserCount int    `json:"completed_user_count"`
	PendingUserCount   int    `json:"pending_user_count"`
	TotalItemCount     int    `json:"total_item_count"`
	CompletedItemCount int    `json:"completed_item_count"`
	PendingItemCount   int    `json:"pending_item_count"`
	OrgCount           int    `json:"org_count"`
	ItemCount          int    `json:"item_count"`
}

// OJTaskCreateResp 创建/派生/重试返回。
type OJTaskCreateResp struct {
	TaskID      uint   `json:"task_id"`
	ExecutionID uint   `json:"execution_id"`
	Status      string `json:"status"`
}

// OJTaskExecutionResp 执行详情响应。
type OJTaskExecutionResp struct {
	ExecutionID        uint   `json:"execution_id"`
	TaskID             uint   `json:"task_id"`
	TriggerType        string `json:"trigger_type"`
	RequestedBy        uint   `json:"requested_by"`
	Status             string `json:"status"`
	PlannedAt          string `json:"planned_at"`
	StartedAt          string `json:"started_at,omitempty"`
	FinishedAt         string `json:"finished_at,omitempty"`
	ErrorMessage       string `json:"error_message,omitempty"`
	TotalUserCount     int    `json:"total_user_count"`
	CompletedUserCount int    `json:"completed_user_count"`
	PendingUserCount   int    `json:"pending_user_count"`
	TotalItemCount     int    `json:"total_item_count"`
	CompletedItemCount int    `json:"completed_item_count"`
	PendingItemCount   int    `json:"pending_item_count"`
}

// OJTaskDetailResp 任务详情响应。
type OJTaskDetailResp struct {
	TaskID           uint                 `json:"task_id"`
	RootTaskID       uint                 `json:"root_task_id"`
	ParentTaskID     *uint                `json:"parent_task_id,omitempty"`
	VersionNo        int                  `json:"version_no"`
	Title            string               `json:"title"`
	Description      string               `json:"description"`
	Mode             string               `json:"mode"`
	Status           string               `json:"status"`
	ExecuteAt        string               `json:"execute_at,omitempty"`
	CreatedBy        uint                 `json:"created_by"`
	UpdatedBy        uint                 `json:"updated_by"`
	CreatedAt        string               `json:"created_at"`
	UpdatedAt        string               `json:"updated_at"`
	Orgs             []*OJTaskOrgItemResp `json:"orgs"`
	Items            []*OJTaskItemResp    `json:"items"`
	CurrentExecution *OJTaskExecutionResp `json:"current_execution,omitempty"`
}

// OJTaskVersionItemResp 版本链项。
type OJTaskVersionItemResp struct {
	TaskID          uint   `json:"task_id"`
	RootTaskID      uint   `json:"root_task_id"`
	ParentTaskID    *uint  `json:"parent_task_id,omitempty"`
	VersionNo       int    `json:"version_no"`
	Title           string `json:"title"`
	Mode            string `json:"mode"`
	Status          string `json:"status"`
	ExecuteAt       string `json:"execute_at,omitempty"`
	CreatedAt       string `json:"created_at"`
	ExecutionID     uint   `json:"execution_id"`
	ExecutionStatus string `json:"execution_status"`
}

// OJTaskVersionListResp 版本链响应。
type OJTaskVersionListResp struct {
	RootTaskID uint                     `json:"root_task_id"`
	Versions   []*OJTaskVersionItemResp `json:"versions"`
}

// OJTaskExecutionUserOrgResp 执行用户命中的组织快照。
type OJTaskExecutionUserOrgResp struct {
	OrgID           uint   `json:"org_id"`
	OrgNameSnapshot string `json:"org_name_snapshot"`
}

// OJTaskExecutionUserItemResp 执行用户题目结果。
type OJTaskExecutionUserItemResp struct {
	TaskItemID            uint   `json:"task_item_id"`
	SortNo                int    `json:"sort_no"`
	Platform              string `json:"platform"`
	QuestionCode          string `json:"question_code"`
	PlatformQuestionID    uint   `json:"platform_question_id"`
	QuestionTitleSnapshot string `json:"question_title_snapshot"`
	ResultStatus          string `json:"result_status"`
	Reason                string `json:"reason,omitempty"`
}

// OJTaskExecutionUserSummaryResp 执行用户分页项。
type OJTaskExecutionUserSummaryResp struct {
	UserID             uint                          `json:"user_id"`
	UserUUIDSnapshot   string                        `json:"user_uuid_snapshot"`
	UsernameSnapshot   string                        `json:"username_snapshot"`
	AvatarSnapshot     string                        `json:"avatar_snapshot"`
	UserStatusSnapshot int8                          `json:"user_status_snapshot"`
	CompletedItemCount int                           `json:"completed_item_count"`
	PendingItemCount   int                           `json:"pending_item_count"`
	AllCompleted       bool                          `json:"all_completed"`
	Orgs               []*OJTaskExecutionUserOrgResp `json:"orgs"`
}

// OJTaskExecutionUserListResp 执行用户分页响应。
type OJTaskExecutionUserListResp struct {
	List     []*OJTaskExecutionUserSummaryResp `json:"list"`
	Total    int64                             `json:"total"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"page_size"`
}

// OJTaskExecutionUserDetailResp 单用户快照详情。
type OJTaskExecutionUserDetailResp struct {
	UserID             uint                           `json:"user_id"`
	UserUUIDSnapshot   string                         `json:"user_uuid_snapshot"`
	UsernameSnapshot   string                         `json:"username_snapshot"`
	AvatarSnapshot     string                         `json:"avatar_snapshot"`
	UserStatusSnapshot int8                           `json:"user_status_snapshot"`
	CompletedItemCount int                            `json:"completed_item_count"`
	PendingItemCount   int                            `json:"pending_item_count"`
	AllCompleted       bool                           `json:"all_completed"`
	Orgs               []*OJTaskExecutionUserOrgResp  `json:"orgs"`
	CompletedItems     []*OJTaskExecutionUserItemResp `json:"completed_items"`
	PendingItems       []*OJTaskExecutionUserItemResp `json:"pending_items"`
}
