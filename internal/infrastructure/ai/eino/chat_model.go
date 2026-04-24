package eino

import (
	"context"
	"fmt"
	"strings"

	arkeino "github.com/cloudwego/eino-ext/components/model/ark"
	openaieino "github.com/cloudwego/eino-ext/components/model/openai"
	qweneino "github.com/cloudwego/eino-ext/components/model/qwen"
	einomodel "github.com/cloudwego/eino/components/model"
)

// NewChatModel 根据配置创建 Eino ChatModel。
// 参数：
//   - ctx：初始化上下文。
//   - cfg：模型 provider、APIKey、模型名、baseURL 等配置。
//
// 返回值：
//   - einomodel.BaseChatModel：Eino 标准聊天模型接口。
//   - error：配置缺失或 provider 不支持时返回错误。
//
// 核心流程：
//  1. 归一化 provider，默认使用 qwen。
//  2. 校验 APIKey 和 model 两个必填参数。
//  3. 根据 provider 创建 Qwen、OpenAI 或 Ark 模型。
//
// 注意事项：
//   - 本函数不读取全局配置，调用方必须显式传入 Options。
func NewChatModel(ctx context.Context, cfg Options) (einomodel.BaseChatModel, error) {
	if cfg.ChatModelFactory != nil {
		// 自定义工厂完全接管模型创建流程，便于未来接入模型网关或路由层。
		return cfg.ChatModelFactory(ctx, cfg)
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "qwen"
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("ai api key is empty")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("ai model is empty")
	}

	temperature := float32(cfg.Temperature)
	maxCompletionTokens := cfg.MaxCompletionTokens

	switch provider {
	case "qwen":
		baseURL := strings.TrimSpace(cfg.BaseURL)
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		return qweneino.NewChatModel(ctx, &qweneino.ChatModelConfig{
			APIKey:      cfg.APIKey,
			BaseURL:     baseURL,
			Model:       cfg.Model,
			Temperature: &temperature,
			MaxTokens:   &maxCompletionTokens,
		})
	case "openai":
		return openaieino.NewChatModel(ctx, &openaieino.ChatModelConfig{
			APIKey:              cfg.APIKey,
			BaseURL:             cfg.BaseURL,
			Model:               cfg.Model,
			ByAzure:             cfg.ByAzure,
			APIVersion:          cfg.APIVersion,
			Temperature:         &temperature,
			MaxCompletionTokens: &maxCompletionTokens,
		})
	case "ark":
		return arkeino.NewChatModel(ctx, &arkeino.ChatModelConfig{
			APIKey:              cfg.APIKey,
			BaseURL:             cfg.BaseURL,
			Model:               cfg.Model,
			Temperature:         &temperature,
			MaxCompletionTokens: &maxCompletionTokens,
		})
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", provider)
	}
}
