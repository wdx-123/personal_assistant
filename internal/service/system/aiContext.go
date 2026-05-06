package system

import (
	"context"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

// aiMemoryRecallInput 描述未来记忆召回组件需要消费的最小上下文。
type aiMemoryRecallInput struct {
	ConversationID string
	UserID         uint
	Query          string
	History        []aidomain.Message
	ToolCallCtx    aidomain.ToolCallContext
}

// aiMemoryProvider 负责为当前会话提供额外的记忆消息。
// 默认不注入实现，因此当前阶段不会改变上下文行为。
type aiMemoryProvider interface {
	RecallMessages(ctx context.Context, input aiMemoryRecallInput) ([]aidomain.Message, error)
}

type aiStructuredMemoryProvider interface {
	Recall(ctx context.Context, input aiMemoryRecallInput) (aiMemoryRecallResult, error)
}

// aiContextCompressionInput 描述上下文压缩组件的输入。
type aiContextCompressionInput struct {
	ConversationID string
	Query          string
	Messages       []aidomain.Message
}

// aiContextCompressor 负责在进入 runtime 前压缩消息上下文。
// 默认不注入实现，因此当前阶段不会裁剪或摘要历史消息。
type aiContextCompressor interface {
	CompressMessages(ctx context.Context, input aiContextCompressionInput) ([]aidomain.Message, error)
}

// aiContextBuildArgs 是一次 runtime 上下文装配所需的输入。
type aiContextBuildArgs struct {
	ConversationID string
	UserID         uint
	Query          string
	StoredMessages []*entity.AIMessage
	VisibleTools   []aidomain.Tool
	ToolCallCtx    aidomain.ToolCallContext
}

// aiContextSnapshot 表示装配完成后可直接喂给 runtime 的上下文片段。
type aiContextSnapshot struct {
	History     []aidomain.Message
	Diagnostics aiHybridContextDiagnostics
}

// aiContextAssembler 负责统一收口历史消息、记忆扩展点、压缩扩展点和动态 prompt。
type aiContextAssembler interface {
	Build(ctx context.Context, args aiContextBuildArgs) (aiContextSnapshot, error)
}

type defaultAIContextAssembler struct {
	memory     aiMemoryProvider
	compressor aiContextCompressor
	planner    aiHybridContextPlanner
}

func newAIContextAssembler(deps AIDeps) aiContextAssembler {
	return &defaultAIContextAssembler{
		memory:     deps.Memory,
		compressor: deps.Compressor,
		planner:    newDefaultAIHybridContextPlanner(),
	}
}

// Build 根据当前会话状态生成 runtime 所需的历史消息和动态 prompt。
// 当前阶段默认行为仅做消息转换和 prompt 构造；未来记忆与压缩能力通过可选接口接入。
func (a *defaultAIContextAssembler) Build(
	ctx context.Context,
	args aiContextBuildArgs,
) (aiContextSnapshot, error) {
	history := messagesToRuntimeHistory(args.StoredMessages)
	recallResult := aiMemoryRecallResult{}
	if a.memory != nil {
		input := aiMemoryRecallInput{
			ConversationID: args.ConversationID,
			UserID:         args.UserID,
			Query:          args.Query,
			History:        history,
			ToolCallCtx:    args.ToolCallCtx,
		}
		if structured, ok := a.memory.(aiStructuredMemoryProvider); ok {
			result, err := structured.Recall(ctx, input)
			if err != nil {
				return aiContextSnapshot{}, err
			}
			recallResult = result
		} else {
			recalled, err := a.memory.RecallMessages(ctx, input)
			if err != nil {
				return aiContextSnapshot{}, err
			}
			recallResult.Messages = recalled
		}
	}

	if a.planner != nil {
		planned, err := a.planner.Plan(ctx, aiHybridContextInput{
			ConversationID: args.ConversationID,
			Query:          args.Query,
			RawHistory:     history,
			Recall:         recallResult,
			VisibleTools:   args.VisibleTools,
		})
		if err != nil {
			return aiContextSnapshot{}, err
		}
		return aiContextSnapshot(planned), nil
	}

	if a.compressor != nil {
		inputMessages := joinAIMemoryFirst(recallResult.Messages, history)
		compressed, err := a.compressor.CompressMessages(ctx, aiContextCompressionInput{
			ConversationID: args.ConversationID,
			Query:          args.Query,
			Messages:       inputMessages,
		})
		if err != nil {
			return aiContextSnapshot{}, err
		}
		history = compressed
	} else {
		history = joinAIMemoryFirst(recallResult.Messages, history)
	}

	return aiContextSnapshot{
		History:     history,
		Diagnostics: recallResult.Diagnostics,
	}, nil
}
