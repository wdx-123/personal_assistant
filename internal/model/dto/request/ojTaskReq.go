package request

import "time"

// OJTaskItemReq 单个题目请求项。
type OJTaskItemReq struct {
	Platform     string `json:"platform" binding:"required,oneof=luogu leetcode lanqiao"`
	QuestionCode string `json:"question_code" binding:"required,max=64"`
}

// CreateOJTaskReq 创建任务请求。
type CreateOJTaskReq struct {
	Title       string          `json:"title" binding:"required,max=200"`
	Description string          `json:"description" binding:"omitempty,max=2000"`
	Mode        string          `json:"mode" binding:"required,oneof=immediate scheduled"`
	ExecuteAt   *time.Time      `json:"execute_at"`
	OrgIDs      []uint          `json:"org_ids" binding:"required,min=1,dive,gt=0"`
	Items       []OJTaskItemReq `json:"items" binding:"required,min=1,dive"`
}

// UpdateOJTaskReq 更新未执行的 scheduled 任务版本。
type UpdateOJTaskReq struct {
	Title       string          `json:"title" binding:"required,max=200"`
	Description string          `json:"description" binding:"omitempty,max=2000"`
	Mode        string          `json:"mode" binding:"required,eq=scheduled"`
	ExecuteAt   *time.Time      `json:"execute_at"`
	OrgIDs      []uint          `json:"org_ids" binding:"required,min=1,dive,gt=0"`
	Items       []OJTaskItemReq `json:"items" binding:"required,min=1,dive"`
}

// ReviseOJTaskReq 基于旧版本派生新版本。
type ReviseOJTaskReq struct {
	Title       string          `json:"title" binding:"required,max=200"`
	Description string          `json:"description" binding:"omitempty,max=2000"`
	Mode        string          `json:"mode" binding:"required,oneof=immediate scheduled"`
	ExecuteAt   *time.Time      `json:"execute_at"`
	OrgIDs      []uint          `json:"org_ids" binding:"required,min=1,dive,gt=0"`
	Items       []OJTaskItemReq `json:"items" binding:"required,min=1,dive"`
}

// OJTaskListReq 任务列表查询。
type OJTaskListReq struct {
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=200"`
	Keyword    string `form:"keyword" binding:"omitempty,max=200"`
	OrgID      *uint  `form:"org_id" binding:"omitempty,gt=0"`
	RootTaskID *uint  `form:"root_task_id" binding:"omitempty,gt=0"`
	OnlyLatest *bool  `form:"only_latest"`
	Mode       string `form:"mode" binding:"omitempty,oneof=immediate scheduled"`
	Status     string `form:"status" binding:"omitempty,oneof=scheduled queued executing succeeded failed deleted"`
}

// OJTaskVersionListReq 版本列表查询。
type OJTaskVersionListReq struct{}

// OJTaskExecutionUserListReq 执行用户分页查询。
type OJTaskExecutionUserListReq struct {
	Page         int    `form:"page" binding:"omitempty,min=1"`
	PageSize     int    `form:"page_size" binding:"omitempty,min=1,max=200"`
	AllCompleted *bool  `form:"all_completed"`
	Username     string `form:"username" binding:"omitempty,max=50"`
}
