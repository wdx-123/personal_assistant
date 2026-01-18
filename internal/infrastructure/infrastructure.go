package infrastructure

import (
	"personal_assistant/global"
	"personal_assistant/internal/infrastructure/leetcode"
	"personal_assistant/internal/infrastructure/luogu"
)

var (
	// leetCodeClient 全局单例 LeetCode 客户端
	// 使用 sync.Once 或在 Init 阶段保证并发安全初始化
	leetCodeClient *leetcode.Client

	// luoguClient 全局单例 Luogu 客户端
	luoguClient *luogu.Client
)

// Init 初始化基础设施层的所有外部服务客户端
// 该函数应在应用启动阶段（main 或 server 启动前）被调用
// 依赖 global.Config 已加载完成
func Init() {
	// 1. 初始化 LeetCode 客户端
	// 从全局配置中读取 crawler.leetcode 配置项
	leetcodeCfg := global.Config.Crawler.LeetCode

	// 创建客户端实例，注入全局 Logger
	lc, err := leetcode.NewFromConfig(leetcodeCfg, leetcode.WithLogger(global.Log))
	if err != nil {
		// 基础设施初始化失败属于致命错误，直接 Panic 阻断启动
		global.Log.Panic("failed to init leetcode client: " + err.Error())
	}
	leetCodeClient = lc

	// 2. 初始化 Luogu 客户端
	luoguCfg := global.Config.Crawler.Luogu
	lg, err := luogu.NewFromConfig(luoguCfg, luogu.WithLogger(global.Log))
	if err != nil {
		global.Log.Panic("failed to init luogu client: " + err.Error())
	}
	luoguClient = lg
}

// LeetCode 获取全局单例的 LeetCode 客户端
// 供 Service 层或 Task 层调用，执行 PublicProfile、RecentAC 等操作
func LeetCode() *leetcode.Client {
	return leetCodeClient
}

// Luogu 获取全局单例的 Luogu 客户端
// 供 Service 层或 Task 层调用，执行 GetPractice 等操作
func Luogu() *luogu.Client {
	return luoguClient
}
