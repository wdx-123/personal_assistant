package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

// JWTRepository JWT仓储接口
type JWTRepository interface {
	// AddToBlacklist 添加Token到黑名单
	AddToBlacklist(ctx context.Context, token string, expiry time.Time) error
	// IsTokenBlacklisted 检查Token是否在黑名单中
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	// CleanExpiredTokens 清理过期Token
	CleanExpiredTokens(ctx context.Context) error

	// SaveUserToken 保存用户Token记录
	SaveUserToken(ctx context.Context, userID uint, token string, expiry time.Time) error
	// GetUserTokens 获取用户的Token记录列表
	GetUserTokens(ctx context.Context, userID uint) ([]*entity.UserToken, error)
	// RevokeUserToken 撤销用户的指定Token
	RevokeUserToken(ctx context.Context, userID uint, token string) error
	// RevokeAllUserTokens 撤销用户的所有Token
	RevokeAllUserTokens(ctx context.Context, userID uint) error

	// ValidateToken 验证Token有效性
	ValidateToken(ctx context.Context, token string) (bool, error)
	// GetTokenInfo 获取Token详细信息
	GetTokenInfo(ctx context.Context, token string) (*entity.TokenInfo, error)
	// UpdateTokenExpiry 更新Token过期时间
	UpdateTokenExpiry(ctx context.Context, token string, newExpiry time.Time) error

	// CreateJwtBlacklist 创建JWT黑名单记录（兼容旧接口）
	CreateJwtBlacklist(ctx context.Context, jwtList *entity.JwtBlacklist) error
	// IsJwtInBlacklist 检查JWT是否在黑名单中（兼容旧接口）
	IsJwtInBlacklist(ctx context.Context, jwt string) (bool, error)
	// GetAllJwtBlacklist 获取所有黑名单JWT（兼容旧接口）
	GetAllJwtBlacklist(ctx context.Context) ([]string, error)
	// GetUserByID 根据ID获取用户（兼容旧接口）
	GetUserByID(ctx context.Context, id uint) (*entity.User, error)
}
