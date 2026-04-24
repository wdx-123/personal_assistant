package eino

import (
	"context"
	"errors"
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
	err        error
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
	if t.err != nil {
		return aidomain.ToolResult{}, t.err
	}
	return t.result, nil
}

func TestRuntimeStreamTextOnlyEmitsFinalContent(t *testing.T) {
	model := &fakeToolCallingChatModel{
		streams: [][]*schema.Message{
			{
				schema.AssistantMessage("你好，", nil),
				schema.AssistantMessage("请继续说明需求。", nil),
			},
		},
	}
	runtime := &Runtime{
		model:        model,
		systemPrompt: "base system prompt",
	}
	sink := &runtimeEventSinkStub{}

	result, err := runtime.Stream(context.Background(), aidomain.StreamInput{
		Content: "你好",
	}, sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.Content != "你好，请继续说明需求。" {
		t.Fatalf("result.Content = %q", result.Content)
	}

	expected := []aidomain.EventName{
		aidomain.EventConversationStarted,
		aidomain.EventAssistantToken,
		aidomain.EventAssistantToken,
		aidomain.EventMessageCompleted,
		aidomain.EventDone,
	}
	if len(sink.events) != len(expected) {
		t.Fatalf("event count = %d, want %d", len(sink.events), len(expected))
	}
	for i, name := range expected {
		if sink.events[i].Name != name {
			t.Fatalf("event[%d] = %q, want %q", i, sink.events[i].Name, name)
		}
	}
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

func TestRuntimeStreamWithToolsCanNaturallyAskForMissingParams(t *testing.T) {
	model := &fakeToolCallingChatModel{
		streams: [][]*schema.Message{
			{
				schema.AssistantMessage("你想查哪个平台，是 leetcode、luogu 还是 lanqiao？", nil),
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
	}
	sink := &runtimeEventSinkStub{}

	result, err := runtime.Stream(context.Background(), aidomain.StreamInput{
		Content:             "帮我看一下我的统计",
		DynamicSystemPrompt: "缺少平台时不要猜，直接追问。",
		Tools:               []aidomain.Tool{tool},
	}, sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if len(tool.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0", len(tool.calls))
	}
	if result.Content == "" {
		t.Fatal("result.Content = empty")
	}
	if sink.events[1].Name != aidomain.EventAssistantToken {
		t.Fatalf("event[1] = %q, want assistant_token", sink.events[1].Name)
	}
}

func TestRuntimeStreamWithToolsEmitsFailedTraceBeforeReturningError(t *testing.T) {
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
		err: errors.New("tool failed"),
	}
	sink := &runtimeEventSinkStub{}

	_, err := runtime.Stream(context.Background(), aidomain.StreamInput{
		Content: "帮我看 leetcode 统计",
		Tools:   []aidomain.Tool{tool},
	}, sink)
	if err == nil {
		t.Fatal("Stream() error = nil, want tool failure")
	}
	if len(sink.events) != 3 {
		t.Fatalf("event count = %d, want 3", len(sink.events))
	}
	if sink.events[1].Name != aidomain.EventToolCallStarted {
		t.Fatalf("event[1] = %q, want tool_call_started", sink.events[1].Name)
	}
	if sink.events[2].Name != aidomain.EventToolCallFinished {
		t.Fatalf("event[2] = %q, want tool_call_finished", sink.events[2].Name)
	}
	payload, ok := sink.events[2].Payload.(aidomain.ToolCallFinishedPayload)
	if !ok {
		t.Fatalf("event[2] payload type = %T", sink.events[2].Payload)
	}
	if payload.Status != "failed" {
		t.Fatalf("payload.Status = %q, want failed", payload.Status)
	}
}
