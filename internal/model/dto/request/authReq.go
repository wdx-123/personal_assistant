package request

import (
	"github.com/gofrs/uuid"
	jwt "github.com/golang-jwt/jwt/v4"
)

/*
	曾经放置包的困惑：
	JWT结构体虽然不直接用于HTTP请求参数绑定，但它们是 认证和授权机制的核心组件 ：
	这些结构体与用户登录、注册等请求密切相关，是HTTP请求处理流程中的重要组成部分。
	将它们放在 request 包中体现了 按功能域划分 的设计思路，而不是严格按照"是否直接用于HTTP参数绑定"来分类。
*/

// BaseClaims 结构体用于存储基本的用户信息，作为JWT的Claim部分
type BaseClaims struct {
	UserID uint      // 用户ID，标识用户唯一性
	UUID   uuid.UUID // 用户的UUID，唯一标识用户
	// 注意：移除RoleID字段，现在可通过权限服务动态获取用户角色
}

// JwtCustomClaims 结构体用于存储JWT的自定义Claims，继承自BaseClaims，并包含标准的JWT注册信息
// 完整用户信息 + JWT标准，安全要求高（短有效期）
type JwtCustomClaims struct {
	BaseClaims           // 基础Claims，包含用户ID、UUID和角色ID
	jwt.RegisteredClaims // 标准JWT声明，例如过期时间、发行者等
}

// JwtCustomRefreshClaims 结构体用于存储刷新Token的自定义Claims，包含用户ID和标准的JWT注册信息
// 仅用户ID + JWT标准，安全要求中（高有效期）
type JwtCustomRefreshClaims struct {
	UserID               uint // 用户ID，用于与刷新Token相关的身份验证
	jwt.RegisteredClaims      // 标准JWT声明
}

type RefreshTokenRequest struct {
	RefreshToken  string `json:"refreshToken"`
	XRefreshToken string `json:"x-refresh-token"`
}
