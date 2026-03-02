package config

// Gaode 高德地图API配置结构体
type Gaode struct {
	Enable bool   `json:"enable" yaml:"enable"` // 是否启用高德地图功能
	Key    string `json:"key" yaml:"key"`       // 高德地图API密钥
}
