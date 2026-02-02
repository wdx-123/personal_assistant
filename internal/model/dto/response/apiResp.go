package response

import "time"

// ApiItem API接口详情（列表项与详情共用）
type ApiItem struct {
	ID        uint      `json:"id"`         // API ID
	Path      string    `json:"path"`       // API路径
	Method    string    `json:"method"`     // 请求方法
	Detail    string    `json:"detail"`     // API描述
	GroupID   uint      `json:"group_id"`   // 所属菜单组ID
	Status    int       `json:"status"`     // 状态：1启用 0禁用
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// ApiListResp API分页列表响应
type ApiListResp struct {
	List     []*ApiItem `json:"list"`      // 数据列表
	Total    int64      `json:"total"`     // 总数
	Page     int        `json:"page"`      // 当前页码
	PageSize int        `json:"page_size"` // 每页大小
}

// SyncApiResp 同步API响应
type SyncApiResp struct {
	Added    int `json:"added"`    // 新增API数量
	Updated  int `json:"updated"`  // 更新API数量
	Disabled int `json:"disabled"` // 禁用API数量
	Total    int `json:"total"`    // 当前API总数
}
