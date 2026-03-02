package config

// Storage 存储驱动配置
type Storage struct {
	Current string       `json:"current" yaml:"current"` // 当前驱动：local 或 qiniu
	Local   StorageLocal `json:"local" yaml:"local"`     // 本地存储配置
	Qiniu   StorageQiniu `json:"qiniu" yaml:"qiniu"`     // 七牛云存储配置
}

// StorageLocal 本地存储配置
type StorageLocal struct {
	BaseURL   string `json:"base_url" yaml:"base_url"`     // 访问基础URL（为空时复用 system.host + static.prefix）
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix"` // 对象键前缀（可选）
}

// StorageQiniu 七牛云存储配置
type StorageQiniu struct {
	Bucket    string `json:"bucket" yaml:"bucket"`         // 七牛空间名
	Domain    string `json:"domain" yaml:"domain"`         // 访问域名
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix"` // 对象键前缀（可选）
	AccessKey string `json:"access_key" yaml:"access_key"` // AK（可使用环境变量覆盖）
	SecretKey string `json:"secret_key" yaml:"secret_key"` // SK（可使用环境变量覆盖）
}
