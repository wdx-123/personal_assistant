package local

import (
	"context"
	"os"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
)

type Runtime struct {
	name              string
	heartbeatInterval time.Duration
}

// NewRuntime 负责创建本地 AI runtime。
// 参数：
//   - heartbeatInterval：等待或长链路场景使用的心跳间隔；当前最小 runtime 仅保留该配置兼容。
//
// 返回值：
//   - *Runtime：可直接用于 Service 的本地 runtime 实例。
//
// 核心流程：
//  1. 读取当前主机名作为 runtime 标识的一部分。
//  2. 修正非法心跳配置，使用默认值兜底。
//  3. 返回一个不依赖外部模型的本地实现。
//
// 注意事项：
//   - 该 runtime 用于本地开发、测试和 Eino 初始化失败时的降级。
func NewRuntime(heartbeatInterval time.Duration) *Runtime {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "local"
	}
	if heartbeatInterval <= 0 {
		heartbeatInterval = 20 * time.Second
	}
	return &Runtime{name: "local:" + host, heartbeatInterval: heartbeatInterval}
}

// Name 返回当前 runtime 的稳定名称。
// 返回值：
//   - string：用于日志、排障和运行时识别的名称。
func (r *Runtime) Name() string {
	if r == nil || strings.TrimSpace(r.name) == "" {
		return "local"
	}
	return r.name
}

// Stream 执行本地最小流式对话。
// 参数：
//   - ctx：请求上下文，取消时停止后续事件输出。
//   - input：本次用户输入与会话消息标识。
//   - sink：事件输出端，负责后续 SSE 和 DB 投影。
//
// 返回值：
//   - aidomain.StreamResult：最终回复内容与结束原因。
//   - error：事件输出失败或 sink 缺失时返回错误。
//
// 核心流程：
//  1. 先发送 conversation_started。
//  2. 根据用户输入生成确定性本地回复。
//  3. 将回复拆成多个 token 事件输出。
//  4. 发送 message_completed 和 done 终态事件。
//
// 注意事项：
//   - 本地 runtime 不调用模型、不调用工具、不触发 interrupt。
func (r *Runtime) Stream(ctx context.Context, input aidomain.StreamInput, sink aidomain.Sink) (aidomain.StreamResult, error) {
	if sink == nil {
		return aidomain.StreamResult{}, errNilSink
	}
	title := deriveTitle(input.Content)
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventConversationStarted,
		Payload: aidomain.ConversationStartedPayload{Title: title},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}

	if err := emitVisibleThinking(ctx, sink, buildThinkingSummary(input.Content)); err != nil {
		return aidomain.StreamResult{}, err
	}

	reply := buildReply(input.Content)
	for _, chunk := range splitChunks(reply, 48) {
		if err := sink.Emit(ctx, aidomain.Event{
			Name:    aidomain.EventAssistantToken,
			Payload: aidomain.AssistantTokenPayload{Token: chunk},
		}); err != nil {
			return aidomain.StreamResult{}, err
		}
	}
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventMessageCompleted,
		Payload: aidomain.MessageCompletedPayload{Content: reply},
	}); err != nil {
		return aidomain.StreamResult{}, err
	}
	if err := sink.Emit(ctx, aidomain.Event{Name: aidomain.EventDone, Payload: map[string]any{}}); err != nil {
		return aidomain.StreamResult{}, err
	}
	return aidomain.StreamResult{Content: reply, FinishReason: "stop"}, nil
}

// buildReply 负责为本地 runtime 生成确定性回复。
// 作用：在没有模型或模型不可用时，仍保证 SSE 与消息落库闭环可验证。
func buildReply(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "我没有收到有效内容，请重新输入你的问题。"
	}
	return "我已收到你的问题：" + content + "\n\n当前阶段 AI 助手只保留基础流式对话能力；我会基于你的输入直接回答，不再调用工具或等待人工确认。"
}

// buildThinkingSummary 为本地 runtime 生成用户可见的外显思考短句。
func buildThinkingSummary(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	if content == "" {
		return strings.Join([]string{
			"正在检查输入是否完整，并确认本轮回答目标。",
			"下一步会先归纳问题重点，再输出正式回复。",
		}, "\n")
	}
	return strings.Join([]string{
		"正在理解你的问题，先提炼核心目标和约束。",
		"当前关注点：" + truncateRunes(content, 24),
		"下一步会按重点组织回答，再输出正式结果。",
	}, "\n")
}

// deriveTitle 根据用户输入生成会话开始事件中的标题。
// 作用：只做轻量截断，不引入模型或复杂规划。
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

// splitChunks 把完整回复拆成固定大小的流式片段。
// 参数：
//   - content：完整文本。
//   - size：每个片段的 rune 数量；小于等于 0 时使用默认值。
//
// 返回值：
//   - []string：按顺序输出的文本片段。
func splitChunks(content string, size int) []string {
	if size <= 0 {
		size = 48
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

func emitVisibleThinking(ctx context.Context, sink aidomain.Sink, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if err := sink.Emit(ctx, aidomain.Event{
		Name:    aidomain.EventThinkingStarted,
		Payload: aidomain.ThinkingStartedPayload{Title: "深度思考"},
	}); err != nil {
		return err
	}
	for _, chunk := range splitChunks(content, 24) {
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

// runtimeError 表示本地 runtime 内部的轻量错误类型。
type runtimeError string

func (e runtimeError) Error() string { return string(e) }

const errNilSink runtimeError = "ai runtime sink is nil"
