package config

// Crawler 爬虫服务聚合配置
// 包含所有外部数据源（如 LeetCode、Luogu 等）的客户端配置
type Crawler struct {
	LeetCode LeetCodeCrawler `json:"leetcode" yaml:"leetcode"` // 力扣服务配置
	Luogu    LuoguCrawler    `json:"luogu" yaml:"luogu"`       // 洛谷服务配置
}

// LeetCodeCrawler 力扣客户端详细配置
// 对应 configs.yaml 中的 crawler.leetcode 节点
type LeetCodeCrawler struct {
	BaseURL                string `json:"base_url" yaml:"base_url"`                                   // 下游服务地址 (必填)
	TimeoutMs              int    `json:"timeout_ms" yaml:"timeout_ms"`                               // 请求超时 (毫秒)
	MaxIdleConns           int    `json:"max_idle_conns" yaml:"max_idle_conns"`                       // HTTP Client MaxIdleConns
	MaxIdleConnsPerHost    int    `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`     // HTTP Client MaxIdleConnsPerHost (高并发核心参数)
	IdleConnTimeoutSec     int    `json:"idle_conn_timeout_sec" yaml:"idle_conn_timeout_sec"`         // 空闲连接保活时间 (秒)
	RetryCount             int    `json:"retry_count" yaml:"retry_count"`                             // 重试次数
	RetryWaitMs            int    `json:"retry_wait_ms" yaml:"retry_wait_ms"`                         // 重试等待时间 (毫秒)
	RetryMaxWaitMs         int    `json:"retry_max_wait_ms" yaml:"retry_max_wait_ms"`                 // 重试最大等待时间 (毫秒)
	ResponseBodyLimitBytes int64  `json:"response_body_limit_bytes" yaml:"response_body_limit_bytes"` // 响应体大小限制 (字节)
}

// LuoguCrawler 洛谷客户端详细配置
// 对应 configs.yaml 中的 crawler.luogu 节点
// 结构与 LeetCodeCrawler 保持一致，但支持独立配置，便于后续差异化调优
type LuoguCrawler struct {
	BaseURL                string `json:"base_url" yaml:"base_url"`
	TimeoutMs              int    `json:"timeout_ms" yaml:"timeout_ms"`
	MaxIdleConns           int    `json:"max_idle_conns" yaml:"max_idle_conns"`
	MaxIdleConnsPerHost    int    `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"`
	IdleConnTimeoutSec     int    `json:"idle_conn_timeout_sec" yaml:"idle_conn_timeout_sec"`
	RetryCount             int    `json:"retry_count" yaml:"retry_count"`
	RetryWaitMs            int    `json:"retry_wait_ms" yaml:"retry_wait_ms"`
	RetryMaxWaitMs         int    `json:"retry_max_wait_ms" yaml:"retry_max_wait_ms"`
	ResponseBodyLimitBytes int64  `json:"response_body_limit_bytes" yaml:"response_body_limit_bytes"`
}
