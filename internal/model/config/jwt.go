package config

// JWT JWT认证配置结构体
type JWT struct {
	AccessTokenSecret      string `json:"access_token_secret" yaml:"access_token_secret"`             // 访问令牌密钥，用于签名和验证访问令牌
	RefreshTokenSecret     string `json:"refresh_token_secret" yaml:"refresh_token_secret"`           // 刷新令牌密钥，用于签名和验证刷新令牌
	AccessTokenExpiryTime  string `json:"access_token_expiry_time" yaml:"access_token_expiry_time"`   // 访问令牌过期时间，2小时后需要刷新
	RefreshTokenExpiryTime string `json:"refresh_token_expiry_time" yaml:"refresh_token_expiry_time"` // 刷新令牌过期时间，7天后需要重新登录
	Issuer                 string `json:"issuer" yaml:"issuer"`                                       // JWT 发行者标识，用于标识令牌的发行方
}
