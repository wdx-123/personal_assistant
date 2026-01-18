package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
	"time"
)

// JWTRepository JWT仓储接口
type JWTRepository interface {
	// Token黑名单管理
	AddToBlacklist(ctx context.Context, token string, expiry time.Time) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	CleanExpiredTokens(ctx context.Context) error

	// 用户Token记录
	SaveUserToken(ctx context.Context, userID uint, token string, expiry time.Time) error
	GetUserTokens(ctx context.Context, userID uint) ([]*entity.UserToken, error)
	RevokeUserToken(ctx context.Context, userID uint, token string) error
	RevokeAllUserTokens(ctx context.Context, userID uint) error

	// Token验证和刷新
	ValidateToken(ctx context.Context, token string) (bool, error)
	GetTokenInfo(ctx context.Context, token string) (*entity.TokenInfo, error)
	UpdateTokenExpiry(ctx context.Context, token string, newExpiry time.Time) error

	// 兼容现有Service层的方法
	CreateJwtBlacklist(ctx context.Context, jwtList *entity.JwtBlacklist) error
	IsJwtInBlacklist(ctx context.Context, jwt string) (bool, error)
	GetAllJwtBlacklist(ctx context.Context) ([]string, error)
	GetUserByID(ctx context.Context, id uint) (*entity.User, error)
}
