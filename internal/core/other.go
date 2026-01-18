package core

import (
	"github.com/songzhibin97/gkit/cache/local_cache"
	"go.uber.org/zap"
	"os"
	"personal_assistant/global"
	"personal_assistant/pkg/util"
)

func OtherInit() {
	// 解析刷新令牌
	refreshTokenExpiry, err := util.ParseDuration(global.Config.JWT.RefreshTokenExpiryTime)
	if err != nil {
		global.Log.Error("Failed to parse refresh token expiry time configuration:", zap.Error(err))
		os.Exit(0)
	}

	// 设置本地缓存
	global.BlackCache = local_cache.NewCache(
		local_cache.SetDefaultExpire(refreshTokenExpiry),
	)

}
