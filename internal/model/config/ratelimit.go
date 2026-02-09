package config

// RateLimit 限流配置
type RateLimit struct {
	Upload UploadRateLimit `json:"upload" yaml:"upload"` // 上传接口限流配置
}

// UploadRateLimit 上传接口限流参数
type UploadRateLimit struct {
	GlobalLimit     int `json:"global_limit" yaml:"global_limit"`           // 全局：每窗口最多请求数
	GlobalWindowSec int `json:"global_window_sec" yaml:"global_window_sec"` // 全局窗口大小（秒）
	UserLimit       int `json:"user_limit" yaml:"user_limit"`               // 用户：每窗口最多请求数
	UserWindowSec   int `json:"user_window_sec" yaml:"user_window_sec"`     // 用户窗口大小（秒）
}
