package eino

import (
	"context"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

type runtimeEventSinkStub struct {
	events []aidomain.Event
}

func (s *runtimeEventSinkStub) Emit(_ context.Context, event aidomain.Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *runtimeEventSinkStub) Heartbeat(context.Context) error {
	return nil
}

type fakeToolCallingChatModel struct {
	streams     [][]*schema.Message
	streamCalls int
	tools       []*schema.ToolInfo
	inputs      [][]*schema.Message
}

var _ einomodel.ToolCallingChatModel = (*fakeToolCallingChatModel)(nil)

func (m *fakeToolCallingChatModel) Generate(
	context.Context,
	[]*schema.Message,
	...einomodel.Option,
) (*schema.Message, error) {
	return schema.AssistantMessage("", nil), nil
}

func (m *fakeToolCallingChatModel) Stream(
	_ context.Context,
	input []*schema.Message,
	_ ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	cloned := make([]*schema.Message, len(input))
	copy(cloned, input)
	m.inputs = append(m.inputs, cloned)

	if m.streamCalls >= len(m.streams) {
		return schema.StreamReaderFromArray([]*schema.Message{}), nil
	}
	out := m.streams[m.streamCalls]
	m.streamCalls++
	return schema.StreamReaderFromArray(out), nil
}

func (m *fakeToolCallingChatModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	m.tools = tools
	return m, nil
}

type fakeRuntimeTool struct {
	spec       aidomain.ToolSpec
	result     aidomain.ToolResult
	calls      []aidomain.ToolCall
	callCtxLog []aidomain.ToolCallContext
}

func (t *fakeRuntimeTool) Spec() aidomain.ToolSpec {
	return t.spec
}

func (t *fakeRuntimeTool) Call(
	_ context.Context,
	call aidomain.ToolCall,
	callCtx aidomain.ToolCallContext,
) (aidomain.ToolResult, error) {
	t.calls = append(t.calls, call)
	t.callCtxLog = append(t.callCtxLog, callCtx)
	return t.result, nil
}

func TestRuntimeStreamWithToolsEmitsToolEventsAndFinalTokens(t *testing.T) {
	model := &fakeToolCallingChatModel{
		streams: [][]*schema.Message{
			{
				schema.AssistantMessage("", []schema.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: schema.FunctionCall{
							Name:      "get_my_oj_stats",
							Arguments: `{"platform":"leetcode"}`,
						},
					},
				}),
			},
			{
				schema.AssistantMessage("统计如下：LeetCode 已通过 123 题。", nil),
			},
		},
	}
	runtime := &Runtime{
		model:        model,
		systemPrompt: "base system prompt",
	}
	tool := &fakeRuntimeTool{
		spec: aidomain.ToolSpec{
			Name:        "get_my_oj_stats",
			Description: "获取当前用户 OJ 统计",
			Parameters: []aidomain.ToolParameter{
				{Name: "platform", Type: aidomain.ToolParameterTypeString, Required: true},
			},
		},
		result: aidomain.ToolResult{
			Output:         `{"platform":"leetcode","passed_number":123}`,
			Summary:        "已返回当前用户的 OJ 统计",
			DetailMarkdown: "```json\n{\"platform\":\"leetcode\",\"passed_number\":123}\n```",
		},
	}
	sink := &runtimeEventSinkStub{}

	result, err := runtime.Stream(context.Background(), aidomain.StreamInput{
		Content:             "帮我看 leetcode 统计",
		DynamicSystemPrompt: "本轮只允许使用可见工具。",
		Tools:               []aidomain.Tool{tool},
		ToolCallContext: aidomain.ToolCallContext{
			Principal: aidomain.AIToolPrincipal{UserID: 7},
		},
	}, sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.Content != "统计如下：LeetCode 已通过 123 题。" {
		t.Fatalf("result.Content = %q", result.Content)
	}
	if len(model.tools) != 1 || model.tools[0].Name != "get_my_oj_stats" {
		t.Fatalf("bound tools = %+v", model.tools)
	}
	if len(model.inputs) == 0 || len(model.inputs[0]) < 2 {
		t.Fatalf("model inputs = %+v", model.inputs)
	}
	if model.inputs[0][0].Content != "base system prompt" {
		t.Fatalf("first system prompt = %q", model.inputs[0][0].Content)
	}
	if model.inputs[0][1].Content != "本轮只允许使用可见工具。" {
		t.Fatalf("dynamic system prompt = %q", model.inputs[0][1].Content)
	}
	if len(tool.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(tool.calls))
	}
	if tool.calls[0].ArgumentsJSON != `{"platform":"leetcode"}` {
		t.Fatalf("tool arguments = %q", tool.calls[0].ArgumentsJSON)
	}
	if len(tool.callCtxLog) != 1 || tool.callCtxLog[0].Principal.UserID != 7 {
		t.Fatalf("tool call context = %+v", tool.callCtxLog)
	}

	eventNames := make([]aidomain.EventName, 0, len(sink.events))
	for _, event := range sink.events {
		eventNames = append(eventNames, event.Name)
	}
	expected := []aidomain.EventName{
		aidomain.EventConversationStarted,
		aidomain.EventToolCallStarted,
		aidomain.EventToolCallFinished,
		aidomain.EventAssistantToken,
		aidomain.EventMessageCompleted,
		aidomain.EventDone,
	}
	if len(eventNames) != len(expected) {
		t.Fatalf("event count = %d, want %d (%v)", len(eventNames), len(expected), eventNames)
	}
	for i, name := range expected {
		if eventNames[i] != name {
			t.Fatalf("event[%d] = %q, want %q", i, eventNames[i], name)
		}
	}
}
