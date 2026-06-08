package aitool

import (
	"context"

	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
)

// AuthorizationService 表示 AI tool 侧依赖的最小授权能力集合。
type AuthorizationService interface {
	IsSuperAdmin(ctx context.Context, userID uint) (bool, error)
	CheckUserCapabilityInOrg(ctx context.Context, userID, orgID uint, capabilityCode string) (bool, error)
	AuthorizeOrgCapability(ctx context.Context, operatorID, orgID uint, capabilityCode string) error
}

// OJService 表示 AI tool 查询个人或组织 OJ 统计时依赖的最小 OJ 能力。
type OJService interface {
	GetRankingList(ctx context.Context, userID uint, req *request.OJRankingListReq) (*resp.OJRankingListResp, error)
	GetUserStats(ctx context.Context, userID uint, req *request.OJStatsReq) (*resp.OJStatsResp, error)
	GetCurve(ctx context.Context, userID uint, req *request.OJCurveReq) (*resp.OJCurveResp, error)
}

// OJTaskService 表示 AI tool 查询 OJ 任务、执行和分析结果时依赖的最小任务能力。
type OJTaskService interface {
	AnalyzeTaskTitles(ctx context.Context, req *request.AnalyzeOJTaskTitlesReq) (*resp.OJTaskAnalyzeResp, error)
	GetTaskDetail(ctx context.Context, userID, taskID uint) (*resp.OJTaskDetailResp, error)
	GetTaskExecutionDetail(ctx context.Context, userID, taskID, executionID uint) (*resp.OJTaskExecutionResp, error)
	GetTaskExecutionUsers(
		ctx context.Context,
		userID, taskID, executionID uint,
		req *request.OJTaskExecutionUserListReq,
	) (*resp.OJTaskExecutionUserListResp, error)
	GetTaskExecutionUserDetail(
		ctx context.Context,
		userID, taskID, executionID, targetUserID uint,
	) (*resp.OJTaskExecutionUserDetailResp, error)
}

// ObservabilityService 表示 AI tool 查询 trace 和指标时依赖的最小观测能力。
type ObservabilityService interface {
	QueryMetrics(ctx context.Context, req *request.ObservabilityMetricsQueryReq) (*resp.ObservabilityMetricsQueryResp, error)
	QueryRuntimeMetrics(
		ctx context.Context,
		req *request.ObservabilityRuntimeMetricQueryReq,
	) (*resp.ObservabilityRuntimeMetricQueryResp, error)
	QueryTraceDetail(
		ctx context.Context,
		id string,
		idType string,
		limit int,
		offset int,
		includePayload bool,
		includeErrorDetail bool,
	) (*resp.ObservabilityTraceQueryResp, error)
	QueryTrace(ctx context.Context, req *request.ObservabilityTraceQueryReq) (*resp.ObservabilityTraceSummaryQueryResp, error)
}

// Deps 表示 AI tool registry 构建时需要的最小业务依赖。
type Deps struct {
	Authorization AuthorizationService
	OJ            OJService
	OJTask        OJTaskService
	Observability ObservabilityService
}
