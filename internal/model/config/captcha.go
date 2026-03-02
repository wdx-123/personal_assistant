package config

// Captcha 验证码配置结构体
type Captcha struct {
	Height   int     `json:"height" yaml:"height"`       // 验证码图片高度，单位像素
	Width    int     `json:"width" yaml:"width"`         // 验证码图片宽度，单位像素
	Length   int     `json:"length" yaml:"length"`       // 验证码字符长度
	MaxSkew  float64 `json:"max_skew" yaml:"max_skew"`   // 字符最大倾斜度，用于增加识别难度
	DotCount int     `json:"dot_count" yaml:"dot_count"` // 干扰点数量，用于防止机器识别
}
