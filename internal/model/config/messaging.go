package config

// Messaging 消息队列配置
type Messaging struct {
	RedisStreamReadCount int    `json:"redis_stream_read_count" yaml:"redis_stream_read_count"` // 每次读取的消息数量
	RedisStreamBlockMs   int    `json:"redis_stream_block_ms" yaml:"redis_stream_block_ms"`     // 阻塞等待时间(毫秒)
	LuoguBindTopic       string `json:"luogu_bind_topic" yaml:"luogu_bind_topic"`
	LuoguBindGroup       string `json:"luogu_bind_group" yaml:"luogu_bind_group"`
	LuoguBindConsumer    string `json:"luogu_bind_consumer" yaml:"luogu_bind_consumer"`
	LeetcodeBindTopic    string `json:"leetcode_bind_topic" yaml:"leetcode_bind_topic"`
	LeetcodeBindGroup    string `json:"leetcode_bind_group" yaml:"leetcode_bind_group"`
	LeetcodeBindConsumer string `json:"leetcode_bind_consumer" yaml:"leetcode_bind_consumer"`
}
