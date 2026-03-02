package system

import (
	"context"
	"errors"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

// JWTGormRepository JWT仓储GORM实现
type JWTGormRepository struct {
	db *gorm.DB
}

// NewJwtRepository 创建JWT仓储实例，返回接口类型
func NewJwtRepository(db *gorm.DB) interfaces.JWTRepository {
	return &JWTGormRepository{db: db}
}

// AddToBlacklist 将token添加到黑名单
func (r *JWTGormRepository) AddToBlacklist(
	ctx context.Context,
	token string,
	expiry time.Time,
) error {
	blacklist := &entity.TokenBlacklist{
		Token:     token,
		ExpiresAt: expiry,
		Reason:    "用户主动登出",
	}
	return r.db.WithContext(ctx).Create(blacklist).Error
}

// IsTokenBlacklisted 检查token是否在黑名单中
func (r *JWTGormRepository) IsTokenBlacklisted(
	ctx context.Context,
	token string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.TokenBlacklist{}).
		Where("token = ? AND expires_at > ?", token, time.Now()).
		Count(&count).Error
	return count > 0, err
}

// CleanExpiredTokens 清理过期的黑名单token
func (r *JWTGormRepository) CleanExpiredTokens(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&entity.TokenBlacklist{}).Error
}

// SaveUserToken 保存用户token记录
func (r *JWTGormRepository) SaveUserToken(
	ctx context.Context,
	userID uint,
	token string,
	expiry time.Time,
) error {
	userToken := &entity.UserToken{
		UserID:    userID,
		Token:     token,
		TokenType: "access",
		ExpiresAt: expiry,
		IsRevoked: false,
	}
	return r.db.WithContext(ctx).Create(userToken).Error
}

// GetUserTokens 获取用户的所有token
func (r *JWTGormRepository) GetUserTokens(
	ctx context.Context,
	userID uint,
) ([]*entity.UserToken, error) {
	var tokens []*entity.UserToken
	err := r.db.WithContext(ctx).Where("user_id = ? AND is_revoked = ? AND expires_at > ?",
		userID, false, time.Now()).Find(&tokens).Error
	return tokens, err
}

// RevokeUserToken 撤销用户的特定token
func (r *JWTGormRepository) RevokeUserToken(
	ctx context.Context,
	userID uint,
	token string,
) error {
	return r.db.WithContext(ctx).Model(&entity.UserToken{}).
		Where("user_id = ? AND token = ?", userID, token).
		Update("is_revoked", true).Error
}

// RevokeAllUserTokens 撤销用户的所有token
func (r *JWTGormRepository) RevokeAllUserTokens(
	ctx context.Context,
	userID uint,
) error {
	return r.db.WithContext(ctx).Model(&entity.UserToken{}).
		Where("user_id = ? AND is_revoked = ?", userID, false).
		Update("is_revoked", true).Error
}

// ValidateToken 验证token是否有效
func (r *JWTGormRepository) ValidateToken(
	ctx context.Context,
	token string,
) (bool, error) {
	// 检查是否在黑名单中
	isBlacklisted, err := r.IsTokenBlacklisted(ctx, token)
	if err != nil {
		return false, err
	}
	if isBlacklisted {
		return false, nil
	}

	// 检查token记录是否存在且未撤销
	var count int64
	err = r.db.WithContext(ctx).Model(&entity.UserToken{}).
		Where("token = ? AND is_revoked = ? AND expires_at > ?",
			token, false, time.Now()).
		Count(&count).Error

	return count > 0, err
}

// GetTokenInfo 获取token信息
func (r *JWTGormRepository) GetTokenInfo(
	ctx context.Context,
	token string,
) (*entity.TokenInfo, error) {
	var userToken entity.UserToken
	err := r.db.WithContext(ctx).Preload("User").
		Where("token = ? AND is_revoked = ? AND expires_at > ?",
			token, false, time.Now()).
		First(&userToken).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	tokenInfo := &entity.TokenInfo{
		UserID:    userToken.UserID,
		Username:  userToken.User.Username,
		TokenType: userToken.TokenType,
		ExpiresAt: userToken.ExpiresAt,
		IsRevoked: userToken.IsRevoked,
	}

	return tokenInfo, nil
}

// UpdateTokenExpiry 更新token过期时间
func (r *JWTGormRepository) UpdateTokenExpiry(
	ctx context.Context,
	token string,
	newExpiry time.Time,
) error {
	return r.db.WithContext(ctx).Model(&entity.UserToken{}).
		Where("token = ?", token).
		Update("expires_at", newExpiry).Error
}

// 兼容现有Service层的方法

// CreateJwtBlacklist 创建JWT黑名单记录（兼容现有Service层）
func (r *JWTGormRepository) CreateJwtBlacklist(
	ctx context.Context,
	jwtList *entity.JwtBlacklist,
) error {
	return r.db.WithContext(ctx).Create(jwtList).Error
}

// IsJwtInBlacklist 检查JWT是否在黑名单中（兼容现有Service层）
func (r *JWTGormRepository) IsJwtInBlacklist(
	ctx context.Context,
	jwt string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.JwtBlacklist{}).
		Where("jwt = ?", jwt).
		Count(&count).Error
	return count > 0, err
}

// GetAllJwtBlacklist 获取所有JWT黑名单（兼容现有Service层）
func (r *JWTGormRepository) GetAllJwtBlacklist(ctx context.Context) ([]string, error) {
	var data []string
	err := r.db.WithContext(ctx).Model(&entity.JwtBlacklist{}).Pluck("jwt", &data).Error
	return data, err
}

// GetUserByID 根据ID获取用户（兼容现有Service层）
func (r *JWTGormRepository) GetUserByID(
	ctx context.Context,
	id uint,
) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
