package eino

import (
	"context"
	"errors"
	"io"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

type Runtime struct {
	model        einomodel.BaseChatModel
	systemPrompt string
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
	if r == nil || r.model == nil {
		return aidomain.StreamResult{}, errors.New("eino runtime model is nil")
	}
	if sink == nil {
		return aidomain.StreamResult{}, errors.New("ai runtime sink is nil")
	}
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventConversationStarted,
		Payload: aidomain.ConversationStartedPayload{Title: deriveTitle(input.Content)},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}

	if err := r.emitVisibleThinking(ctx, input, sink); err != nil {
		return aidomain.StreamResult{}, err
	}

	reader, err := r.model.Stream(ctx, r.buildMessages(input))
	if err != nil {
		return aidomain.StreamResult{}, err
	}
	defer reader.Close()

	var output strings.Builder
	for {
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
		output.WriteString(msg.Content)
		if err := sink.Emit(ctx, aidomain.Event{
			Name:    aidomain.EventAssistantToken,
			Payload: aidomain.AssistantTokenPayload{Token: msg.Content},
		}); err != nil {
			return aidomain.StreamResult{}, err
		}
	}

	content := output.String()
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventMessageCompleted,
		Payload: aidomain.MessageCompletedPayload{Content: content},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}
	if err := sink.Emit(ctx, aidomain.Event{Name: aidomain.EventDone, Payload: map[string]any{}}); err != nil {
		return aidomain.StreamResult{}, err
	}
	return aidomain.StreamResult{Content: content, FinishReason: "stop"}, nil
}

func (r *Runtime) emitVisibleThinking(
	ctx context.Context,
	input aidomain.StreamInput,
	sink aidomain.Sink,
) error {
	content, err := r.generateThinkingSummary(ctx, input)
	if err != nil {
		content = buildVisibleThinkingSummary(input.Content)
	}
	content = normalizeThinkingSummary(content, input.Content)
	if content == "" {
		return nil
	}
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventThinkingStarted,
		Payload: aidomain.ThinkingStartedPayload{Title: "深度思考"},
	}); err != nil {
		return err
	}
	for _, chunk := range splitTextChunks(content, 24) {
		if err := sink.Emit(ctx, aidomain.Event{
			Name:    aidomain.EventThinkingDelta,
			Payload: aidomain.ThinkingDeltaPayload{Delta: chunk},
		}); err != nil {
			return err
		}
	}
	return sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventThinkingCompleted,
		Payload: aidomain.ThinkingCompletedPayload{Content: content},
	})
}

func (r *Runtime) generateThinkingSummary(ctx context.Context, input aidomain.StreamInput) (string, error) {
	messages := []*schema.Message{
		schema.SystemMessage(strings.TrimSpace(`
你需要先输出一段可以直接展示给用户的“深度思考”短句。
要求：
1. 只描述“当前判断 / 正在做什么 / 下一步是什么”。
2. 不泄露模型私有推理，不展示完整推导链。
3. 不输出最终答案，不要复述太多用户原文。
4. 最多 3 行，总长度控制在 120 个中文字符以内。
5. 直接输出正文，不要加标题、编号、markdown 列表符号。`)),
		schema.UserMessage(strings.TrimSpace(input.Content)),
	}
	return r.readStreamContent(ctx, messages)
}

func (r *Runtime) readStreamContent(ctx context.Context, messages []*schema.Message) (string, error) {
	reader, err := r.model.Stream(ctx, messages)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var output strings.Builder
	for {
		msg, recvErr := reader.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return "", recvErr
		}
		if msg == nil || msg.Content == "" {
			continue
		}
		output.WriteString(msg.Content)
	}
	return output.String(), nil
}

// buildMessages 把 domain 层历史消息转换成 Eino schema 消息。
// 参数：
//   - input：包含历史消息与当前用户输入的 StreamInput。
//
// 返回值：
//   - []*schema.Message：传给 Eino ChatModel 的消息序列。
//
// 注意事项：
//   - 这里只处理 user/assistant 文本消息，不注入 tool message。
func (r *Runtime) buildMessages(input aidomain.StreamInput) []*schema.Message {
	messages := []*schema.Message{schema.SystemMessage(r.systemPrompt)}
	for _, item := range input.History {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		switch strings.TrimSpace(item.Role) {
		case aidomain.RoleAssistant:
			messages = append(messages, schema.AssistantMessage(content, nil))
		default:
			messages = append(messages, schema.UserMessage(content))
		}
	}
	if strings.TrimSpace(input.Content) != "" {
		messages = append(messages, schema.UserMessage(strings.TrimSpace(input.Content)))
	}
	return messages
}

func buildVisibleThinkingSummary(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	if content == "" {
		return strings.Join([]string{
			"正在确认输入是否完整，并收拢本轮回答目标。",
			"下一步会先组织回答结构，再输出正式结果。",
		}, "\n")
	}
	return strings.Join([]string{
		"正在理解你的问题，先提炼核心目标和约束。",
		"当前关注点：" + truncateRunes(content, 24),
		"下一步会按重点组织回答，再输出正式结果。",
	}, "\n")
}

func normalizeThinkingSummary(content string, fallback string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, 3)
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789.、 "))
		if line == "" {
			continue
		}
		filtered = append(filtered, line)
		if len(filtered) == 3 {
			break
		}
	}
	if len(filtered) == 0 {
		return buildVisibleThinkingSummary(fallback)
	}
	content = strings.Join(filtered, "\n")
	return truncateRunes(content, 120)
}

func splitTextChunks(content string, size int) []string {
	if size <= 0 {
		size = 24
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]string, 0, (len(runes)/size)+1)
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func truncateRunes(content string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
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
