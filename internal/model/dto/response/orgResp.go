package response

// OrgItem 组织信息项（用于列表/详情响应）
type OrgItem struct {
	ID          uint   `json:"id"`          // 组织ID
	Name        string `json:"name"`        // 组织名称
	Description string `json:"description"` // 组织描述
	Code        string `json:"code"`        // 加入邀请码
	OwnerID     uint   `json:"owner_id"`    // 创建者/负责人ID
	CreatedAt   string `json:"created_at"`  // 创建时间
	UpdatedAt   string `json:"updated_at"`  // 更新时间
}

// OrgListResp 组织列表响应
type OrgListResp struct {
	List  []*OrgItem `json:"list"`  // 组织列表数据
	Total int64      `json:"total"` // 总记录数
}

// MyOrgItem 我的组织信息项（用于"获取我的组织"响应）
type MyOrgItem struct {
	ID      uint   `json:"id"`       // 组织ID
	Name    string `json:"name"`     // 组织名称
	IsOwner bool   `json:"is_owner"` // 当前用户是否为组织所有者
}
