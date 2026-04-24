package system

import aidomain "personal_assistant/internal/domain/ai"

// aiPromptBuilder 负责生成本轮 runtime 需要的动态 system prompt。
// 该接口仅在 service/system 内部使用，不向 controller 或 domain 泄漏具体实现。
type aiPromptBuilder interface {
	BuildDynamicPrompt(tools []aidomain.Tool, principal aidomain.AIToolPrincipal) string
}

// defaultAIToolPromptBuilder 复用当前内建的工具约束 prompt 逻辑。
type defaultAIToolPromptBuilder struct{}

func (defaultAIToolPromptBuilder) BuildDynamicPrompt(
	tools []aidomain.Tool,
	principal aidomain.AIToolPrincipal,
) string {
	return buildAIToolDynamicPrompt(tools, principal)
}
