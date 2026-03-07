package lanqiao

import "fmt"

type solveStatsRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	SyncNum  int    `json:"sync_num"`
}

// SolveStatsResponse 蓝桥做题统计响应。
// data 下的 stats/problems 按 sync_num 不同模式可选返回。
type SolveStatsResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Stats    *SolveStatsStats    `json:"stats,omitempty"`
		Problems []SolveStatsProblem `json:"problems,omitempty"`
	} `json:"data"`
}

type SolveStatsStats struct {
	TotalPassed int `json:"total_passed"`
	TotalFailed int `json:"total_failed"`
}

type SolveStatsProblem struct {
	ProblemName string `json:"problem_name"`
	ProblemID   int    `json:"problem_id"`
	CreatedAt   string `json:"created_at"`
	IsPassed    bool   `json:"is_passed"`
}

type RemoteHTTPError struct {
	URL        string
	Path       string
	StatusCode int
	Body       string
	Message    string
}

func (e *RemoteHTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("lanqiao remote error: %s (status=%d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("lanqiao remote error: status=%d url=%s", e.StatusCode, e.URL)
}
