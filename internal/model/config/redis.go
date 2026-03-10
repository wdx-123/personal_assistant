package config

// Redis 缓存数据库配置
type Redis struct {
	Address                         string `json:"address" yaml:"address"`                                                           // Redis 服务器的地址
	Password                        string `json:"password" yaml:"password"`                                                         // 连接 Redis 时的密码，如果没有设置密码则留空
	DB                              int    `json:"db" yaml:"db"`                                                                     // 指定使用的数据库索引，单实例模式下可选择的数据库，默认为 0
	ActiveUserStateTTLSeconds       int    `json:"active_user_state_ttl_seconds" yaml:"active_user_state_ttl_seconds"`               // 用户活跃态缓存基础 TTL，单位秒
	ActiveUserStateTTLJitterSeconds int    `json:"active_user_state_ttl_jitter_seconds" yaml:"active_user_state_ttl_jitter_seconds"` // 用户活跃态缓存抖动窗口，单位秒
}
