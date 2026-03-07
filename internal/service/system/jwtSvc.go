package system

import (
	"context"
	"errors"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	erro "personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/util"

	"github.com/go-redis/redis/v8"

	"github.com/gofrs/uuid"
)

type JWTService struct {
	repositoryGroup *repository.Group
}

func NewJWTService(repositoryGroup *repository.Group) *JWTService {
	return &JWTService{
		repositoryGroup: repositoryGroup,
	}
}

// SetRedisJWT 将JWT存储到Redis中
func (j *JWTService) SetRedisJWT(jwt string, uuid uuid.UUID) error {
	// 解析配置中的JWT过期时间
	dr, err := util.ParseDuration(global.Config.JWT.RefreshTokenExpiryTime)
	if err != nil {
		return err
	}
	// 设置JWT在Redis中的过期时间
	return global.Redis.Set(context.Background(), uuid.String(), jwt, dr).Err()
}

// GetRedisJWT 从Redis中获取JWT
func (j *JWTService) GetRedisJWT(ctx context.Context, uuid uuid.UUID) (string, error) {
	// 从Redis获取指定uuid对应的JWT
	return global.Redis.Get(ctx, uuid.String()).Result()
}

// JoinInBlacklist 将JWT添加到黑名单
func (j *JWTService) JoinInBlacklist(ctx context.Context, jwtList entity.JwtBlacklist) error {
	// 将JWT记录插入到数据库中的黑名单表
	jwtRepo := j.repositoryGroup.SystemRepositorySupplier.GetJWTRepository()
	if err := jwtRepo.CreateJwtBlacklist(ctx, &jwtList); err != nil {
		return err
	}
	// 将JWT添加到内存中的黑名单缓存
	global.BlackCache.SetDefault(jwtList.JWT, struct{}{})
	return nil
}

// IsInBlacklist 检查JWT是否在黑名单中
func (j *JWTService) IsInBlacklist(jwt string) bool {
	// 从黑名单缓存中检查JWT是否存在
	_, ok := global.BlackCache.Get(jwt)
	return ok
}

// GetUserFromJWT 获取用户信息
func (j *JWTService) GetUserFromJWT(ctx context.Context, token string) (user *entity.User, jwtError *erro.JWTError) {
	jwtTool := jwt.NewJWT()
	// 注意：这里解析的是刷新令牌（来自 HttpOnly Cookie 的 x-refresh-token），
	// 因此必须使用 ParseRefreshToken，而不是 ParseAccessToken。
	refreshClaims, err := jwtTool.ParseRefreshToken(token)
	if err != nil {
		return nil, erro.ClassifyJWTError(err)
	}
	// 验证用户是否存在，且未被冻结
	jwtRepo := j.repositoryGroup.SystemRepositorySupplier.GetJWTRepository()
	user, err = jwtRepo.GetUserByID(ctx, refreshClaims.UserID)
	if err != nil {
		return nil, erro.ClassifyJWTError(err)
	}
	if user.Freeze {
		return user, &erro.JWTError{
			Code:    erro.CodeUserFrozen,
			Message: "用户已被冻结",
			Err:     errors.New("user has been frozen"),
		}
	}
	return user, nil
}

// GetAccessToken 获取
func (j *JWTService) GetAccessToken(ctx context.Context, token string) (resR *response.RefreshTokenResponse, jwtError *erro.JWTError) {
	user, jwtErr := j.GetUserFromJWT(ctx, token)
	if jwtErr != nil {
		return nil, jwtErr
	}
	jwtTool := jwt.NewJWT()
	claims := jwtTool.CreateAccessClaims(request.BaseClaims{
		UserID: user.ID,
		UUID:   user.UUID,
		// 注意：移除了RoleID，现在通过权限服务动态获取用户角色
	})
	Token, err := jwtTool.CreateAccessToken(claims)
	if err != nil {
		return nil, &erro.JWTError{
			Code:    erro.CodeInternalError,
			Message: "生成Token失败",
			Err:     errors.New("create token failed"),
		}
	}
	resR = &response.RefreshTokenResponse{
		AccessToken:          Token,
		AccessTokenExpiresAt: claims.ExpiresAt.Unix() * 1000,
	}
	return resR, nil
}

// IssueLoginTokens 登录后签发访问令牌与刷新令牌（支持多点登录管控）
func (j *JWTService) IssueLoginTokens(
	ctx context.Context,
	user entity.User,
) (*response.LoginResponse, string, int64, *erro.JWTError) {
	if user.Freeze {
		return nil, "", 0, &erro.JWTError{
			Code:    erro.CodeUserFrozen,
			Message: "用户已被冻结",
			Err:     errors.New("user frozen"),
		}
	}

	jwtTool := jwt.NewJWT()
	base := request.BaseClaims{UserID: user.ID, UUID: user.UUID}

	// 访问令牌
	accessClaims := jwtTool.CreateAccessClaims(base)
	accessToken, err := jwtTool.CreateAccessToken(accessClaims)
	if err != nil {
		return nil, "", 0, &erro.JWTError{
			Code:    erro.CodeInternalError,
			Message: "生成访问令牌失败",
			Err:     err,
		}
	}

	// 刷新令牌
	refreshClaims := jwtTool.CreateRefreshClaims(base)
	refreshToken, err := jwtTool.CreateRefreshToken(refreshClaims)
	if err != nil {
		return nil, "", 0, &erro.JWTError{
			Code:    erro.CodeInternalError,
			Message: "生成刷新令牌失败",
			Err:     err,
		}
	}

	// 多点登录控制
	if global.Config.System.UseMultipoint {
		// 尝试获取旧的JWT
		old, err := j.GetRedisJWT(ctx, user.UUID)

		if errors.Is(err, redis.Nil) {
			// 无旧记录（Redis中不存在该用户的token），这属于正常情况，直接设置新的刷新令牌
			if err := j.SetRedisJWT(refreshToken, user.UUID); err != nil {
				return nil, "", 0, &erro.JWTError{
					Code:    erro.CodeInternalError,
					Message: "设置登录状态失败",
					Err:     err,
				}
			}
		} else if err != nil {
			// 读取Redis失败（真正的错误，如连接超时、认证失败等）
			return nil, "", 0, &erro.JWTError{
				Code:    erro.CodeInternalError,
				Message: "读取登录状态失败",
				Err:     err,
			}
		} else {
			// 存在旧记录，将旧刷新令牌加入黑名单
			bl := entity.JwtBlacklist{JWT: old}
			if err = j.JoinInBlacklist(ctx, bl); err != nil {
				return nil, "", 0, &erro.JWTError{
					Code:    erro.CodeInternalError,
					Message: "旧令牌加入黑名单失败",
					Err:     err,
				}
			}
			// 写入新令牌
			if err = j.SetRedisJWT(refreshToken, user.UUID); err != nil {
				return nil, "", 0, &erro.JWTError{
					Code:    erro.CodeInternalError,
					Message: "设置登录状态失败",
					Err:     err,
				}
			}
		}
	}

	// 响应封装
	res := &response.LoginResponse{
		User:                 user,
		AccessToken:          accessToken,
		AccessTokenExpiresAt: accessClaims.ExpiresAt.Unix() * 1000,
		RefreshToken:         refreshToken,
	}
	return res, refreshToken, refreshClaims.ExpiresAt.Unix() * 1000, nil
}
