package aiselect

import (
	"context"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/service/system/aitool"
)

type fakeProgressiveToolSelector struct {
	groupResult aidomain.ToolGroupSelection
	groupErr    error
	toolResult  aidomain.ToolSelection
	toolErr     error
	groupCalls  int
	toolCalls   int
}

func (f *fakeProgressiveToolSelector) SelectGroup(
	context.Context,
	aidomain.ToolGroupSelectionInput,
) (aidomain.ToolGroupSelection, error) {
	f.groupCalls++
	return f.groupResult, f.groupErr
}

func (f *fakeProgressiveToolSelector) SelectTools(
	context.Context,
	aidomain.ToolSelectionInput,
) (aidomain.ToolSelection, error) {
	f.toolCalls++
	return f.toolResult, f.toolErr
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

type fakeAuthorization struct{}

func (f *fakeAuthorization) IsSuperAdmin(context.Context, uint) (bool, error) {
	return false, nil
}

func (f *fakeAuthorization) CheckUserCapabilityInOrg(context.Context, uint, uint, string) (bool, error) {
	return false, nil
}

func (f *fakeAuthorization) AuthorizeOrgCapability(context.Context, uint, uint, string) error {
	return nil
}

type fakeOJService struct{}

func (f *fakeOJService) GetRankingList(context.Context, uint, *request.OJRankingListReq) (*resp.OJRankingListResp, error) {
	return &resp.OJRankingListResp{}, nil
}

func (f *fakeOJService) GetUserStats(context.Context, uint, *request.OJStatsReq) (*resp.OJStatsResp, error) {
	return &resp.OJStatsResp{}, nil
}

func (f *fakeOJService) GetCurve(context.Context, uint, *request.OJCurveReq) (*resp.OJCurveResp, error) {
	return &resp.OJCurveResp{}, nil
}

func TestBuildAIToolExecutionPlanUsesDirectAnswerWithoutTools(t *testing.T) {
	planner, visibleTools, promptBuilder := newProgressivePlanTestPlanner(t)
	selector := &fakeProgressiveToolSelector{
		groupResult: aidomain.ToolGroupSelection{
			Decision: aidomain.ToolSelectionDecisionDirectAnswer,
			Reason:   "问题不需要工具",
		},
	}
	planner.selector = selector
	planner.promptBuilder = promptBuilder
	promptBuilder.output = "decision prompt"

	plan, err := planner.BuildExecutionPlan(
		context.Background(),
		"你好",
		nil,
		visibleTools,
		aidomain.AIToolPrincipal{UserID: 7},
	)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	if len(plan.Tools) != 0 {
		t.Fatalf("plan.Tools len = %d, want 0", len(plan.Tools))
	}
	if plan.DynamicSystemPrompt != "decision prompt" {
		t.Fatalf("plan.DynamicSystemPrompt = %q", plan.DynamicSystemPrompt)
	}
	if selector.groupCalls != 1 || selector.toolCalls != 0 {
		t.Fatalf("selector calls = (%d,%d), want (1,0)", selector.groupCalls, selector.toolCalls)
	}
}

func TestBuildAIToolExecutionPlanSelectsSubsetOfTools(t *testing.T) {
	planner, visibleTools, promptBuilder := newProgressivePlanTestPlanner(t)
	selector := &fakeProgressiveToolSelector{
		groupResult: aidomain.ToolGroupSelection{
			Decision: aidomain.ToolSelectionDecisionSelectGroup,
			GroupID:  aidomain.ToolGroupOJPersonal,
		},
		toolResult: aidomain.ToolSelection{
			SelectedToolNames: []string{"get_my_ranking"},
			Confidence:        aidomain.ToolSelectionConfidenceHigh,
		},
	}
	planner.selector = selector
	planner.promptBuilder = promptBuilder
	promptBuilder.output = "react prompt"

	plan, err := planner.BuildExecutionPlan(
		context.Background(),
		"帮我查我的排名",
		nil,
		visibleTools,
		aidomain.AIToolPrincipal{UserID: 7},
	)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	if len(plan.Tools) != 1 {
		t.Fatalf("plan.Tools len = %d, want 1", len(plan.Tools))
	}
	if plan.Tools[0].Spec().Name != "get_my_ranking" {
		t.Fatalf("selected tool = %q", plan.Tools[0].Spec().Name)
	}
	if selector.groupCalls != 1 || selector.toolCalls != 1 {
		t.Fatalf("selector calls = (%d,%d), want (1,1)", selector.groupCalls, selector.toolCalls)
	}
}

func TestBuildAIToolExecutionPlanExpandsWholeGroupWhenLowConfidence(t *testing.T) {
	planner, visibleTools, promptBuilder := newProgressivePlanTestPlanner(t)
	selector := &fakeProgressiveToolSelector{
		groupResult: aidomain.ToolGroupSelection{
			Decision: aidomain.ToolSelectionDecisionSelectGroup,
			GroupID:  aidomain.ToolGroupOJPersonal,
		},
		toolResult: aidomain.ToolSelection{
			SelectedToolNames: []string{"get_my_ranking"},
			Confidence:        aidomain.ToolSelectionConfidenceLow,
		},
	}
	planner.selector = selector
	planner.promptBuilder = promptBuilder
	promptBuilder.output = "react prompt"

	plan, err := planner.BuildExecutionPlan(
		context.Background(),
		"帮我看个人 OJ 表现",
		nil,
		visibleTools,
		aidomain.AIToolPrincipal{UserID: 7},
	)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	if len(plan.Tools) != 3 {
		t.Fatalf("plan.Tools len = %d, want 3", len(plan.Tools))
	}
}

func TestBuildAIToolExecutionPlanFallsBackWhenSelectorFails(t *testing.T) {
	planner, visibleTools, promptBuilder := newProgressivePlanTestPlanner(t)
	selector := &fakeProgressiveToolSelector{groupErr: context.DeadlineExceeded}
	planner.selector = selector
	planner.promptBuilder = promptBuilder
	promptBuilder.output = "fallback prompt"

	plan, err := planner.BuildExecutionPlan(
		context.Background(),
		"帮我查排名",
		nil,
		visibleTools,
		aidomain.AIToolPrincipal{UserID: 7},
	)
	if err != nil {
		t.Fatalf("BuildExecutionPlan() error = %v", err)
	}
	if len(plan.Tools) != len(visibleTools) {
		t.Fatalf("plan.Tools len = %d, want %d", len(plan.Tools), len(visibleTools))
	}
	if plan.DynamicSystemPrompt != "fallback prompt" {
		t.Fatalf("plan.DynamicSystemPrompt = %q", plan.DynamicSystemPrompt)
	}
}

func newProgressivePlanTestPlanner(t *testing.T) (*Planner, []aidomain.Tool, *fakePromptBuilder) {
	t.Helper()
	registry := aitool.NewRegistry(aitool.Deps{
		Authorization: &fakeAuthorization{},
		OJ:            &fakeOJService{},
	})
	visibleTools, err := registry.FilterVisibleTools(context.Background(), aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{UserID: 7},
	})
	if err != nil {
		t.Fatalf("FilterVisibleTools() error = %v", err)
	}
	if len(visibleTools) != 3 {
		t.Fatalf("visibleTools len = %d, want 3", len(visibleTools))
	}
	builder := &fakePromptBuilder{output: "prompt"}
	return NewPlanner(registry, nil, builder), visibleTools, builder
}
