package request

type OJRankingListReq struct {
	Page     int    `json:"page" binding:"omitempty,min=1"`
	PageSize int    `json:"page_size" binding:"omitempty,min=1,max=100"`
	Platform string `json:"platform" binding:"omitempty,oneof=leetcode luogu"`
	Scope    string `json:"scope" binding:"omitempty,oneof=current_org all_members org"`
	OrgID    *uint  `json:"org_id" binding:"omitempty,min=1"`
}
