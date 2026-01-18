package response

import "personal_assistant/internal/model/entity"

// OrgListResp 组织列表响应
type OrgListResp struct {
	List  []*entity.Org `json:"list"`  // 组织列表数据
	Total int64         `json:"total"` // 总记录数（仅在分页时有效，全量查询时可能为0或总数）
}
