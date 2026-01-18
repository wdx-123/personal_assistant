package luogu

import "fmt"

// getPracticeRequest 获取练习记录请求参数
type getPracticeRequest struct {
	UID      int     `json:"uid"`       // 洛谷用户数字 ID
	SleepSec float64 `json:"sleep_sec"` // 模拟请求延迟秒数
}

// GetPracticeResponse 练习记录响应结构
type GetPracticeResponse struct {
	OK   bool `json:"ok"` // 接口调用状态
	Data struct {
		User struct {
			UID    int    `json:"uid"`    // 用户 UID
			Name   string `json:"name"`   // 用户名
			Avatar string `json:"avatar"` // 头像 URL
		} `json:"user"`
		Passed      []PassedProblem `json:"passed"`       // 已通过题目详情列表
		PassedCount int             `json:"passed_count"` // 通过总数 (冗余字段)
	} `json:"data"`
}

// PassedProblem 单个题目详情
type PassedProblem struct {
	PID        string `json:"pid"`        // 题目 ID (如 P1001)
	Title      string `json:"title"`      // 题目标题
	Difficulty int    `json:"difficulty"` // 难度 (数字表示，具体映射需参考洛谷文档)
	Type       string `json:"type"`       // 题目类型 (如 P, B 等)
}

// RemoteHTTPError 远程服务调用异常
type RemoteHTTPError struct {
	URL        string
	Path       string
	StatusCode int
	Body       string
}

func (e *RemoteHTTPError) Error() string {
	return fmt.Sprintf("luogu remote error: status=%d url=%s", e.StatusCode, e.URL)
}
