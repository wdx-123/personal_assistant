package config

// Static 静态文件配置
type Static struct {
	Path                 string   `json:"path" yaml:"path"`                                     // 静态文件根目录
	Prefix               string   `json:"prefix" yaml:"prefix"`                                 // URL 前缀
	MaxSize              int      `json:"max_size" yaml:"max_size"`                             // 单文件最大大小（MB）
	MaxUploads           int      `json:"max_uploads" yaml:"max_uploads"`                       // 单次最大上传数量
	AllowedTypes         []string `json:"allowed_types" yaml:"allowed_types"`                   // 允许的文件扩展名列表
	MaxConcurrentUploads int      `json:"max_concurrent_uploads" yaml:"max_concurrent_uploads"` // 最大并发上传数，0 表示不限
	UserQuotaMB          int      `json:"user_quota_mb" yaml:"user_quota_mb"`                   // 单用户最大存储空间（MB），0 表示不限
}
