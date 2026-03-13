package response

import "time"

type OJCurveResp struct {
	Platform     string          `json:"platform"`
	Bound        bool            `json:"bound"`
	CurrentTotal int             `json:"current_total"`
	LastSyncAt   *time.Time      `json:"last_sync_at,omitempty"`
	Points       []*OJCurvePoint `json:"points"`
}

type OJCurvePoint struct {
	Date        string `json:"date"`
	SolvedCount int    `json:"solved_count"`
	SolvedTotal int    `json:"solved_total"`
}
