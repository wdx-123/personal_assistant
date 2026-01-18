package request

type BindOJAccountReq struct {
	Platform   string `json:"platform" binding:"required,oneof=leetcode luogu"`
	Identifier string `json:"identifier" binding:"required"`
}

