package config

// Security 安全基础设施配置。
type Security struct {
	SensitiveData SensitiveData `json:"sensitive_data" yaml:"sensitive_data"`
}

// SensitiveData 定义敏感数据编解码器配置。
type SensitiveData struct {
	Enabled       bool   `json:"enabled" yaml:"enabled"`                 // 是否启用敏感数据编解码器
	CipherPrefix  string `json:"cipher_prefix" yaml:"cipher_prefix"`     // 密文前缀，默认为 "enc:v1:"
	AESKeyBase64  string `json:"aes_key_base64" yaml:"aes_key_base64"`   // AES密钥，必须是base64编码后的32字节密钥
	HashKeyBase64 string `json:"hash_key_base64" yaml:"hash_key_base64"` // 哈希密钥，必须是base64编码后的32字节密钥
}
