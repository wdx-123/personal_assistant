package aitool

import (
	"context"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	bizerrors "personal_assistant/pkg/errors"
)

type fakeAIToolAuthorization struct {
	superAdmin     bool
	capabilities   map[uint]map[string]bool
	checkCalls     int
	authorizeCalls int
}

func (f *fakeAIToolAuthorization) IsSuperAdmin(context.Context, uint) (bool, error) {
	return f.superAdmin, nil
}

func (f *fakeAIToolAuthorization) CheckUserCapabilityInOrg(
	_ context.Context,
	_ uint,
	orgID uint,
	capabilityCode string,
) (bool, error) {
	f.checkCalls++
	return f.hasCapability(orgID, capabilityCode), nil
}

func (f *fakeAIToolAuthorization) AuthorizeOrgCapability(
	_ context.Context,
	_ uint,
	orgID uint,
	capabilityCode string,
) error {
	f.authorizeCalls++
	if f.superAdmin || f.hasCapability(orgID, capabilityCode) {
		return nil
	}
	return bizerrors.New(bizerrors.CodePermissionDenied)
}

func (f *fakeAIToolAuthorization) hasCapability(orgID uint, capabilityCode string) bool {
	if f == nil {
		return false
	}
	if caps, ok := f.capabilities[orgID]; ok {
		return caps[capabilityCode]
	}
	return false
}

type fakeAIToolOJService struct{}

func (f *fakeAIToolOJService) GetRankingList(
	context.Context,
	uint,
	*request.OJRankingListReq,
) (*resp.OJRankingListResp, error) {
	return &resp.OJRankingListResp{}, nil
}

func (f *fakeAIToolOJService) GetUserStats(
	context.Context,
	uint,
	*request.OJStatsReq,
) (*resp.OJStatsResp, error) {
	return &resp.OJStatsResp{}, nil
}

func (f *fakeAIToolOJService) GetCurve(
	context.Context,
	uint,
	*request.OJCurveReq,
) (*resp.OJCurveResp, error) {
	return &resp.OJCurveResp{}, nil
}

type fakeAIToolOJTaskService struct {
	taskDetailResp          *resp.OJTaskDetailResp
	taskExecutionResp       *resp.OJTaskExecutionResp
	taskExecutionUsersResp  *resp.OJTaskExecutionUserListResp
	taskExecutionUserResp   *resp.OJTaskExecutionUserDetailResp
	analyzeResp             *resp.OJTaskAnalyzeResp
	taskDetailCalls         int
	taskExecutionCalls      int
	taskExecutionUsersCalls int
	taskExecutionUserCalls  int
	analyzeCalls            int
}

func (f *fakeAIToolOJTaskService) AnalyzeTaskTitles(
	context.Context,
	*request.AnalyzeOJTaskTitlesReq,
) (*resp.OJTaskAnalyzeResp, error) {
	f.analyzeCalls++
	if f.analyzeResp == nil {
		return &resp.OJTaskAnalyzeResp{}, nil
	}
	return f.analyzeResp, nil
}

func (f *fakeAIToolOJTaskService) GetTaskDetail(
	context.Context,
	uint,
	uint,
) (*resp.OJTaskDetailResp, error) {
	f.taskDetailCalls++
	if f.taskDetailResp == nil {
		return &resp.OJTaskDetailResp{}, nil
	}
	return f.taskDetailResp, nil
}

func (f *fakeAIToolOJTaskService) GetTaskExecutionDetail(
	context.Context,
	uint,
	uint,
	uint,
) (*resp.OJTaskExecutionResp, error) {
	f.taskExecutionCalls++
	if f.taskExecutionResp == nil {
		return &resp.OJTaskExecutionResp{}, nil
	}
	return f.taskExecutionResp, nil
}

func (f *fakeAIToolOJTaskService) GetTaskExecutionUsers(
	context.Context,
	uint,
	uint,
	uint,
	*request.OJTaskExecutionUserListReq,
) (*resp.OJTaskExecutionUserListResp, error) {
	f.taskExecutionUsersCalls++
	if f.taskExecutionUsersResp == nil {
		return &resp.OJTaskExecutionUserListResp{}, nil
	}
	return f.taskExecutionUsersResp, nil
}

func (f *fakeAIToolOJTaskService) GetTaskExecutionUserDetail(
	context.Context,
	uint,
	uint,
	uint,
	uint,
) (*resp.OJTaskExecutionUserDetailResp, error) {
	f.taskExecutionUserCalls++
	if f.taskExecutionUserResp == nil {
		return &resp.OJTaskExecutionUserDetailResp{}, nil
	}
	return f.taskExecutionUserResp, nil
}

type fakeAIToolObservabilityService struct {
	runtimeMetricsResp *resp.ObservabilityRuntimeMetricQueryResp
	runtimeCalls       int
}

func (f *fakeAIToolObservabilityService) QueryMetrics(
	context.Context,
	*request.ObservabilityMetricsQueryReq,
) (*resp.ObservabilityMetricsQueryResp, error) {
	return &resp.ObservabilityMetricsQueryResp{}, nil
}

func (f *fakeAIToolObservabilityService) QueryRuntimeMetrics(
	context.Context,
	*request.ObservabilityRuntimeMetricQueryReq,
) (*resp.ObservabilityRuntimeMetricQueryResp, error) {
	f.runtimeCalls++
	if f.runtimeMetricsResp == nil {
		return &resp.ObservabilityRuntimeMetricQueryResp{}, nil
	}
	return f.runtimeMetricsResp, nil
}

func (f *fakeAIToolObservabilityService) QueryTraceDetail(
	context.Context,
	string,
	string,
	int,
	int,
	bool,
	bool,
) (*resp.ObservabilityTraceQueryResp, error) {
	return &resp.ObservabilityTraceQueryResp{}, nil
}

func (f *fakeAIToolObservabilityService) QueryTrace(
	context.Context,
	*request.ObservabilityTraceQueryReq,
) (*resp.ObservabilityTraceSummaryQueryResp, error) {
	return &resp.ObservabilityTraceSummaryQueryResp{}, nil
}

func TestAIToolRegistryFilterVisibleToolsByPolicy(t *testing.T) {
	orgID := uint(10)
	auth := &fakeAIToolAuthorization{
		capabilities: map[uint]map[string]bool{
			orgID: {consts.CapabilityCodeOJTaskManage: true},
		},
	}
	registry := newAIToolRegistry(Deps{
		Authorization: auth,
		OJ:            &fakeAIToolOJService{},
		OJTask:        &fakeAIToolOJTaskService{},
		Observability: &fakeAIToolObservabilityService{},
	})

	tools, err := registry.FilterVisibleTools(context.Background(), aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{
			UserID:       1,
			CurrentOrgID: &orgID,
		},
	})
	if err != nil {
		t.Fatalf("FilterVisibleTools() error = %v", err)
	}

	names := toolNames(tools)
	assertContainsTool(t, names, "get_my_oj_stats")
	assertContainsTool(t, names, "get_org_ranking_summary")
	assertNotContainsTool(t, names, "query_runtime_metrics")

	auth.superAdmin = true
	tools, err = registry.FilterVisibleTools(context.Background(), aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{
			UserID:       1,
			CurrentOrgID: &orgID,
			IsSuperAdmin: true,
		},
	})
	if err != nil {
		t.Fatalf("FilterVisibleTools() error = %v", err)
	}

	names = toolNames(tools)
	assertContainsTool(t, names, "query_runtime_metrics")
}

func TestAIToolExecutionReauthorizesTaskOrgCapability(t *testing.T) {
	taskSvc := &fakeAIToolOJTaskService{
		taskDetailResp: &resp.OJTaskDetailResp{
			TaskID: 1,
			Orgs: []*resp.OJTaskOrgItemResp{
				{OrgID: 9, OrgName: "org-9"},
			},
		},
		taskExecutionResp: &resp.OJTaskExecutionResp{
			TaskID:      1,
			ExecutionID: 2,
			Status:      "succeeded",
		},
	}
	auth := &fakeAIToolAuthorization{
		capabilities: map[uint]map[string]bool{
			9: {consts.CapabilityCodeOJTaskManage: true},
		},
	}
	tool := newAIToolRegistry(Deps{
		Authorization: auth,
		OJTask:        taskSvc,
	}).findTool("get_task_execution_summary")
	if tool == nil {
		t.Fatal("tool get_task_execution_summary not found")
	}

	_, err := tool.Call(context.Background(), aidomain.ToolCall{
		ID:            "call_1",
		Name:          "get_task_execution_summary",
		ArgumentsJSON: `{"task_id":1,"execution_id":2}`,
	}, aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{UserID: 7},
	})
	if err != nil {
		t.Fatalf("tool.Call() error = %v", err)
	}
	if taskSvc.taskDetailCalls != 1 {
		t.Fatalf("taskDetailCalls = %d, want 1", taskSvc.taskDetailCalls)
	}
	if taskSvc.taskExecutionCalls != 1 {
		t.Fatalf("taskExecutionCalls = %d, want 1", taskSvc.taskExecutionCalls)
	}
	if auth.authorizeCalls != 1 {
		t.Fatalf("authorizeCalls = %d, want 1", auth.authorizeCalls)
	}

	taskSvc.taskDetailResp = &resp.OJTaskDetailResp{
		TaskID: 1,
		Orgs: []*resp.OJTaskOrgItemResp{
			{OrgID: 10, OrgName: "org-10"},
		},
	}
	_, err = tool.Call(context.Background(), aidomain.ToolCall{
		ID:            "call_2",
		Name:          "get_task_execution_summary",
		ArgumentsJSON: `{"task_id":1,"execution_id":2}`,
	}, aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{UserID: 7},
	})
	if bizErr := bizerrors.FromError(err); bizErr == nil || bizErr.Code != bizerrors.CodePermissionDenied {
		t.Fatalf("tool.Call() error = %v, want permission denied", err)
	}
	if taskSvc.taskExecutionCalls != 1 {
		t.Fatalf("taskExecutionCalls after denied call = %d, want still 1", taskSvc.taskExecutionCalls)
	}
}

func TestAIToolSuperAdminExecutionDeniedWhenNotSuperAdmin(t *testing.T) {
	auth := &fakeAIToolAuthorization{}
	obsSvc := &fakeAIToolObservabilityService{}
	tool := newAIToolRegistry(Deps{
		Authorization: auth,
		Observability: obsSvc,
	}).findTool("query_runtime_metrics")
	if tool == nil {
		t.Fatal("tool query_runtime_metrics not found")
	}

	_, err := tool.Call(context.Background(), aidomain.ToolCall{
		ID:            "call_3",
		Name:          "query_runtime_metrics",
		ArgumentsJSON: `{"metric":"outbox_events_total","status":"pending","granularity":"5m"}`,
	}, aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{UserID: 9},
	})
	if bizErr := bizerrors.FromError(err); bizErr == nil || bizErr.Code != bizerrors.CodePermissionDenied {
		t.Fatalf("tool.Call() error = %v, want permission denied", err)
	}
	if obsSvc.runtimeCalls != 0 {
		t.Fatalf("runtimeCalls = %d, want 0", obsSvc.runtimeCalls)
	}
}

func toolNames(tools []aidomain.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		names = append(names, tool.Spec().Name)
	}
	return names
}

func assertContainsTool(t *testing.T, names []string, target string) {
	t.Helper()
	for _, name := range names {
		if name == target {
			return
		}
	}
	t.Fatalf("tool %s not found in %v", target, names)
}

func assertNotContainsTool(t *testing.T, names []string, target string) {
	t.Helper()
	for _, name := range names {
		if name == target {
			t.Fatalf("tool %s unexpectedly found in %v", target, names)
		}
	}
}

func TestAIToolRegistrySchemasExposeDetailedConstraints(t *testing.T) {
	registry := newAIToolRegistry(Deps{
		Authorization: &fakeAIToolAuthorization{superAdmin: true},
		OJ:            &fakeAIToolOJService{},
		OJTask:        &fakeAIToolOJTaskService{},
		Observability: &fakeAIToolObservabilityService{},
	})

	checks := []struct {
		toolName string
		param    string
		assert   func(t *testing.T, param aidomain.ToolParameter)
	}{
		{
			toolName: "get_my_ranking",
			param:    "platform",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertEnum(t, param, []string{"leetcode", "luogu", "lanqiao"})
				assertExamplesPresent(t, param)
			},
		},
		{
			toolName: "get_my_ranking",
			param:    "scope",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertEnum(t, param, []string{"current_org", "all_members"})
				if param.DefaultValue != "current_org" {
					t.Fatalf("scope default = %q, want current_org", param.DefaultValue)
				}
			},
		},
		{
			toolName: "get_org_ranking_summary",
			param:    "page_size",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertMinMax(t, param, 1, 100)
				if param.DefaultValue != "20" {
					t.Fatalf("page_size default = %q, want 20", param.DefaultValue)
				}
			},
		},
		{
			toolName: "list_task_execution_users",
			param:    "username",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				if param.MaxLength == nil || *param.MaxLength != 50 {
					t.Fatalf("username max_length = %v, want 50", param.MaxLength)
				}
			},
		},
		{
			toolName: "query_trace_detail_by_request_id",
			param:    "request_id",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				if param.MinLength == nil || *param.MinLength != 1 {
					t.Fatalf("request_id min_length = %v, want 1", param.MinLength)
				}
				assertExamplesPresent(t, param)
			},
		},
		{
			toolName: "query_trace_summary",
			param:    "root_stage",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertEnum(t, param, []string{"http.request", "task", "all"})
				if param.DefaultValue != "http.request" {
					t.Fatalf("root_stage default = %q, want http.request", param.DefaultValue)
				}
			},
		},
		{
			toolName: "query_trace_summary",
			param:    "start_at",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				if param.Format != aidomain.ToolParameterFormatRFC3339 {
					t.Fatalf("start_at format = %q, want RFC3339", param.Format)
				}
			},
		},
		{
			toolName: "query_runtime_metrics",
			param:    "granularity",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertEnum(t, param, []string{"1m", "5m", "1h", "1d"})
				if param.DefaultValue != "5m" {
					t.Fatalf("granularity default = %q, want 5m", param.DefaultValue)
				}
			},
		},
		{
			toolName: "query_runtime_metrics",
			param:    "limit",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertMinMax(t, param, 1, 2000)
			},
		},
		{
			toolName: "query_observability_metrics",
			param:    "granularity",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertEnum(t, param, []string{"1m", "5m", "1d", "1w"})
			},
		},
		{
			toolName: "query_observability_metrics",
			param:    "start_at",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				if param.Format != aidomain.ToolParameterFormatRFC3339 {
					t.Fatalf("start_at format = %q, want RFC3339", param.Format)
				}
			},
		},
		{
			toolName: "query_observability_metrics",
			param:    "limit",
			assert: func(t *testing.T, param aidomain.ToolParameter) {
				assertMinMax(t, param, 1, 50000)
				if param.DefaultValue != "5000" {
					t.Fatalf("limit default = %q, want 5000", param.DefaultValue)
				}
			},
		},
	}

	for _, tc := range checks {
		t.Run(tc.toolName+"_"+tc.param, func(t *testing.T) {
			tool := registry.findTool(tc.toolName)
			if tool == nil {
				t.Fatalf("tool %s not found", tc.toolName)
			}
			param := mustFindParam(t, tool.Spec().Parameters, tc.param)
			tc.assert(t, param)
		})
	}

	analyzeTool := registry.findTool("analyze_task_titles")
	if analyzeTool == nil {
		t.Fatal("tool analyze_task_titles not found")
	}
	itemsParam := mustFindParam(t, analyzeTool.Spec().Parameters, "items")
	if itemsParam.MinItems == nil || *itemsParam.MinItems != 1 {
		t.Fatalf("items min_items = %v, want 1", itemsParam.MinItems)
	}
	if itemsParam.Items == nil {
		t.Fatal("items param missing item schema")
	}
	platformParam := mustFindParam(t, itemsParam.Items.Properties, "platform")
	assertEnum(t, platformParam, []string{"leetcode", "luogu", "lanqiao"})
	titleParam := mustFindParam(t, itemsParam.Items.Properties, "title")
	if titleParam.MaxLength == nil || *titleParam.MaxLength != 255 {
		t.Fatalf("title max_length = %v, want 255", titleParam.MaxLength)
	}
}

func TestAIToolRegistryProgressiveMetadataAndExpansion(t *testing.T) {
	registry := newAIToolRegistry(Deps{
		Authorization: &fakeAIToolAuthorization{},
		OJ:            &fakeAIToolOJService{},
		OJTask:        &fakeAIToolOJTaskService{},
		Observability: &fakeAIToolObservabilityService{},
	})
	visibleTools, err := registry.FilterVisibleTools(context.Background(), aidomain.ToolCallContext{
		Principal: aidomain.AIToolPrincipal{
			UserID:       1,
			IsSuperAdmin: true,
		},
	})
	if err != nil {
		t.Fatalf("FilterVisibleTools() error = %v", err)
	}

	groupBriefs := registry.ListVisibleToolGroupBriefs(visibleTools)
	if len(groupBriefs) != 5 {
		t.Fatalf("groupBriefs len = %d, want 5", len(groupBriefs))
	}

	personalBriefs := registry.ListVisibleToolBriefsByGroup(visibleTools, aidomain.ToolGroupOJPersonal)
	if len(personalBriefs) != 3 {
		t.Fatalf("personalBriefs len = %d, want 3", len(personalBriefs))
	}
	if personalBriefs[0].Name == "" || personalBriefs[0].Summary == "" || personalBriefs[0].WhenToUse == "" {
		t.Fatalf("personalBriefs[0] = %+v, want populated brief", personalBriefs[0])
	}

	selected := registry.ExpandVisibleToolsByNames(visibleTools, []string{"get_my_oj_curve", "get_my_ranking"})
	if len(selected) != 2 {
		t.Fatalf("selected len = %d, want 2", len(selected))
	}
	if selected[0].Spec().Name != "get_my_oj_curve" || selected[1].Spec().Name != "get_my_ranking" {
		t.Fatalf("selected tools = [%s, %s]", selected[0].Spec().Name, selected[1].Spec().Name)
	}

	groupExpanded := registry.ExpandVisibleToolsByGroup(visibleTools, aidomain.ToolGroupObservabilityMetrics)
	if len(groupExpanded) != 2 {
		t.Fatalf("groupExpanded len = %d, want 2", len(groupExpanded))
	}
}

func TestAIValidateRuntimeMetricsArgs(t *testing.T) {
	if err := aiValidateRuntimeMetricsArgs(aiRuntimeMetricsArgs{
		Metric:      "task_execution_total",
		Status:      "published",
		StartAt:     "2026-04-24T09:20:00Z",
		EndAt:       "2026-04-24T10:20:00Z",
		Granularity: "5m",
	}); err == nil {
		t.Fatal("aiValidateRuntimeMetricsArgs() error = nil, want invalid status")
	}

	if err := aiValidateRuntimeMetricsArgs(aiRuntimeMetricsArgs{
		Metric:      "task_duration_seconds",
		Status:      "success",
		StartAt:     "2026-04-01T09:20:00Z",
		EndAt:       "2026-04-24T10:20:00Z",
		Granularity: "5m",
	}); err == nil {
		t.Fatal("aiValidateRuntimeMetricsArgs() error = nil, want invalid duration range")
	}

	if err := aiValidateRuntimeMetricsArgs(aiRuntimeMetricsArgs{
		Metric:      "event_consume_total",
		Status:      "error",
		StartAt:     "2026-04-24T09:20:00Z",
		EndAt:       "2026-04-24T10:20:00Z",
		Granularity: "5m",
	}); err != nil {
		t.Fatalf("aiValidateRuntimeMetricsArgs() error = %v", err)
	}
}

func TestAIValidateObservabilityMetricsArgs(t *testing.T) {
	if err := aiValidateObservabilityMetricsArgs(aiObservabilityMetricsArgs{
		Granularity: "1m",
		StartAt:     "2026-04-24T10:20:00Z",
		EndAt:       "2026-04-24T09:20:00Z",
	}); err == nil {
		t.Fatal("aiValidateObservabilityMetricsArgs() error = nil, want invalid time range")
	}

	if err := aiValidateObservabilityMetricsArgs(aiObservabilityMetricsArgs{
		Granularity: "1m",
		StartAt:     "2026-04-24T09:20:00Z",
		EndAt:       "2026-04-24T10:20:00Z",
		StatusClass: 7,
	}); err == nil {
		t.Fatal("aiValidateObservabilityMetricsArgs() error = nil, want invalid status_class")
	}

	if err := aiValidateObservabilityMetricsArgs(aiObservabilityMetricsArgs{
		Granularity: "1m",
		StartAt:     "2026-04-24T09:20:00Z",
		EndAt:       "2026-04-24T10:20:00Z",
		StatusClass: 2,
	}); err != nil {
		t.Fatalf("aiValidateObservabilityMetricsArgs() error = %v", err)
	}
}

func TestAIValidateTraceSummaryArgs(t *testing.T) {
	if err := aiValidateTraceSummaryArgs(aiTraceSummaryArgs{
		StartAt: "2026-04-24T10:20:00Z",
		EndAt:   "2026-04-24T09:20:00Z",
	}); err == nil {
		t.Fatal("aiValidateTraceSummaryArgs() error = nil, want invalid time range")
	}

	if err := aiValidateTraceSummaryArgs(aiTraceSummaryArgs{
		StartAt: "2026-04-24T09:20:00Z",
		EndAt:   "2026-04-24T10:20:00Z",
	}); err != nil {
		t.Fatalf("aiValidateTraceSummaryArgs() error = %v", err)
	}
}

func mustFindParam(t *testing.T, params []aidomain.ToolParameter, name string) aidomain.ToolParameter {
	t.Helper()
	for _, param := range params {
		if param.Name == name {
			return param
		}
	}
	t.Fatalf("param %s not found", name)
	return aidomain.ToolParameter{}
}

func assertEnum(t *testing.T, param aidomain.ToolParameter, want []string) {
	t.Helper()
	if len(param.Enum) != len(want) {
		t.Fatalf("%s enum = %v, want %v", param.Name, param.Enum, want)
	}
	for _, item := range want {
		found := false
		for _, got := range param.Enum {
			if got == item {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s enum missing %q in %v", param.Name, item, param.Enum)
		}
	}
}

func assertMinMax(t *testing.T, param aidomain.ToolParameter, min float64, max float64) {
	t.Helper()
	if param.Minimum == nil || *param.Minimum != min {
		t.Fatalf("%s min = %v, want %v", param.Name, param.Minimum, min)
	}
	if param.Maximum == nil || *param.Maximum != max {
		t.Fatalf("%s max = %v, want %v", param.Name, param.Maximum, max)
	}
}

func assertExamplesPresent(t *testing.T, param aidomain.ToolParameter) {
	t.Helper()
	if len(param.Examples) == 0 {
		t.Fatalf("%s examples = empty", param.Name)
	}
}
