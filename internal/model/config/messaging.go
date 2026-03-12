package config

// Messaging 消息队列配置
type Messaging struct {
	RedisStreamReadCount int `json:"redis_stream_read_count" yaml:"redis_stream_read_count"` // 每次读取的消息数量
	RedisStreamBlockMs   int `json:"redis_stream_block_ms" yaml:"redis_stream_block_ms"`     // 阻塞等待时间(毫秒)
	// OutboxRelayLockEnabled 是否启用分布式锁来协调 Outbox 转发，防止多实例重复转发
	OutboxRelayLockEnabled bool `json:"outbox_relay_lock_enabled" yaml:"outbox_relay_lock_enabled"`
	// OutboxRelayLockTTLSeconds 分布式锁 TTL，单位秒，需略大于单次转发预估时间，防止死锁
	OutboxRelayLockTTLSeconds int    `json:"outbox_relay_lock_ttl_seconds" yaml:"outbox_relay_lock_ttl_seconds"`
	LuoguBindTopic            string `json:"luogu_bind_topic" yaml:"luogu_bind_topic"`
	LuoguBindGroup            string `json:"luogu_bind_group" yaml:"luogu_bind_group"`
	LuoguBindConsumer         string `json:"luogu_bind_consumer" yaml:"luogu_bind_consumer"`
	LeetcodeBindTopic         string `json:"leetcode_bind_topic" yaml:"leetcode_bind_topic"`
	LeetcodeBindGroup         string `json:"leetcode_bind_group" yaml:"leetcode_bind_group"`
	LeetcodeBindConsumer      string `json:"leetcode_bind_consumer" yaml:"leetcode_bind_consumer"`
	CacheProjectionTopic      string `json:"cache_projection_topic" yaml:"cache_projection_topic"`
	CacheProjectionGroup      string `json:"cache_projection_group" yaml:"cache_projection_group"`
	CacheProjectionConsumer   string `json:"cache_projection_consumer" yaml:"cache_projection_consumer"`
	PermissionProjectionTopic string `json:"permission_projection_topic" yaml:"permission_projection_topic"`
	PermissionProjectionGroup string `json:"permission_projection_group" yaml:"permission_projection_group"`
	PermissionProjectionConsumer string `json:"permission_projection_consumer" yaml:"permission_projection_consumer"`
	PermissionPolicyReloadChannel string `json:"permission_policy_reload_channel" yaml:"permission_policy_reload_channel"`
}
