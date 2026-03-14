package request

type OJCurveReq struct {
	Platform string `json:"platform" binding:"required,oneof=leetcode luogu lanqiao"`
}
