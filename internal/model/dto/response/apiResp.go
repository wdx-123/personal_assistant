package response

import "time"

// ApiItem API接口详情（列表项与详情共用）
type ApiItem struct {
	ID         uint       `json:"id"`           // API ID
	Path       string     `json:"path"`         // API路径
	Method     string     `json:"method"`       // 请求方法
	Detail     string     `json:"detail"`       // API描述
	Status     int        `json:"status"`       // 状态：1启用 0禁用
	SyncState  string     `json:"sync_state"`   // 路由同步状态
	LastSeenAt *time.Time `json:"last_seen_at"` // 最近一次扫描命中时间
	MenuID     *uint      `json:"menu_id"`      // 归属菜单ID，未绑定时为null
	MenuName   string     `json:"menu_name"`    // 归属菜单名称，未绑定时为空字符串
	CreatedAt  time.Time  `json:"created_at"`   // 创建时间
	UpdatedAt  time.Time  `json:"updated_at"`   // 更新时间
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
	Added         int `json:"added"`          // 新增API数量
	Restored      int `json:"restored"`       // 恢复为 registered 的API数量
	MarkedMissing int `json:"marked_missing"` // 标记为 missing 的API数量
	Archived      int `json:"archived"`       // 归档数量（当前同步流程固定为0）
	Total         int `json:"total"`          // 当前API总数
}
