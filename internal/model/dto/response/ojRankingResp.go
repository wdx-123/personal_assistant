package response

type OJRankingListResp struct {
	List   []*OJRankingListItem `json:"list"`
	MyRank *OJRankingMyRank     `json:"my_rank,omitempty"`
	Total  int64                `json:"total"`
}

type OJRankingListItem struct {
	Rank            int                      `json:"rank"`
	UserID          uint                     `json:"user_id"`
	RealName        string                   `json:"real_name"`
	Avatar          string                   `json:"avatar"`
	TotalPassed     int                      `json:"total_passed"`
	PlatformDetails *OJRankingPlatformDetails `json:"platform_details,omitempty"`
}

type OJRankingPlatformDetails struct {
	Luogu    int `json:"luogu,omitempty"`
	Leetcode int `json:"leetcode,omitempty"`
}

type OJRankingMyRank struct {
	Rank        int `json:"rank"`
	TotalPassed int `json:"total_passed"`
}
