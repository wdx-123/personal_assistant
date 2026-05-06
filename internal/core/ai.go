package core

import (
	"context"
	"strings"

	"personal_assistant/global"
	infraeino "personal_assistant/internal/infrastructure/ai/eino"
	infralocal "personal_assistant/internal/infrastructure/ai/local"
	streamsse "personal_assistant/internal/infrastructure/sse"

	"go.uber.org/zap"
)

// InitAI 初始化项目级 AI runtime。
// 参数：无。
//
// 返回值：无。
//
// 核心流程：
//  1. 读取 SSE 基础设施中的连接策略，用于统一 heartbeat 默认值。
//  2. 先创建 local runtime 并写入全局兜底实例。
//  3. 根据 `sse.ai_runtime_mode` 判断是否启用 Eino。
//  4. Eino 初始化成功后替换全局 runtime；失败时继续使用 local。
//
// 注意事项：
//   - 这里属于初始化/装配层，可以读取 global.Config。
//   - Service 层只依赖 global.AIRuntime 注入后的 domain/ai.Runtime，不直接依赖 Eino SDK。
func InitAI() {
	policy := streamsse.ConnectionPolicy{}
	if global.StreamInfra != nil {
		policy = global.StreamInfra.Policy
	}
	policy = policy.Normalize()

	localRuntime := infralocal.NewRuntime(policy.HeartbeatInterval)
	global.AIRuntime = localRuntime

	if global.Config == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(global.Config.SSE.AIRuntimeMode))
	if mode == "" || mode == "local" {
		return
	}
	if mode != "eino" {
		global.Log.Warn("未知 AI runtime，回退到 local", zap.String("mode", mode))
		return
	}

	runtime, err := infraeino.NewRuntime(context.Background(), infraeino.Options{
		Provider:            global.Config.AI.Provider,
		APIKey:              global.Config.AI.APIKey,
		BaseURL:             global.Config.AI.BaseURL,
		Model:               global.Config.AI.Model,
		ByAzure:             global.Config.AI.ByAzure,
		APIVersion:          global.Config.AI.APIVersion,
		SystemPrompt:        global.Config.AI.SystemPrompt,
		Temperature:         global.Config.AI.Temperature,
		MaxCompletionTokens: global.Config.AI.MaxCompletionTokens,
		HeartbeatInterval:   policy.HeartbeatInterval,
	})
	if err != nil {
		global.Log.Warn("Eino runtime 初始化失败，回退到 local", zap.Error(err))
		return
	}
	global.AIRuntime = runtime
}
