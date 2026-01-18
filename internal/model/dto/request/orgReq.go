package request

// OrgListReq 组织列表请求参数
type OrgListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`      // 页码，默认1。如果为0或不传，则返回所有数据（不分页）
	PageSize int `form:"page_size" binding:"omitempty,min=1"` // 每页数量，默认10。配合Page使用。
}
