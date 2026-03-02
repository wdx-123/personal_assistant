package config

// Email 邮件发送配置结构体
type Email struct {
	Host     string `json:"host" yaml:"host"`         // SMTP 服务器地址
	Port     int    `json:"port" yaml:"port"`         // SMTP 服务器端口号
	From     string `json:"from" yaml:"from"`         // 发件人邮箱地址
	Nickname string `json:"nickname" yaml:"nickname"` // 发件人显示名称
	Secret   string `json:"secret" yaml:"secret"`     // 邮箱授权码或密码
	IsSSL    bool   `json:"is_ssl" yaml:"is_ssl"`     // 是否使用SSL加密连接
}
