package response

type BindOJAccountResp struct {
	Platform   string `json:"platform"`
	Identifier string `json:"identifier"`
	RealName   string `json:"real_name"`
	UserAvatar string `json:"user_avatar"`

	EasyNumber   int `json:"easy_number,omitempty"`
	MediumNumber int `json:"medium_number,omitempty"`
	HardNumber   int `json:"hard_number,omitempty"`
	TotalNumber  int `json:"total_number,omitempty"`

	PassedNumber int `json:"passed_number,omitempty"`
}

func (b BindOJAccountResp) ToResponse(input *BindOJAccountResp) *BindOJAccountResp {
	return input
}
