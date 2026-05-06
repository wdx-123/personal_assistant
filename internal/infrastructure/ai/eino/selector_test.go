package eino

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestProgressiveToolSelectorSelectGroupParsesJSON(t *testing.T) {
	model := &fakeToolCallingChatModel{
		generateMsg: schema.AssistantMessage(`{"decision":"select_group","group_id":"oj_personal","reason":"用户在问个人 OJ 表现"}`, nil),
	}
	selector := &ProgressiveToolSelector{
		model:        model,
		systemPrompt: "selector system prompt",
	}

	result, err := selector.SelectGroup(context.Background(), aidomain.ToolGroupSelectionInput{
		Query: "帮我看下我的力扣排名",
		Groups: []aidomain.ToolGroupBrief{
			{ID: aidomain.ToolGroupOJPersonal, Summary: "个人 OJ"},
		},
	})
	if err != nil {
		t.Fatalf("SelectGroup() error = %v", err)
	}
	if result.Decision != aidomain.ToolSelectionDecisionSelectGroup {
		t.Fatalf("decision = %q", result.Decision)
	}
	if result.GroupID != aidomain.ToolGroupOJPersonal {
		t.Fatalf("group_id = %q", result.GroupID)
	}
	if len(model.generateInputs) != 1 {
		t.Fatalf("generateInputs = %d, want 1", len(model.generateInputs))
	}
	if got := model.generateInputs[0][1].Content; !strings.Contains(got, `"summary":"个人 OJ"`) {
		t.Fatalf("selector prompt = %q, want contains group brief", got)
	}
}

func TestProgressiveToolSelectorSelectToolsRejectsInvalidJSON(t *testing.T) {
	model := &fakeToolCallingChatModel{
		generateMsg: schema.AssistantMessage(`not-json`, nil),
	}
	selector := &ProgressiveToolSelector{model: model, systemPrompt: "selector"}

	_, err := selector.SelectTools(context.Background(), aidomain.ToolSelectionInput{
		Query: "帮我查指标",
		Group: aidomain.ToolGroupBrief{ID: aidomain.ToolGroupObservabilityMetrics},
		Tools: []aidomain.ToolBrief{
			{Name: "query_runtime_metrics", Summary: "runtime"},
		},
	})
	if err == nil {
		t.Fatal("SelectTools() error = nil, want invalid json")
	}
}

func TestProgressiveToolSelectorSelectToolsUsesBriefOnly(t *testing.T) {
	model := &fakeToolCallingChatModel{
		generateMsg: schema.AssistantMessage(`{"selected_tool_names":["query_runtime_metrics"],"confidence":"high"}`, nil),
	}
	selector := &ProgressiveToolSelector{model: model, systemPrompt: "selector"}

	_, err := selector.SelectTools(context.Background(), aidomain.ToolSelectionInput{
		Query: "帮我查运行时指标",
		Group: aidomain.ToolGroupBrief{
			ID:      aidomain.ToolGroupObservabilityMetrics,
			Summary: "查询运行时指标与 HTTP 观测指标。",
		},
		Tools: []aidomain.ToolBrief{
			{
				Name:          "query_runtime_metrics",
				Summary:       "查询运行时指标。",
				WhenToUse:     "用户要看运行时指标时使用。",
				RequiredSlots: []string{"metric"},
				DomainTags:    []string{"observability", "metrics"},
			},
		},
	})
	if err != nil {
		t.Fatalf("SelectTools() error = %v", err)
	}
	got := model.generateInputs[0][1].Content
	if !strings.Contains(got, `"required_slots":["metric"]`) {
		t.Fatalf("selector prompt = %q, want contains tool brief", got)
	}
	if strings.Contains(got, "format=rfc3339") || strings.Contains(got, `"format":"rfc3339"`) {
		t.Fatalf("selector prompt unexpectedly contains full schema detail: %q", got)
	}
}
