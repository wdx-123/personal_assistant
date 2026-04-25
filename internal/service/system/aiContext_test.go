package system

import (
	"context"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

type fakeContextTool struct {
	spec aidomain.ToolSpec
}

func (t *fakeContextTool) Spec() aidomain.ToolSpec {
	return t.spec
}

func (t *fakeContextTool) Call(context.Context, aidomain.ToolCall, aidomain.ToolCallContext) (aidomain.ToolResult, error) {
	return aidomain.ToolResult{}, nil
}

type fakeMemoryProvider struct {
	output []aidomain.Message
	calls  int
}

func (f *fakeMemoryProvider) RecallMessages(context.Context, aiMemoryRecallInput) ([]aidomain.Message, error) {
	f.calls++
	return f.output, nil
}

type fakeContextCompressor struct {
	output []aidomain.Message
	calls  int
}

func (f *fakeContextCompressor) CompressMessages(context.Context, aiContextCompressionInput) ([]aidomain.Message, error) {
	f.calls++
	return f.output, nil
}

type fakePromptBuilder struct {
	output string
	calls  int
}

func (f *fakePromptBuilder) BuildDynamicPrompt([]aidomain.Tool, aidomain.AIToolPrincipal) string {
	f.calls++
	return f.output
}

func (f *fakePromptBuilder) BuildDecisionPrompt(
	aidomain.ToolSelectionDecision,
	string,
	[]string,
) string {
	f.calls++
	return f.output
}

func TestDefaultAIContextAssemblerUsesStoredHistory(t *testing.T) {
	assembler := newAIContextAssembler(AIDeps{})
	tool := &fakeContextTool{
		spec: aidomain.ToolSpec{
			Name:        "get_my_oj_stats",
			Description: "获取当前登录用户在指定 OJ 平台上的个人统计。",
		},
	}
	orgID := uint(9)

	snapshot, err := assembler.Build(context.Background(), aiContextBuildArgs{
		ConversationID: "conv_1",
		UserID:         7,
		Query:          "帮我看一下排名",
		StoredMessages: []*entity.AIMessage{
			{ID: "msg_1", Role: aidomain.RoleUser, Content: "第一句"},
			{ID: "msg_2", Role: aidomain.RoleAssistant, Content: "第二句"},
			{ID: "msg_3", Role: aidomain.RoleUser, Content: "   "},
		},
		VisibleTools: []aidomain.Tool{tool},
		ToolCallCtx: aidomain.ToolCallContext{
			Principal: aidomain.AIToolPrincipal{
				UserID:       7,
				CurrentOrgID: &orgID,
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(snapshot.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(snapshot.History))
	}
	if snapshot.History[0].Role != aidomain.RoleUser {
		t.Fatalf("history[0].Role = %q", snapshot.History[0].Role)
	}
	if snapshot.History[1].Role != aidomain.RoleAssistant {
		t.Fatalf("history[1].Role = %q", snapshot.History[1].Role)
	}
}

func TestDefaultAIContextAssemblerCallsOptionalProviders(t *testing.T) {
	memory := &fakeMemoryProvider{
		output: []aidomain.Message{
			{ID: "mem_1", Role: aidomain.RoleAssistant, Content: "记忆片段"},
		},
	}
	compressor := &fakeContextCompressor{
		output: []aidomain.Message{
			{ID: "cmp_1", Role: aidomain.RoleAssistant, Content: "压缩后的上下文"},
		},
	}
	assembler := newAIContextAssembler(AIDeps{
		Memory:     memory,
		Compressor: compressor,
	})

	snapshot, err := assembler.Build(context.Background(), aiContextBuildArgs{
		ConversationID: "conv_2",
		UserID:         8,
		Query:          "帮我查一下统计",
		StoredMessages: []*entity.AIMessage{
			{ID: "msg_1", Role: aidomain.RoleUser, Content: "原始历史"},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if memory.calls != 1 {
		t.Fatalf("memory calls = %d, want 1", memory.calls)
	}
	if compressor.calls != 1 {
		t.Fatalf("compressor calls = %d, want 1", compressor.calls)
	}
	if len(snapshot.History) != 1 || snapshot.History[0].ID != "cmp_1" {
		t.Fatalf("snapshot.History = %+v, want compressed output", snapshot.History)
	}
}
