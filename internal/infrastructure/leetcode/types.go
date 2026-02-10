package leetcode

import "fmt"

// publicProfileRequest 获取公开资料请求参数
type publicProfileRequest struct {
	Username string  `json:"username"`  // 目标用户的 LeetCode 用户名或 slug
	SleepSec float64 `json:"sleep_sec"` // 模拟请求延迟秒数，用于绕过简单的反爬策略
}

// recentACRequest 获取最近通过记录请求参数
type recentACRequest struct {
	Username string `json:"username"`  // 目标用户的 LeetCode 用户名或 slug
	SleepSec int    `json:"sleep_sec"` // 模拟请求延迟秒数
}

// PublicProfileResponse 公开资料响应结构
// 对应 LeetCode 个人主页的核心数据
type PublicProfileResponse struct {
	OK   bool `json:"ok"` // 接口调用状态
	Data struct {
		Profile struct {
			UserSlug   string `json:"userSlug"`   // 用户唯一标识符 (Slug)
			RealName   string `json:"realName"`   // 用户真实姓名（若已公开）
			UserAvatar string `json:"userAvatar"` // 用户头像 URL
		} `json:"profile"`
	} `json:"data"`
}

// submitStatsRequest 提交统计请求参数
type submitStatsRequest struct {
	Username string  `json:"username"`  // 目标用户的 LeetCode 用户名或 slug
	SleepSec float64 `json:"sleep_sec"` // 模拟请求延迟秒数
}

// SubmitStatsResponse 提交统计响应结构
type SubmitStatsResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Stats struct {
			UserProfileUserQuestionProgress struct {
				NumAcceptedQuestions []struct {
					Difficulty string `json:"difficulty"`
					Count      int    `json:"count"`
				} `json:"numAcceptedQuestions"`
			} `json:"userProfileUserQuestionProgress"`
		} `json:"stats"`
	} `json:"data"`
}

// RecentACResponse 最近通过记录响应结构
// 包含用户最近 AC 的题目列表
type RecentACResponse struct {
	OK   bool `json:"ok"` // 接口调用状态
	Data struct {
		RecentAccepted []RecentAcceptedItem `json:"recent_accepted"` // 最近通过题目列表
	} `json:"data"`
}

// RecentAcceptedItem 单条 AC 记录详情
type RecentAcceptedItem struct {
	Title     string `json:"title"`     // 题目标题
	Slug      string `json:"slug"`      // 题目 Slug (用于构建链接)
	Timestamp int64  `json:"timestamp"` // 提交时间戳 (秒)
	Time      string `json:"time"`      // 提交时间 (格式化字符串)
}

// RemoteHTTPError 远程服务调用异常
// 当上游 LeetCode 服务返回非 200 状态码时抛出
type RemoteHTTPError struct {
	URL        string // 请求地址
	Path       string // 请求路径
	StatusCode int    // HTTP 状态码
	Body       string // 响应体片段 (用于调试)
}

func (e *RemoteHTTPError) Error() string {
	return fmt.Sprintf("leetcode remote error: status=%d url=%s", e.StatusCode, e.URL)
}
