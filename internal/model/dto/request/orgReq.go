package request

// OrgListReq 组织列表请求参数
type OrgListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`      // 页码，默认1。如果为0或不传，则返回所有数据（不分页）
	PageSize int    `form:"page_size" binding:"omitempty,min=1"` // 每页数量，默认10。配合Page使用。
	Keyword  string `form:"keyword"`                             // 按名称模糊搜索
}

// CreateOrgReq 创建组织请求
type CreateOrgReq struct {
	Name        string `json:"name" binding:"required,max=100"`        // 组织名称，必填
	Description string `json:"description" binding:"omitempty,max=255"` // 组织描述，可选
	Code        string `json:"code" binding:"omitempty,max=20"`        // 加入邀请码，可选
}

// UpdateOrgReq 更新组织请求（全部可选，支持部分更新）
type UpdateOrgReq struct {
	Name        *string `json:"name" binding:"omitempty,max=100"`        // 组织名称
	Description *string `json:"description" binding:"omitempty,max=255"` // 组织描述
	Code        *string `json:"code" binding:"omitempty,max=20"`        // 加入邀请码
}

// SetCurrentOrgReq 切换当前组织请求
type SetCurrentOrgReq struct {
	OrgID uint `json:"org_id" binding:"required"` // 组织ID，必填
}
