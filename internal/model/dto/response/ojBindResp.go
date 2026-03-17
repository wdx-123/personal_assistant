package response

type BindOJAccountResp struct {
	Platform   string `json:"platform"`
	Identifier string `json:"identifier"`
	RealName   string `json:"real_name"`
	UserAvatar string `json:"user_avatar"`

	// 统一使用 PassedNumber 表示通过的题目总数
	// LeetCode: 对应 TotalNumber (Easy + Medium + Hard)
	// Luogu: 对应 PassedNumber
	PassedNumber int `json:"passed_number"`

	// 蓝桥特有字段：提交成功/失败次数，不代表通过题总数。
	SubmitSuccessCount int `json:"submit_success_count"`
	SubmitFailedCount  int `json:"submit_failed_count"`
}

func (b BindOJAccountResp) ToResponse(input *BindOJAccountResp) *BindOJAccountResp {
	return input
}
