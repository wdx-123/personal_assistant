package messaging

import (
	"context"
	"time"
)

// Message 定义消息的标准结构
type Message struct {
	ID            string            `json:"id"`             // 消息唯一ID
	Topic         string            `json:"topic"`          // 主题/频道
	Key           string            `json:"key"`            // 分区键 (可选)
	Payload       []byte            `json:"payload"`        // 消息体
	Metadata      map[string]string `json:"metadata"`       // 元数据
	OccurredAt    time.Time         `json:"occurred_at"`    // 发生时间
	PublishedAt   time.Time         `json:"published_at"`   // 发布时间
}

// Publisher 消息发布者接口
type Publisher interface {
	// Publish 发布单条消息
	Publish(ctx context.Context, msg *Message) error
	
	// Close 关闭连接
	Close() error
}

// Subscriber 消息订阅者接口
type Subscriber interface {
	// Subscribe 订阅主题
	// handler: 消息处理函数，返回 error 会触发重试（取决于具体实现）
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	
	// Close 关闭连接
	Close() error
}

// MessageHandler 消息处理函数定义
type MessageHandler func(ctx context.Context, msg *Message) error
