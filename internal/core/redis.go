package core

import (
	"context"
	"os"
	"time"

	"personal_assistant/global"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// ConnectRedis 初始化并返回一个 Redis 客户端，支持集群或单节点配置
func ConnectRedis() *redis.Client {
	redisCfg := global.Config.Redis

	// 使用单节点配置创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Address,  // 设置 Redis 服务器地址
		Password: redisCfg.Password, // 设置 Redis 密码
		DB:       redisCfg.DB,       // 设置使用的数据库索引，0，默认数据库
	})

	// Ping Redis 服务器以检查连接是否正常
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		global.Log.Error("Failed to connect to Redis:", zap.Error(err))
		os.Exit(1)
	}

	return client
}
