package system

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
	registry := newAIToolRegistry(AIDeps{
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
	tool := newAIToolRegistry(AIDeps{
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
	tool := newAIToolRegistry(AIDeps{
		Authorization: auth,
		Observability: obsSvc,
	}).findTool("query_runtime_metrics")
	if tool == nil {
		t.Fatal("tool query_runtime_metrics not found")
	}

	_, err := tool.Call(context.Background(), aidomain.ToolCall{
		ID:            "call_3",
		Name:          "query_runtime_metrics",
		ArgumentsJSON: `{"metric":"job_duration"}`,
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
