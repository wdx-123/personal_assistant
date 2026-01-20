package request

type OJStatsReq struct {
	Platform string `json:"platform" binding:"required,oneof=leetcode luogu"`
}
