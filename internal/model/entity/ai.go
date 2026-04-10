package entity

import (
	"time"

	"gorm.io/gorm"
)

// AIConversation 定义当前文件中的核心数据结构或能力抽象。
type AIConversation struct {
	ID            string         `json:"id" gorm:"type:varchar(64);primaryKey;comment:'AI会话ID'"`
	UserID        uint           `json:"user_id" gorm:"index;not null;comment:'所属用户ID'"`
	OrgID         *uint          `json:"org_id,omitempty" gorm:"index;comment:'当前组织ID'"`
	Title         string         `json:"title" gorm:"type:varchar(100);not null;default:'';comment:'会话标题'"`
	Preview       string         `json:"preview" gorm:"type:varchar(500);not null;default:'';comment:'会话预览'"`
	IsGenerating  bool           `json:"is_generating" gorm:"not null;default:false;index;comment:'是否正在生成中'"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty" gorm:"index;comment:'最后消息时间'"`
	CreatedAt     time.Time      `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"type:datetime;not null;comment:'更新时间'"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index;comment:'软删除时间'"`
}

// TableName 返回当前模型在持久化层使用的数据表名。
// 参数：
//   - 无。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (AIConversation) TableName() string {
	return "ai_conversations"
}

// AIMessage 定义当前文件中的核心数据结构或能力抽象。
type AIMessage struct {
	ID             string         `json:"id" gorm:"type:varchar(64);primaryKey;comment:'AI消息ID'"`
	ConversationID string         `json:"conversation_id" gorm:"type:varchar(64);index;not null;comment:'会话ID'"`
	Role           string         `json:"role" gorm:"type:varchar(16);not null;comment:'消息角色'"`
	Content        string         `json:"content" gorm:"type:longtext;not null;comment:'消息正文'"`
	Status         string         `json:"status" gorm:"type:varchar(32);not null;default:'success';comment:'消息状态'"`
	TraceItemsJSON string         `json:"trace_items_json" gorm:"type:longtext;not null;comment:'trace_items JSON'"`
	UIBlocksJSON   string         `json:"ui_blocks_json" gorm:"type:longtext;not null;comment:'ui_blocks JSON'"`
	ScopeJSON      string         `json:"scope_json" gorm:"type:longtext;not null;comment:'scope JSON'"`
	ErrorText      string         `json:"error_text" gorm:"type:varchar(500);not null;default:'';comment:'错误文案'"`
	CreatedAt      time.Time      `json:"created_at" gorm:"type:datetime;not null;index;comment:'创建时间'"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"type:datetime;not null;comment:'更新时间'"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index;comment:'软删除时间'"`
}

// TableName 返回当前模型在持久化层使用的数据表名。
// 参数：
//   - 无。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (AIMessage) TableName() string {
	return "ai_messages"
}

// AIInterrupt 定义当前文件中的核心数据结构或能力抽象。
type AIInterrupt struct {
	InterruptID      string         `json:"interrupt_id" gorm:"type:varchar(64);primaryKey;comment:'Interrupt ID'"`
	ConversationID   string         `json:"conversation_id" gorm:"type:varchar(64);index;not null;comment:'会话ID'"`
	MessageID        string         `json:"message_id" gorm:"type:varchar(64);index;not null;comment:'消息ID'"`
	UserID           uint           `json:"user_id" gorm:"index;not null;comment:'所属用户ID'"`
	Status           string         `json:"status" gorm:"type:varchar(32);not null;index;comment:'中断状态'"`
	ToolKey          string         `json:"tool_key" gorm:"type:varchar(100);not null;default:'';comment:'工具标识'"`
	Decision         string         `json:"decision" gorm:"type:varchar(16);not null;default:'';comment:'用户决策'"`
	Reason           string         `json:"reason" gorm:"type:varchar(500);not null;default:'';comment:'决策原因'"`
	RuntimeStateJSON string         `json:"runtime_state_json" gorm:"type:longtext;not null;comment:'运行时状态 JSON'"`
	OwnerNodeID      string         `json:"owner_node_id" gorm:"type:varchar(128);not null;default:'';comment:'运行归属节点ID'"`
	CreatedAt        time.Time      `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"type:datetime;not null;comment:'更新时间'"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index;comment:'软删除时间'"`
}

// TableName 返回当前模型在持久化层使用的数据表名。
// 参数：
//   - 无。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func (AIInterrupt) TableName() string {
	return "ai_interrupts"
}
