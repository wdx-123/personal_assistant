package config

// Upload 文件上传配置结构体
type Upload struct {
	Size int    `json:"size" yaml:"size"` // 文件上传大小限制，单位MB
	Path string `json:"path" yaml:"path"` // 文件上传保存路径
}
