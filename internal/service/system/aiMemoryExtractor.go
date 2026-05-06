package system

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	aimemory "personal_assistant/internal/infrastructure/ai/memory"
)

const (
	defaultAIMemoryExtractorMode         = "rule"
	defaultAIMemoryExtractTimeoutSeconds = 20
	defaultAIMemoryExtractMaxChars       = 6000
)

type aiMemoryFallbackExtractor struct {
	primary  aidomain.MemoryExtractor
	fallback aidomain.MemoryExtractor
}

func newAIMemoryExtractor() aidomain.MemoryExtractor {
	ruleExtractor := aimemory.NewRuleExtractor(aimemory.Options{})
	if aiMemoryExtractorMode() != "llm" {
		return ruleExtractor
	}

	llmExtractor, err := aimemory.NewLLMExtractor(context.Background(), aimemory.LLMExtractorOptions{
		Provider:            aiMemoryProviderName(),
		APIKey:              aiMemoryAPIKey(),
		BaseURL:             aiMemoryBaseURL(),
		Model:               aiMemoryModel(),
		ByAzure:             aiMemoryByAzure(),
		APIVersion:          aiMemoryAPIVersion(),
		Temperature:         aiMemoryTemperature(),
		MaxCompletionTokens: aiMemoryMaxCompletionTokens(),
		Timeout:             time.Duration(aiMemoryExtractTimeoutSeconds()) * time.Second,
		MaxInputChars:       aiMemoryExtractMaxChars(),
	})
	if err != nil {
		if global.Log != nil {
			global.Log.Warn("AI memory LLM extractor 初始化失败，回退到规则抽取", zap.Error(err))
		}
		return ruleExtractor
	}
	return aiMemoryFallbackExtractor{
		primary:  llmExtractor,
		fallback: ruleExtractor,
	}
}

func (e aiMemoryFallbackExtractor) Extract(
	ctx context.Context,
	input aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	if e.primary == nil {
		if e.fallback == nil {
			return aidomain.MemoryExtractionResult{}, nil
		}
		return e.fallback.Extract(ctx, input)
	}
	result, err := e.primary.Extract(ctx, input)
	if err == nil {
		return result, nil
	}
	if global.Log != nil {
		global.Log.Warn("AI memory LLM extractor 执行失败，回退到规则抽取", zap.Error(err))
	}
	if e.fallback == nil {
		return aidomain.MemoryExtractionResult{}, err
	}
	return e.fallback.Extract(ctx, input)
}

func aiMemoryExtractorMode() string {
	if global.Config == nil {
		return defaultAIMemoryExtractorMode
	}
	mode := strings.ToLower(strings.TrimSpace(global.Config.AI.Memory.ExtractorMode))
	if mode == "" {
		return defaultAIMemoryExtractorMode
	}
	return mode
}

func aiMemoryExtractTimeoutSeconds() int {
	if global.Config == nil || global.Config.AI.Memory.ExtractTimeoutSeconds <= 0 {
		return defaultAIMemoryExtractTimeoutSeconds
	}
	return global.Config.AI.Memory.ExtractTimeoutSeconds
}

func aiMemoryExtractMaxChars() int {
	if global.Config == nil || global.Config.AI.Memory.ExtractMaxChars <= 0 {
		return defaultAIMemoryExtractMaxChars
	}
	return global.Config.AI.Memory.ExtractMaxChars
}

func aiMemoryProviderName() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.Provider)
}

func aiMemoryBaseURL() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.BaseURL)
}

func aiMemoryModel() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.Model)
}

func aiMemoryByAzure() bool {
	return global.Config != nil && global.Config.AI.ByAzure
}

func aiMemoryAPIVersion() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.APIVersion)
}

func aiMemoryTemperature() float64 {
	if global.Config == nil {
		return 0
	}
	return global.Config.AI.Temperature
}

func aiMemoryMaxCompletionTokens() int {
	if global.Config == nil {
		return 0
	}
	return global.Config.AI.MaxCompletionTokens
}
