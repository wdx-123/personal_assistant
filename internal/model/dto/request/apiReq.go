package request

// ApiListReq API列表请求参数
// 分页获取系统注册的API接口列表，支持按状态、分组、方法、关键词过滤
type ApiListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`       // 页码，默认1
	PageSize int    `form:"page_size" binding:"omitempty,min=1"`  // 每页数量，默认10
	Status   *int   `form:"status"`                               // 状态过滤：1启用 0禁用
	GroupID  *uint  `form:"group_id"`                             // 所属菜单组ID
	Method   string `form:"method"`                               // 请求方法过滤：GET/POST/PUT/DELETE/PATCH
	Keyword  string `form:"keyword"`                              // 按路径或描述模糊搜索
}

// ApiListFilter API列表查询过滤条件（供 Repository 层使用）
type ApiListFilter struct {
	Page     int    // 页码，默认1
	PageSize int    // 每页数量，默认10
	Status   *int   // 状态过滤：1启用 0禁用
	GroupID  *uint  // 所属菜单组ID
	Method   string // 请求方法过滤
	Keyword  string // 按路径或描述模糊搜索
}

// CreateApiReq 创建API请求
type CreateApiReq struct {
	Path    string `json:"path" binding:"required"`                                   // API路径
	Method  string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH"` // 请求方法
	Detail  string `json:"detail"`                                                    // API描述
	GroupID uint   `json:"group_id"`                                                  // 所属菜单组ID
	Status  int    `json:"status"`                                                    // 状态：1启用 0禁用，默认1
}

// UpdateApiReq 更新API请求
type UpdateApiReq struct {
	Path    *string `json:"path"`     // API路径
	Method  *string `json:"method"`   // 请求方法：GET/POST/PUT/DELETE/PATCH
	Detail  *string `json:"detail"`   // API描述
	GroupID *uint   `json:"group_id"` // 所属菜单组ID
	Status  *int    `json:"status"`   // 状态：1启用 0禁用
}

// SyncApiReq 同步路由到API表请求
type SyncApiReq struct {
	DeleteRemoved bool `json:"delete_removed"` // 是否删除已移除的路由（默认仅禁用）
}
