package core

import (
	"context"

	"personal_assistant/global"
	"personal_assistant/internal/repository"

	"go.uber.org/zap"
)

// LoadJWTBlacklistWithRepository 启动阶段预热 JWT 黑名单到本地缓存。
func LoadJWTBlacklistWithRepository(ctx context.Context, repositoryGroup *repository.Group) {
	if repositoryGroup == nil || repositoryGroup.SystemRepositorySupplier == nil {
		return
	}
	jwtRepo := repositoryGroup.SystemRepositorySupplier.GetJWTRepository()
	if jwtRepo == nil {
		return
	}
	data, err := jwtRepo.GetAllJwtBlacklist(ctx)
	if err != nil {
		global.Log.Error("failed to load jwt blacklist from repository", zap.Error(err))
		return
	}
	for i := 0; i < len(data); i++ {
		global.BlackCache.SetDefault(data[i], struct{}{})
	}
}
