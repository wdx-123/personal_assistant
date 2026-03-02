package entity

import "time"

// UserToken 用户Token记录表 - 管理用户的访问令牌和刷新令牌，支持令牌撤销和追踪
type UserToken struct {
	MODEL
	UserID    uint      `json:"user_id" gorm:"type:bigint unsigned;not null;index;comment:'关联用户ID'"`                  // 关联的用户ID，外键引用users表
	User      User      `json:"user" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;comment:'关联用户信息'"`           // 关联的用户实体，级联删除
	Token     string    `json:"token" gorm:"type:varchar(512);unique;not null;comment:'Token值'"`                      // JWT Token的完整字符串
	TokenType string    `json:"token_type" gorm:"type:varchar(20);not null;default:'access';index;comment:'Token类型'"` // Token类型：access访问令牌, refresh刷新令牌
	ExpiresAt time.Time `json:"expires_at" gorm:"type:datetime;not null;index;comment:'Token过期时间'"`                   // Token的过期时间，用于清理过期令牌
	IsRevoked bool      `json:"is_revoked" gorm:"type:boolean;not null;default:false;index;comment:'是否已撤销'"`          // Token是否已被撤销（主动登出、安全原因等）
	IP        string    `json:"ip" gorm:"type:varchar(45);default:'';comment:'签发IP地址'"`                               // Token签发时的客户端IP地址
	UserAgent string    `json:"user_agent" gorm:"type:varchar(500);default:'';comment:'用户代理信息'"`                      // Token签发时的用户代理字符串
}

// TokenBlacklist Token黑名单表 - 存储被主动撤销或需要禁用的Token
type TokenBlacklist struct {
	MODEL
	Token     string    `json:"token" gorm:"type:varchar(512);unique;not null;comment:'被禁用的Token值'"`  // 被加入黑名单的Token完整字符串
	ExpiresAt time.Time `json:"expires_at" gorm:"type:datetime;not null;index;comment:'Token原始过期时间'"` // Token的原始过期时间，用于定期清理黑名单
	Reason    string    `json:"reason" gorm:"type:varchar(200);default:'';comment:'加入黑名单的原因'"`        // 加入黑名单的原因：logout/security/admin等
}

// TokenInfo Token信息结构体（用于返回）
type TokenInfo struct {
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	TokenType string    `json:"token_type"`
	ExpiresAt time.Time `json:"expires_at"`
	IsRevoked bool      `json:"is_revoked"`
}
