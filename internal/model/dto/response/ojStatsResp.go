package response

// LanqiaoSyncAlertResp 描述蓝桥同步状态提醒，仅在蓝桥统计查询中有业务含义。
type LanqiaoSyncAlertResp struct {
	Danger           bool   `json:"danger"`
	Reason           string `json:"reason"`
	Message          string `json:"message"`
	FailureThreshold int    `json:"failure_threshold"`
	SyncDisabled     bool   `json:"sync_disabled"`
}

// OJStatsResp 是 /oj/stats 的专用响应体，避免和绑定接口共用 DTO。
type OJStatsResp struct {
	Platform   string `json:"platform"`
	Identifier string `json:"identifier"`
	RealName   string `json:"real_name"`
	UserAvatar string `json:"user_avatar"`

	PassedNumber int `json:"passed_number"`

	// 蓝桥特有字段：提交成功/失败次数，不代表通过题总数。
	SubmitSuccessCount int `json:"submit_success_count"`
	SubmitFailedCount  int `json:"submit_failed_count"`

	LanqiaoSyncAlert *LanqiaoSyncAlertResp `json:"lanqiao_sync_alert,omitempty"`
}

func (o OJStatsResp) ToResponse(input *OJStatsResp) *OJStatsResp {
	return input
}
