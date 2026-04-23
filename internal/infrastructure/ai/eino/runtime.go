package eino

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

type Runtime struct {
	// model 是底层 Eino ChatModel，实现真实的对话和 tool calling。
	model einomodel.BaseChatModel
	// systemPrompt 是 runtime 固定注入的基础系统提示词。
	systemPrompt string
	// bindMu 保护不支持无副作用 WithTools 的模型在 BindTools 时的并发安全。
	bindMu sync.Mutex
}

// NewRuntime 创建 Eino 基础流式 runtime。
// 参数：
//   - ctx：初始化上下文。
//   - opt：模型和提示词配置。
//
// 返回值：
//   - *Runtime：可被 Service 注入使用的 Eino runtime。
//   - error：模型初始化失败时返回错误，由 core 层决定是否回退 local。
//
// 核心流程：
//  1. 使用 Options 创建 ChatModel。
//  2. 归一化 system prompt，缺失时使用基础对话提示词。
//  3. 返回只负责基础流式对话的 runtime。
//
// 注意事项：
//   - 当前阶段不注册 Tool，不启用 ApprovalMiddleware，也不使用 checkpoint/resume。
func NewRuntime(ctx context.Context, opt Options) (*Runtime, error) {
	model, err := NewChatModel(ctx, opt)
	if err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(opt.SystemPrompt)
	if prompt == "" {
		prompt = "你是 personal_assistant 项目的 AI 助手。当前阶段只提供基础流式对话，不调用工具，不请求人工确认。请直接、准确地回答用户问题。"
	}
	return &Runtime{model: model, systemPrompt: prompt}, nil
}

// Name 返回当前 runtime 的稳定名称。
func (r *Runtime) Name() string {
	return "eino"
}

// Stream 调用 Eino ChatModel 执行基础流式对话。
// 参数：
//   - ctx：请求上下文，取消时模型流应停止。
//   - input：用户输入和历史消息。
//   - sink：事件输出端。
//
// 返回值：
//   - aidomain.StreamResult：最终聚合内容与结束原因。
//   - error：模型调用或事件输出失败时返回。
//
// 核心流程：
//  1. 校验 runtime 和 sink。
//  2. 先发送 conversation_started 事件。
//  3. 构造 Eino 消息数组并调用模型 Stream。
//  4. 把模型返回的每个文本片段转成 assistant_token。
//  5. 输出 message_completed 和 done 终态事件。
//
// 注意事项：
//   - 本实现不允许模型调用工具，也不会进入人工确认或恢复流程。
func (r *Runtime) Stream(
	ctx context.Context,
	input aidomain.StreamInput,
	sink aidomain.Sink,
) (aidomain.StreamResult, error) {
	// 运行前先确保 runtime 和 sink 都已正确初始化。
	if r == nil || r.model == nil {
		return aidomain.StreamResult{}, errors.New("eino runtime model is nil")
	}
	if sink == nil {
		return aidomain.StreamResult{}, errors.New("ai runtime sink is nil")
	}

	// 每轮流式响应开始前先发 conversation_started，供 projector 建立起始态。
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventConversationStarted,
		Payload: aidomain.ConversationStartedPayload{Title: deriveTitle(input.Content)},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}

	// 无工具时保持原有纯文本流式路径，避免无谓进入 tool loop。
	if len(input.Tools) == 0 {
		return r.streamTextOnly(ctx, input, sink)
	}

	// 有工具时切换到 tool calling 路径。
	return r.streamWithTools(ctx, input, sink)
}

// streamTextOnly 负责执行不带工具调用的纯文本流式对话。
func (r *Runtime) streamTextOnly(
	ctx context.Context,
	input aidomain.StreamInput,
	sink aidomain.Sink,
) (aidomain.StreamResult, error) {
	// 先把 domain 消息转换成 Eino 消息，再启动模型流。
	reader, err := r.model.Stream(ctx, r.buildMessages(input))
	if err != nil {
		return aidomain.StreamResult{}, err
	}
	defer reader.Close()

	// output 用于在流结束后拼出完整 assistant 正文。
	var output strings.Builder
	for {
		// 持续读取模型增量输出，直到收到 EOF。
		msg, recvErr := reader.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return aidomain.StreamResult{}, recvErr
		}
		if msg == nil || msg.Content == "" {
			continue
		}

		// 把文本片段累计到最终正文，同时实时转发 assistant_token。
		output.WriteString(msg.Content)
		if err := sink.Emit(ctx, aidomain.Event{
			Name:    aidomain.EventAssistantToken,
			Payload: aidomain.AssistantTokenPayload{Token: msg.Content},
		}); err != nil {
			return aidomain.StreamResult{}, err
		}
	}

	// 纯文本流结束后补发最终正文，帮助 projector 收敛最终状态。
	content := output.String()
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventMessageCompleted,
		Payload: aidomain.MessageCompletedPayload{Content: content},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}

	// 最后发 done 终态，通知上层 SSE 可以正常收尾。
	if err := sink.Emit(ctx, aidomain.Event{Name: aidomain.EventDone, Payload: map[string]any{}}); err != nil {
		return aidomain.StreamResult{}, err
	}
	return aidomain.StreamResult{Content: content, FinishReason: "stop"}, nil
}

// streamWithTools 负责执行带 tool calling 的多轮 assistant/tool 循环。
func (r *Runtime) streamWithTools(
	ctx context.Context,
	input aidomain.StreamInput,
	sink aidomain.Sink,
) (aidomain.StreamResult, error) {
	// 先把本轮可见工具绑定到模型实例上。
	modelWithTools, unlock, err := r.bindToolModel(input.Tools)
	if err != nil {
		return aidomain.StreamResult{}, err
	}
	defer unlock()

	// messages 保存 assistant 和 tool 的完整往返上下文。
	messages := r.buildMessages(input)
	// toolMap 用于把模型返回的 tool name 映射回真实实现。
	toolMap := make(map[string]aidomain.Tool, len(input.Tools))
	for _, tool := range input.Tools {
		if tool == nil {
			continue
		}

		// tool 名做 trim，避免模型或注册表中的空白差异导致找不到实现。
		spec := tool.Spec()
		toolMap[strings.TrimSpace(spec.Name)] = tool
	}

	// maxToolTurns 防止模型持续循环调用工具导致请求无限悬挂。
	const maxToolTurns = 8
	for turn := 0; turn < maxToolTurns; turn++ {
		// 每一轮都让模型基于最新 messages 再生成一次 assistant 响应。
		reader, err := modelWithTools.Stream(ctx, messages)
		if err != nil {
			return aidomain.StreamResult{}, err
		}

		// drainAssistantTurn 会拼出本轮 assistant 的完整内容和 tool call 列表。
		contentChunks, assistantMessage, err := drainAssistantTurn(reader)
		if err != nil {
			return aidomain.StreamResult{}, err
		}
		if assistantMessage == nil {
			// 极端情况下没有任何 assistant 消息时，补一个空 assistant 占位。
			assistantMessage = schema.AssistantMessage("", nil)
		}

		// 先把 assistant 响应写回上下文，后续 tool message 才能正确挂在这轮 assistant 之后。
		messages = append(messages, schema.AssistantMessage(assistantMessage.Content, assistantMessage.ToolCalls))
		if len(assistantMessage.ToolCalls) == 0 {
			// 某些模型只在最终 concat 内容里给出正文，这里兜底补上内容片段。
			if len(contentChunks) == 0 && assistantMessage.Content != "" {
				contentChunks = append(contentChunks, assistantMessage.Content)
			}
			for _, chunk := range contentChunks {
				// 跳过纯空白 chunk，避免前端看到无意义 token 闪烁。
				if strings.TrimSpace(chunk) == "" {
					continue
				}

				// 把最终 assistant 正文按 chunk 形式实时发给 projector/SSE。
				if err := sink.Emit(ctx, aidomain.Event{
					Name:    aidomain.EventAssistantToken,
					Payload: aidomain.AssistantTokenPayload{Token: chunk},
				}); err != nil {
					return aidomain.StreamResult{}, err
				}
			}

			// 本轮没有 tool call，说明模型已经进入最终回答阶段。
			content := assistantMessage.Content
			if err := sink.Emit(ctx, aidomain.Event{
				Name:    aidomain.EventMessageCompleted,
				Payload: aidomain.MessageCompletedPayload{Content: content},
			}); err != nil {
				return aidomain.StreamResult{}, err
			}
			if err := sink.Emit(ctx, aidomain.Event{Name: aidomain.EventDone, Payload: map[string]any{}}); err != nil {
				return aidomain.StreamResult{}, err
			}

			// finish reason 优先取模型原始元信息，没有时回退 stop。
			finishReason := "stop"
			if assistantMessage.ResponseMeta != nil && strings.TrimSpace(assistantMessage.ResponseMeta.FinishReason) != "" {
				finishReason = assistantMessage.ResponseMeta.FinishReason
			}
			return aidomain.StreamResult{Content: content, FinishReason: finishReason}, nil
		}

		// assistant 返回 tool call 后，顺序执行工具并把 tool message 追加回上下文。
		toolMessages, err := r.executeToolCalls(ctx, toolMap, assistantMessage.ToolCalls, input.ToolCallContext, sink)
		if err != nil {
			return aidomain.StreamResult{}, err
		}
		messages = append(messages, toolMessages...)
	}

	// 超过保护阈值仍未收敛时，直接终止本轮请求。
	return aidomain.StreamResult{}, errors.New("eino runtime exceeded max tool turns")
}

// bindToolModel 负责把 domain tool spec 转成 Eino tool schema 并绑定到模型。
func (r *Runtime) bindToolModel(tools []aidomain.Tool) (einomodel.BaseChatModel, func(), error) {
	// 先构建所有工具的 schema 定义。
	toolInfos := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}

		// 每个工具都转成 Eino 可识别的 ToolInfo。
		info, err := buildSchemaToolInfo(tool.Spec())
		if err != nil {
			return nil, func() {}, err
		}
		toolInfos = append(toolInfos, info)
	}

	// 支持 WithTools 的模型优先走无副作用绑定路径。
	if toolCallingModel, ok := r.model.(einomodel.ToolCallingChatModel); ok {
		bound, err := toolCallingModel.WithTools(toolInfos)
		return bound, func() {}, err
	}
	// 只支持 BindTools 的模型需要串行绑定，避免并发请求互相污染。
	if chatModel, ok := r.model.(einomodel.ChatModel); ok {
		r.bindMu.Lock()
		if err := chatModel.BindTools(toolInfos); err != nil {
			r.bindMu.Unlock()
			return nil, func() {}, err
		}
		return chatModel, func() { r.bindMu.Unlock() }, nil
	}
	// 两种能力都不支持时，说明当前模型无法执行 tool calling。
	return nil, func() {}, errors.New("eino runtime model does not support tool calling")
}

// buildSchemaToolInfo 把 domain 层 ToolSpec 转成 Eino 的 ToolInfo。
func buildSchemaToolInfo(spec aidomain.ToolSpec) (*schema.ToolInfo, error) {
	// 先把参数列表转成按名称索引的 schema 参数定义。
	params := make(map[string]*schema.ParameterInfo, len(spec.Parameters))
	for _, param := range spec.Parameters {
		info, err := buildSchemaParameterInfo(param)
		if err != nil {
			return nil, err
		}
		params[param.Name] = info
	}

	// ToolInfo 只承载 name、描述和参数协议，不包含任何业务实现。
	return &schema.ToolInfo{
		Name:        spec.Name,
		Desc:        spec.Description,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}, nil
}

// buildSchemaParameterInfo 递归把 domain 层参数定义转换成 Eino 参数协议。
func buildSchemaParameterInfo(param aidomain.ToolParameter) (*schema.ParameterInfo, error) {
	// 先填充当前参数节点的基础元信息。
	info := &schema.ParameterInfo{
		Type:     schema.DataType(param.Type),
		Desc:     param.Description,
		Enum:     param.Enum,
		Required: param.Required,
	}
	if param.Items != nil {
		// array 参数需要继续递归描述元素结构。
		itemInfo, err := buildSchemaParameterInfo(*param.Items)
		if err != nil {
			return nil, err
		}
		info.ElemInfo = itemInfo
	}
	if len(param.Properties) > 0 {
		// object 参数需要递归构建所有子字段定义。
		subParams := make(map[string]*schema.ParameterInfo, len(param.Properties))
		for _, child := range param.Properties {
			childInfo, err := buildSchemaParameterInfo(child)
			if err != nil {
				return nil, err
			}
			subParams[child.Name] = childInfo
		}
		info.SubParams = subParams
	}
	return info, nil
}

// drainAssistantTurn 负责从一轮 assistant 输出流中拼出最终消息和增量文本块。
func drainAssistantTurn(
	reader *schema.StreamReader[*schema.Message],
) ([]string, *schema.Message, error) {
	defer reader.Close()

	// chunks 保存原始消息块，contentChunks 额外保留文本增量供前端逐块输出。
	chunks := make([]*schema.Message, 0, 8)
	contentChunks := make([]string, 0, 8)
	for {
		// 持续读取模型输出，直到本轮 assistant 结束。
		msg, recvErr := reader.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return nil, nil, recvErr
		}
		if msg == nil {
			continue
		}

		// 每个消息块既参与最终 concat，也单独保留文本片段。
		chunks = append(chunks, msg)
		if msg.Content != "" {
			contentChunks = append(contentChunks, msg.Content)
		}
	}
	if len(chunks) == 0 {
		// 完全没有 chunk 时返回空 assistant，避免调用方处理 nil。
		return contentChunks, schema.AssistantMessage("", nil), nil
	}

	// Eino 提供的 ConcatMessages 会统一合并文本和 tool calls。
	assistantMessage, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, nil, err
	}
	return contentChunks, assistantMessage, nil
}

// executeToolCalls 负责顺序执行本轮 assistant 产出的所有工具调用。
func (r *Runtime) executeToolCalls(
	ctx context.Context,
	toolMap map[string]aidomain.Tool,
	toolCalls []schema.ToolCall,
	callCtx aidomain.ToolCallContext,
	sink aidomain.Sink,
) ([]*schema.Message, error) {
	// 每个工具执行完成后都会生成一条 tool message 回填给模型。
	messages := make([]*schema.Message, 0, len(toolCalls))
	for idx, toolCall := range toolCalls {
		// 先归一化调用 ID 和工具名，用于 trace 以及 tool message 关联。
		callID := deriveToolCallID(toolCall, idx)
		toolName := strings.TrimSpace(toolCall.Function.Name)
		toolImpl, ok := toolMap[toolName]
		if !ok {
			return nil, fmt.Errorf("ai tool not found: %s", toolName)
		}

		// 工具开始前先发 started 事件，让 projector 建立 running 态 trace 项。
		if err := sink.Emit(ctx, aidomain.Event{
			Name: aidomain.EventToolCallStarted,
			Payload: aidomain.ToolCallStartedPayload{
				Key:         callID,
				ToolName:    toolName,
				Title:       "调用工具 " + toolName,
				Description: "正在执行工具调用。",
			},
		}); err != nil {
			return nil, err
		}

		// 记录耗时并执行真实工具实现。
		startedAt := time.Now()
		result, err := toolImpl.Call(ctx, aidomain.ToolCall{
			ID:            callID,
			Name:          toolName,
			ArgumentsJSON: toolCall.Function.Arguments,
		}, callCtx)
		durationMS := time.Since(startedAt).Milliseconds()
		if err != nil {
			// 工具失败时也要补 finished 事件，让前端和 trace 看到失败状态。
			if emitErr := sink.Emit(ctx, aidomain.Event{
				Name: aidomain.EventToolCallFinished,
				Payload: aidomain.ToolCallFinishedPayload{
					Key:            callID,
					ToolName:       toolName,
					Description:    "工具调用失败。",
					DurationMS:     durationMS,
					Status:         "failed",
					Content:        summarizeToolOutput(err.Error()),
					DetailMarkdown: err.Error(),
				},
			}); emitErr != nil {
				return nil, emitErr
			}
			return nil, err
		}

		// 工具成功后把摘要和详情折叠进 finished 事件。
		if err := sink.Emit(ctx, aidomain.Event{
			Name: aidomain.EventToolCallFinished,
			Payload: aidomain.ToolCallFinishedPayload{
				Key:            callID,
				ToolName:       toolName,
				Description:    "工具调用完成。",
				DurationMS:     durationMS,
				Status:         "success",
				Content:        summarizeToolOutput(result.Summary),
				DetailMarkdown: result.DetailMarkdown,
			},
		}); err != nil {
			return nil, err
		}

		// ToolMessage 会作为下一轮模型输入，让模型基于工具输出继续生成回答。
		messages = append(messages, schema.ToolMessage(result.Output, callID, schema.WithToolName(toolName)))
	}
	return messages, nil
}

// buildMessages 把 domain 层历史消息转换成 Eino schema 消息。
// 参数：
//   - input：包含历史消息与当前用户输入的 StreamInput。
//
// 返回值：
//   - []*schema.Message：传给 Eino ChatModel 的消息序列。
//
// 注意事项：
//   - 当本轮存在动态工具约束时，会额外插入一条动态 system prompt。
func (r *Runtime) buildMessages(input aidomain.StreamInput) []*schema.Message {
	// 预留 system prompt、动态 prompt 和当前用户输入的容量。
	messages := make([]*schema.Message, 0, len(input.History)+3)
	if strings.TrimSpace(r.systemPrompt) != "" {
		// 固定 system prompt 始终放在最前面，提供通用对话约束。
		messages = append(messages, schema.SystemMessage(r.systemPrompt))
	}
	if strings.TrimSpace(input.DynamicSystemPrompt) != "" {
		// 动态 prompt 用于注入本轮工具清单和调用约束。
		messages = append(messages, schema.SystemMessage(strings.TrimSpace(input.DynamicSystemPrompt)))
	}
	for _, item := range input.History {
		// 历史空消息不参与上下文，避免噪音输入。
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch strings.TrimSpace(item.Role) {
		case aidomain.RoleAssistant:
			// assistant 历史按 assistant message 回放。
			messages = append(messages, schema.AssistantMessage(content, nil))
		default:
			// 其余角色统一按 user message 处理。
			messages = append(messages, schema.UserMessage(content))
		}
	}
	if strings.TrimSpace(input.Content) != "" {
		// 当前用户输入始终放在最后，触发本轮模型生成。
		messages = append(messages, schema.UserMessage(strings.TrimSpace(input.Content)))
	}
	return messages
}

// deriveToolCallID 负责为工具调用生成稳定 trace key。
func deriveToolCallID(toolCall schema.ToolCall, index int) string {
	// 模型已提供 ID 时直接复用，保证与上游 tool call 标识一致。
	if strings.TrimSpace(toolCall.ID) != "" {
		return strings.TrimSpace(toolCall.ID)
	}
	// 否则按顺序生成兜底 ID，避免 trace 丢失主键。
	return fmt.Sprintf("tool_call_%d", index+1)
}

// summarizeToolOutput 负责把工具输出压缩成适合 trace 摘要展示的短文本。
func summarizeToolOutput(content string) string {
	// 先去掉前后空白，避免摘要出现纯空格内容。
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// 长内容截断到 120 个 rune，防止 trace 卡片过长。
	runes := []rune(content)
	if len(runes) <= 120 {
		return string(runes)
	}
	return string(runes[:120])
}

// deriveTitle 根据用户输入生成会话开始事件标题。
func deriveTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "新建会话"
	}
	runes := []rune(content)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	return string(runes)
}
