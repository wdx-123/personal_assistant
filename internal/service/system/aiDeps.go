package system

import (
	"context"

	"personal_assistant/internal/service/system/aiselect"
	"personal_assistant/internal/service/system/aitool"
)

type aiMemoryWritebackHook interface {
	OnTurnCompleted(ctx context.Context, input aiMemoryWritebackInput) error
}

// AIDeps 收口 AIService 运行期依赖，其中 tool 相关能力下沉到子包。
type AIDeps struct {
	Tools         aitool.Deps
	Memory        aiMemoryProvider
	Compressor    aiContextCompressor
	Writeback     aiMemoryWritebackHook
	PromptBuilder aiselect.PromptBuilder
	Selector      aiselect.Selector
}
