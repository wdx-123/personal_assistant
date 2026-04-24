package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	bizerrors "personal_assistant/pkg/errors"
)

// aiAuthorizationService 表示 AI tool 侧依赖的最小授权能力集合。
// 它只暴露可见性过滤和执行前鉴权必需的方法，不泄露完整授权服务细节。
type aiAuthorizationService interface {
	IsSuperAdmin(ctx context.Context, userID uint) (bool, error)
	CheckUserCapabilityInOrg(ctx context.Context, userID, orgID uint, capabilityCode string) (bool, error)
	AuthorizeOrgCapability(ctx context.Context, operatorID, orgID uint, capabilityCode string) error
}

// aiOJService 表示 AI tool 查询个人或组织 OJ 统计时依赖的最小 OJ 能力。
type aiOJService interface {
	GetRankingList(ctx context.Context, userID uint, req *request.OJRankingListReq) (*resp.OJRankingListResp, error)
	GetUserStats(ctx context.Context, userID uint, req *request.OJStatsReq) (*resp.OJStatsResp, error)
	GetCurve(ctx context.Context, userID uint, req *request.OJCurveReq) (*resp.OJCurveResp, error)
}

// aiOJTaskService 表示 AI tool 查询 OJ 任务、执行和分析结果时依赖的最小任务能力。
type aiOJTaskService interface {
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

// aiObservabilityService 表示 AI tool 查询 trace 和指标时依赖的最小观测能力。
type aiObservabilityService interface {
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

// AIDeps 表示 AIService 构建 tool loop 时需要的最小服务依赖。
type AIDeps struct {
	// Authorization 提供工具可见性过滤和执行前二次鉴权所需的授权能力。
	Authorization aiAuthorizationService
	// OJ 提供个人排行、统计和曲线等 OJ 查询能力。
	OJ aiOJService
	// OJTask 提供任务执行和题目分析等 OJ 任务能力。
	OJTask aiOJTaskService
	// Observability 提供 trace 和指标查询能力。
	Observability aiObservabilityService
	// Memory 提供可选的额外记忆召回能力；未注入时保持只读历史消息路径。
	Memory aiMemoryProvider
	// Compressor 提供可选的上下文压缩能力；未注入时保持原始历史消息。
	Compressor aiContextCompressor
	// PromptBuilder 允许调用方替换默认动态 prompt 构造逻辑。
	PromptBuilder aiPromptBuilder
}

// aiToolPolicyKind 表示工具可见性和执行前鉴权采用的策略类型。
type aiToolPolicyKind string

const (
	// aiToolPolicySelfOnly 表示工具只允许围绕当前登录用户自己的数据执行。
	aiToolPolicySelfOnly aiToolPolicyKind = "self_only"
	// aiToolPolicyOrgCapability 表示工具要求当前用户对目标组织具备指定 capability。
	aiToolPolicyOrgCapability aiToolPolicyKind = "org_capability"
	// aiToolPolicySuperAdminOnly 表示工具只允许超级管理员使用。
	aiToolPolicySuperAdminOnly aiToolPolicyKind = "super_admin_only"
)

// aiToolPolicy 描述单个工具的访问策略。
type aiToolPolicy struct {
	// Kind 表示当前工具使用哪一类访问控制策略。
	Kind aiToolPolicyKind
	// CapabilityCode 表示组织能力策略下要求的 capability code。
	CapabilityCode string
}

// aiServiceTool 表示 service 层注册的一个具体 AI tool 实现。
type aiServiceTool struct {
	// spec 是暴露给 runtime 和模型的稳定工具协议。
	spec aidomain.ToolSpec
	// policy 描述工具的可见性和执行前鉴权要求。
	policy aiToolPolicy
	// call 承载工具的实际业务执行逻辑。
	call func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error)
}

// Spec 返回工具的稳定协议定义。
func (t *aiServiceTool) Spec() aidomain.ToolSpec {
	// spec 在注册阶段就已固定，不在运行时动态变更。
	return t.spec
}

// Call 执行具体工具逻辑。
func (t *aiServiceTool) Call(
	ctx context.Context,
	call aidomain.ToolCall,
	callCtx aidomain.ToolCallContext,
) (aidomain.ToolResult, error) {
	// 工具或执行闭包未初始化时，直接返回内部错误，避免 nil 调用 panic。
	if t == nil || t.call == nil {
		return aidomain.ToolResult{}, bizerrors.NewWithMsg(bizerrors.CodeInternalError, "AI tool 未正确初始化")
	}

	// 真正的业务逻辑交给具体工具闭包执行。
	return t.call(ctx, call, callCtx)
}

// aiToolRegistry 负责注册工具、过滤可见性，并提供执行前定位能力。
type aiToolRegistry struct {
	// authorization 用于按 principal 判断组织能力类工具是否可见。
	authorization aiAuthorizationService
	// tools 保存当前进程可注册的全部工具目录。
	tools []*aiServiceTool
}

// newAIToolRegistry 创建 AI tool 注册表。
func newAIToolRegistry(deps AIDeps) *aiToolRegistry {
	// 先创建空注册表，并挂上可见性过滤会用到的授权服务。
	r := &aiToolRegistry{authorization: deps.Authorization}
	// 再根据当前注入依赖构建完整工具目录。
	r.tools = r.buildCatalog(deps)
	return r
}

// buildCatalog 根据当前依赖拼出本进程真正可提供的工具目录。
func (r *aiToolRegistry) buildCatalog(deps AIDeps) []*aiServiceTool {
	// 先按预估容量创建切片，减少追加时的扩容次数。
	tools := make([]*aiServiceTool, 0, 11)
	if deps.OJ != nil {
		// 个人 OJ 工具只依赖 OJService，本身不需要额外授权服务。
		tools = append(tools,
			newAIGetMyRankingTool(deps.OJ),
			newAIGetMyOJStatsTool(deps.OJ),
			newAIGetMyOJCurveTool(deps.OJ),
		)
	}
	if deps.OJ != nil && deps.Authorization != nil {
		// 组织排行榜工具既需要 OJ 数据，也需要组织能力鉴权。
		tools = append(tools, newAIGetOrgRankingSummaryTool(deps.OJ, deps.Authorization))
	}
	if deps.OJTask != nil && deps.Authorization != nil {
		// 任务执行类工具统一依赖 OJTaskService 和授权服务。
		tools = append(tools,
			newAIGetTaskExecutionSummaryTool(deps.OJTask, deps.Authorization),
			newAIListTaskExecutionUsersTool(deps.OJTask, deps.Authorization),
			newAIGetTaskExecutionUserDetailTool(deps.OJTask, deps.Authorization),
			newAIAnalyzeTaskTitlesTool(deps.OJTask, deps.Authorization),
		)
	}
	if deps.Observability != nil && deps.Authorization != nil {
		// 观测类工具要求 super admin，因此同时依赖观测服务和授权服务。
		tools = append(tools,
			newAIQueryTraceDetailByRequestIDTool(deps.Observability, deps.Authorization),
			newAIQueryTraceSummaryTool(deps.Observability, deps.Authorization),
			newAIQueryRuntimeMetricsTool(deps.Observability, deps.Authorization),
			newAIQueryObservabilityMetricsTool(deps.Observability, deps.Authorization),
		)
	}
	return tools
}

// FilterVisibleTools 按本轮 principal 过滤出模型真正可见的工具集合。
func (r *aiToolRegistry) FilterVisibleTools(
	ctx context.Context,
	callCtx aidomain.ToolCallContext,
) ([]aidomain.Tool, error) {
	// 没有可注册工具时直接退化成无工具模式。
	if r == nil || len(r.tools) == 0 {
		return nil, nil
	}

	// visible 收集通过 policy 过滤的工具，供 runtime 暴露给模型。
	visible := make([]aidomain.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		// 每个工具都按其 policy 和当前 principal 做一次可见性判断。
		ok, err := r.isVisible(ctx, tool.policy, callCtx.Principal)
		if err != nil {
			return nil, err
		}
		if ok {
			visible = append(visible, tool)
		}
	}
	return visible, nil
}

// isVisible 判断某个 policy 在当前 principal 下是否应该暴露给模型。
func (r *aiToolRegistry) isVisible(
	ctx context.Context,
	policy aiToolPolicy,
	principal aidomain.AIToolPrincipal,
) (bool, error) {
	switch policy.Kind {
	case aiToolPolicySelfOnly:
		// SelfOnly 只要求当前有合法用户上下文即可。
		return principal.UserID != 0, nil
	case aiToolPolicySuperAdminOnly:
		// 观测类工具只有超级管理员可见。
		return principal.IsSuperAdmin, nil
	case aiToolPolicyOrgCapability:
		// 超级管理员对组织能力类工具直接视为可见。
		if principal.IsSuperAdmin {
			return true, nil
		}
		// 缺少授权服务、用户或组织上下文时，该工具对本轮不可见。
		if r == nil || r.authorization == nil || principal.UserID == 0 || principal.CurrentOrgID == nil || *principal.CurrentOrgID == 0 {
			return false, nil
		}
		// 组织能力类工具按当前组织上下文做一次轻量可见性探测。
		return r.authorization.CheckUserCapabilityInOrg(
			ctx,
			principal.UserID,
			*principal.CurrentOrgID,
			policy.CapabilityCode,
		)
	default:
		// 未识别的策略类型一律按不可见处理。
		return false, nil
	}
}

// findTool 按名称从注册表里查找具体工具实现。
func (r *aiToolRegistry) findTool(name string) *aiServiceTool {
	// 统一 trim 输入，避免调用方传入带空白的工具名。
	normalizedName := strings.TrimSpace(name)
	for _, tool := range r.tools {
		// 只返回名称完全匹配的工具实现。
		if tool != nil && tool.spec.Name == normalizedName {
			return tool
		}
	}
	return nil
}

// buildAIToolDynamicPrompt 生成“本轮工具使用说明”提示词。
//
// 该提示词会喂给模型，用于约束模型的工具使用行为，包括：
//  1. 只能使用本轮明确列出的工具；
//  2. 缺少精确标识时不要猜；
//  3. 工具可见 ≠ 最终一定执行成功，执行期仍会鉴权；
//  4. 当前组织上下文是什么；
//  5. 每个工具的参数定义是什么。
//
// 设计价值：
// - 减少模型臆造工具；
// - 减少模型胡乱猜 org_id / task_id / request_id；
// - 将“后端权限事实”同步给模型，提高调用成功率。
func buildAIToolDynamicPrompt(
	tools []aidomain.Tool,
	principal aidomain.AIToolPrincipal,
) string {
	// builder 按顺序拼出固定约束、组织上下文和可见工具清单。
	var builder strings.Builder
	builder.WriteString("你是 personal_assistant 的 AI 助手。\n")
	builder.WriteString("本轮只能使用下面明确列出的工具；不要假设还有其他工具。\n")
	builder.WriteString("如果用户请求需要 org_id、task_id、execution_id、request_id 等精确标识，而上下文里没有，不要猜测，直接向用户索取。\n")
	builder.WriteString("工具可见性已经按当前授权事实过滤，但真正执行时仍会再次鉴权；如果工具报权限错误，直接向用户说明。\n")
	if principal.CurrentOrgID != nil && *principal.CurrentOrgID > 0 {
		// 当前组织上下文单独写进 prompt，帮助模型优先复用默认 org_id。
		builder.WriteString(fmt.Sprintf("当前组织上下文 org_id=%d。\n", *principal.CurrentOrgID))
	}
	if len(tools) == 0 {
		// 无工具时明确告知模型只能基于上下文回答，避免虚构工具调用。
		builder.WriteString("本轮没有可用工具，请直接基于已有上下文回答，无法确认的数据不要编造。")
		return builder.String()
	}

	// 有工具时逐个列出名称、描述和参数协议。
	builder.WriteString("本轮可用工具清单：\n")
	for idx, tool := range tools {
		spec := tool.Spec()
		builder.WriteString(fmt.Sprintf("%d. %s: %s\n", idx+1, spec.Name, spec.Description))
		if len(spec.Parameters) == 0 {
			builder.WriteString("   参数：无\n")
			continue
		}
		builder.WriteString("   参数：\n")
		for _, param := range spec.Parameters {
			builder.WriteString("   - ")
			builder.WriteString(formatAIToolParameterPrompt(param))
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
}

// formatAIToolParameterPrompt 把单个参数协议转成可读的 prompt 文本。
func formatAIToolParameterPrompt(param aidomain.ToolParameter) string {
	// meta 先拼出类型、必填性和枚举约束。
	meta := string(param.Type)
	if param.Required {
		meta += ", required"
	} else {
		meta += ", optional"
	}
	if len(param.Enum) > 0 {
		meta += ", enum=" + strings.Join(param.Enum, "|")
	}
	if param.Type == aidomain.ToolParameterTypeArray && param.Items != nil {
		// array 参数额外补出元素类型说明。
		meta += ", items=" + describeAIToolParameterType(*param.Items)
	}

	// line 是当前参数的主描述行。
	line := fmt.Sprintf("%s (%s)", param.Name, meta)
	if strings.TrimSpace(param.Description) != "" {
		line += ": " + strings.TrimSpace(param.Description)
	}
	if len(param.Properties) == 0 {
		return line
	}

	// object 参数递归列出所有子字段，方便模型一次性看懂结构。
	children := make([]string, 0, len(param.Properties))
	for _, child := range param.Properties {
		children = append(children, formatAIToolParameterPrompt(child))
	}
	return line + "; fields={" + strings.Join(children, "; ") + "}"
}

// describeAIToolParameterType 返回参数类型的简短可读描述。
func describeAIToolParameterType(param aidomain.ToolParameter) string {
	switch param.Type {
	case aidomain.ToolParameterTypeObject:
		// object 直接返回 object 标记。
		return "object"
	case aidomain.ToolParameterTypeArray:
		// array 继续递归描述元素类型。
		if param.Items == nil {
			return "array"
		}
		return "array<" + describeAIToolParameterType(*param.Items) + ">"
	default:
		// 基础类型直接返回底层 type 值。
		return string(param.Type)
	}
}

// newAISelfOnlyPolicy 创建 SelfOnly 访问策略。
func newAISelfOnlyPolicy() aiToolPolicy {
	// SelfOnly 只依赖当前用户事实，不要求组织能力或超级管理员。
	return aiToolPolicy{Kind: aiToolPolicySelfOnly}
}

// newAIOrgCapabilityPolicy 创建组织能力访问策略。
func newAIOrgCapabilityPolicy(capabilityCode string) aiToolPolicy {
	// capability code 由具体工具声明，执行时再结合参数做真实鉴权。
	return aiToolPolicy{
		Kind:           aiToolPolicyOrgCapability,
		CapabilityCode: capabilityCode,
	}
}

// newAISuperAdminOnlyPolicy 创建超级管理员访问策略。
func newAISuperAdminOnlyPolicy() aiToolPolicy {
	// 观测类工具统一走 super admin 策略。
	return aiToolPolicy{Kind: aiToolPolicySuperAdminOnly}
}

// decodeAIToolArgs 负责把模型传入的 JSON 参数解析到目标结构。
func decodeAIToolArgs(call aidomain.ToolCall, out any) error {
	// 空参数默认按空对象处理，兼容无参工具或模型漏传空对象的情况。
	if strings.TrimSpace(call.ArgumentsJSON) == "" {
		call.ArgumentsJSON = "{}"
	}
	// JSON 解析失败时统一包装成参数错误，方便前端和模型理解。
	if err := json.Unmarshal([]byte(call.ArgumentsJSON), out); err != nil {
		return bizerrors.WrapWithMsg(bizerrors.CodeInvalidParams, "AI tool 参数解析失败", err)
	}
	return nil
}

// buildAIToolResult 负责把工具返回值编码成模型输出和 trace 展示内容。
func buildAIToolResult(payload any, summary string) (aidomain.ToolResult, error) {
	// raw 用于回传给模型，保持紧凑 JSON 结构。
	raw, err := json.Marshal(payload)
	if err != nil {
		return aidomain.ToolResult{}, bizerrors.Wrap(bizerrors.CodeInternalError, err)
	}
	// pretty 用于 trace 明细展示，提升可读性。
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return aidomain.ToolResult{}, bizerrors.Wrap(bizerrors.CodeInternalError, err)
	}

	// 未显式提供 summary 时，从原始 JSON 截断一份短摘要。
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = truncateRunes(string(raw), 120)
	}
	return aidomain.ToolResult{
		Output:         string(raw),
		Summary:        summary,
		DetailMarkdown: "```json\n" + string(pretty) + "\n```",
	}, nil
}

// requireAISuperAdmin 在工具执行阶段强制校验超级管理员权限。
func requireAISuperAdmin(
	ctx context.Context,
	authorization aiAuthorizationService,
	principal aidomain.AIToolPrincipal,
) error {
	// 缺少授权服务时视为无法授权。
	if authorization == nil {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	// 再次实时查询超级管理员状态，避免只依赖 prompt 阶段的可见性判断。
	ok, err := authorization.IsSuperAdmin(ctx, principal.UserID)
	if err != nil {
		return bizerrors.Wrap(bizerrors.CodeInternalError, err)
	}
	if !ok {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	return nil
}

// requireAIOrgCapability 在工具执行阶段强制校验指定组织能力。
func requireAIOrgCapability(
	ctx context.Context,
	authorization aiAuthorizationService,
	principal aidomain.AIToolPrincipal,
	orgID uint,
	capabilityCode string,
) error {
	// 缺少授权服务时无法完成组织能力校验。
	if authorization == nil {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	// 组织能力校验必须落到明确的 org_id 上。
	if orgID == 0 {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "org_id 不能为空")
	}
	// 真实授权交给正式 AuthorizationService 收口。
	return authorization.AuthorizeOrgCapability(ctx, principal.UserID, orgID, capabilityCode)
}

// requireAITaskOrgCapability 依据任务关联组织做执行阶段能力收口。
func requireAITaskOrgCapability(
	ctx context.Context,
	authorization aiAuthorizationService,
	principal aidomain.AIToolPrincipal,
	taskDetail *resp.OJTaskDetailResp,
	capabilityCode string,
) error {
	// 任务详情不存在时直接按任务不存在处理。
	if taskDetail == nil {
		return bizerrors.New(bizerrors.CodeOJTaskNotFound)
	}

	// 从任务详情里提取所有关联组织，供后续统一做能力校验。
	orgIDs := make([]uint, 0, len(taskDetail.Orgs))
	for _, item := range taskDetail.Orgs {
		if item == nil || item.OrgID == 0 {
			continue
		}
		orgIDs = append(orgIDs, item.OrgID)
	}
	if len(orgIDs) == 0 {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	return requireAIOrgCapabilityForMany(ctx, authorization, principal, orgIDs, capabilityCode)
}

// requireAIOrgCapabilityForMany 顺序校验多个组织上的同一项 capability。
func requireAIOrgCapabilityForMany(
	ctx context.Context,
	authorization aiAuthorizationService,
	principal aidomain.AIToolPrincipal,
	orgIDs []uint,
	capabilityCode string,
) error {
	// seen 用于去重，避免同一组织重复触发授权调用。
	seen := make(map[uint]struct{}, len(orgIDs))
	for _, orgID := range orgIDs {
		if orgID == 0 {
			continue
		}
		if _, ok := seen[orgID]; ok {
			continue
		}

		// 每个唯一组织都做一次真实 capability 校验。
		seen[orgID] = struct{}{}
		if err := requireAIOrgCapability(ctx, authorization, principal, orgID, capabilityCode); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return bizerrors.New(bizerrors.CodePermissionDenied)
	}
	return nil
}

// aiMyRankingArgs 表示个人排行工具的输入参数。
type aiMyRankingArgs struct {
	// Platform 表示目标 OJ 平台。
	Platform string `json:"platform"`
	// Scope 表示排行范围，省略时默认 current_org。
	Scope string `json:"scope,omitempty"`
}

// newAIGetMyRankingTool 创建个人排行摘要工具。
func newAIGetMyRankingTool(ojSvc aiOJService) *aiServiceTool {
	// 个人排行工具只面向当前登录用户自己的数据。
	return &aiServiceTool{
		// spec 描述模型可见的工具协议。
		spec: aidomain.ToolSpec{
			Name:        "get_my_ranking",
			Description: "获取当前登录用户在指定 OJ 排行榜中的个人排名摘要，不返回其他用户完整榜单。",
			Parameters: []aidomain.ToolParameter{
				{Name: "platform", Type: aidomain.ToolParameterTypeString, Description: "OJ 平台", Required: true, Enum: []string{"leetcode", "luogu", "lanqiao"}},
				{Name: "scope", Type: aidomain.ToolParameterTypeString, Description: "排行范围，默认 current_org", Enum: []string{"current_org", "all_members"}},
			},
		},
		// policy 声明该工具只围绕当前用户自己的数据执行。
		policy: newAISelfOnlyPolicy(),
		// call 负责查询当前用户在指定平台下的个人排行摘要。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析模型传入的结构化参数。
			var args aiMyRankingArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调 OJService 获取当前用户自己的排行信息。
			rankResp, err := ojSvc.GetRankingList(ctx, callCtx.Principal.UserID, &request.OJRankingListReq{
				Page:     1,
				PageSize: 1,
				Platform: strings.TrimSpace(args.Platform),
				Scope:    strings.TrimSpace(args.Scope),
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}

			// 只返回个人摘要，不把完整榜单数据直接透给模型。
			payload := map[string]any{
				"platform": args.Platform,
				"scope":    defaultString(args.Scope, "current_org"),
				"my_rank":  nil,
				"total":    int64(0),
			}
			if rankResp != nil {
				payload["my_rank"] = rankResp.MyRank
				payload["total"] = rankResp.Total
			}
			return buildAIToolResult(payload, "已返回当前用户的排行摘要")
		},
	}
}

// aiMyPlatformArgs 表示个人平台统计类工具的输入参数。
type aiMyPlatformArgs struct {
	// Platform 表示目标 OJ 平台。
	Platform string `json:"platform"`
}

// newAIGetMyOJStatsTool 创建个人 OJ 统计工具。
func newAIGetMyOJStatsTool(ojSvc aiOJService) *aiServiceTool {
	// 个人统计工具只查询当前用户在单个平台上的统计。
	return &aiServiceTool{
		// spec 描述模型可见的工具名、描述和参数协议。
		spec: aidomain.ToolSpec{
			Name:        "get_my_oj_stats",
			Description: "获取当前登录用户在指定 OJ 平台上的个人统计。",
			Parameters: []aidomain.ToolParameter{
				{Name: "platform", Type: aidomain.ToolParameterTypeString, Description: "OJ 平台", Required: true, Enum: []string{"leetcode", "luogu", "lanqiao"}},
			},
		},
		// policy 仍然是 SelfOnly。
		policy: newAISelfOnlyPolicy(),
		// call 负责查询并返回当前用户的平台统计。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析参数，确保 platform 可用。
			var args aiMyPlatformArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调 OJService 拉取当前用户的平台统计。
			stats, err := ojSvc.GetUserStats(ctx, callCtx.Principal.UserID, &request.OJStatsReq{
				Platform: strings.TrimSpace(args.Platform),
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(stats, "已返回当前用户的 OJ 统计")
		},
	}
}

// newAIGetMyOJCurveTool 创建个人做题曲线工具。
func newAIGetMyOJCurveTool(ojSvc aiOJService) *aiServiceTool {
	// 个人曲线工具只查询当前用户自己的做题趋势。
	return &aiServiceTool{
		// spec 定义模型如何调用该工具。
		spec: aidomain.ToolSpec{
			Name:        "get_my_oj_curve",
			Description: "获取当前登录用户在指定 OJ 平台上的最近做题曲线。",
			Parameters: []aidomain.ToolParameter{
				{Name: "platform", Type: aidomain.ToolParameterTypeString, Description: "OJ 平台", Required: true, Enum: []string{"leetcode", "luogu", "lanqiao"}},
			},
		},
		// policy 声明这是 SelfOnly 工具。
		policy: newAISelfOnlyPolicy(),
		// call 负责查询并返回个人曲线。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析平台参数。
			var args aiMyPlatformArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调 OJService 获取当前用户的平台曲线数据。
			curve, err := ojSvc.GetCurve(ctx, callCtx.Principal.UserID, &request.OJCurveReq{
				Platform: strings.TrimSpace(args.Platform),
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(curve, "已返回当前用户的 OJ 曲线")
		},
	}
}

// aiOrgRankingArgs 表示组织排行摘要工具的输入参数。
type aiOrgRankingArgs struct {
	// OrgID 表示目标组织 ID，省略时默认当前组织。
	OrgID *uint `json:"org_id,omitempty"`
	// Platform 表示目标 OJ 平台。
	Platform string `json:"platform"`
	// Page 表示页码，省略时交给下游服务兜底。
	Page int `json:"page,omitempty"`
	// PageSize 表示分页大小，省略时交给下游服务兜底。
	PageSize int `json:"page_size,omitempty"`
}

// newAIGetOrgRankingSummaryTool 创建组织排行摘要工具。
func newAIGetOrgRankingSummaryTool(
	ojSvc aiOJService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// 组织排行工具要求目标组织具备 OJ 任务管理能力。
	return &aiServiceTool{
		// spec 告诉模型可以按 org_id 和 platform 查询组织排行摘要。
		spec: aidomain.ToolSpec{
			Name:        "get_org_ranking_summary",
			Description: "获取指定组织在指定 OJ 平台排行榜中的摘要，需要 OJ 任务管理能力。",
			Parameters: []aidomain.ToolParameter{
				{Name: "org_id", Type: aidomain.ToolParameterTypeInteger, Description: "目标组织 ID；省略时默认当前组织"},
				{Name: "platform", Type: aidomain.ToolParameterTypeString, Description: "OJ 平台", Required: true, Enum: []string{"leetcode", "luogu", "lanqiao"}},
				{Name: "page", Type: aidomain.ToolParameterTypeInteger, Description: "页码，默认 1"},
				{Name: "page_size", Type: aidomain.ToolParameterTypeInteger, Description: "分页大小，默认 20"},
			},
		},
		// policy 声明该工具受组织 capability 控制。
		policy: newAIOrgCapabilityPolicy(consts.CapabilityCodeOJTaskManage),
		// call 负责解析目标组织、做真实鉴权并查询组织排行摘要。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析模型传入的组织和分页参数。
			var args aiOrgRankingArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// org_id 允许省略，但最终必须解析成一个确定组织。
			orgID, err := resolveAIOrgID(args.OrgID, callCtx.Principal.CurrentOrgID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			// 执行前再次按目标组织做正式 capability 鉴权。
			if err := requireAIOrgCapability(
				ctx,
				authorization,
				callCtx.Principal,
				orgID,
				consts.CapabilityCodeOJTaskManage,
			); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 通过 OJService 查询指定组织的排行摘要。
			out, err := ojSvc.GetRankingList(ctx, callCtx.Principal.UserID, &request.OJRankingListReq{
				Page:     args.Page,
				PageSize: args.PageSize,
				Platform: strings.TrimSpace(args.Platform),
				Scope:    "org",
				OrgID:    &orgID,
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回组织排行榜摘要")
		},
	}
}

// aiTaskExecutionArgs 表示任务执行摘要工具的输入参数。
type aiTaskExecutionArgs struct {
	// TaskID 表示目标任务 ID。
	TaskID uint `json:"task_id"`
	// ExecutionID 表示目标执行 ID。
	ExecutionID uint `json:"execution_id"`
}

// newAIGetTaskExecutionSummaryTool 创建任务执行摘要工具。
func newAIGetTaskExecutionSummaryTool(
	taskSvc aiOJTaskService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// 任务执行摘要工具既要复用任务可见性，也要对关联组织做 capability 收口。
	return &aiServiceTool{
		// spec 描述模型需要提供 task_id 和 execution_id。
		spec: aidomain.ToolSpec{
			Name:        "get_task_execution_summary",
			Description: "获取指定任务执行的摘要，需要先具备任务可见性，再通过关联组织能力校验。",
			Parameters: []aidomain.ToolParameter{
				{Name: "task_id", Type: aidomain.ToolParameterTypeInteger, Description: "任务 ID", Required: true},
				{Name: "execution_id", Type: aidomain.ToolParameterTypeInteger, Description: "执行 ID", Required: true},
			},
		},
		// policy 用于控制该工具是否在当前组织上下文里可见。
		policy: newAIOrgCapabilityPolicy(consts.CapabilityCodeOJTaskManage),
		// call 负责先校验任务可见性，再按任务关联组织做能力收口。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析任务和执行标识。
			var args aiTaskExecutionArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 先复用现有任务详情查询，让 OJTaskService 承担原有可见性校验。
			detail, err := taskSvc.GetTaskDetail(ctx, callCtx.Principal.UserID, args.TaskID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			// 再按任务关联组织做 OJ 任务管理能力收口。
			if err := requireAITaskOrgCapability(
				ctx,
				authorization,
				callCtx.Principal,
				detail,
				consts.CapabilityCodeOJTaskManage,
			); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 能力校验通过后，再查询具体执行摘要。
			out, err := taskSvc.GetTaskExecutionDetail(ctx, callCtx.Principal.UserID, args.TaskID, args.ExecutionID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回任务执行摘要")
		},
	}
}

// aiTaskExecutionUsersArgs 表示任务执行用户列表工具的输入参数。
type aiTaskExecutionUsersArgs struct {
	// TaskID 表示目标任务 ID。
	TaskID uint `json:"task_id"`
	// ExecutionID 表示目标执行 ID。
	ExecutionID uint `json:"execution_id"`
	// Page 表示页码。
	Page int `json:"page,omitempty"`
	// PageSize 表示分页大小。
	PageSize int `json:"page_size,omitempty"`
	// AllCompleted 表示是否只保留全部完成的用户。
	AllCompleted *bool `json:"all_completed,omitempty"`
	// Username 表示用户名关键字过滤条件。
	Username string `json:"username,omitempty"`
}

// newAIListTaskExecutionUsersTool 创建任务执行用户列表工具。
func newAIListTaskExecutionUsersTool(
	taskSvc aiOJTaskService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// 用户列表工具复用任务可见性和组织能力双重收口。
	return &aiServiceTool{
		// spec 描述模型可传的分页和筛选参数。
		spec: aidomain.ToolSpec{
			Name:        "list_task_execution_users",
			Description: "分页列出指定任务执行下的用户结果，需要任务可见性和组织能力。",
			Parameters: []aidomain.ToolParameter{
				{Name: "task_id", Type: aidomain.ToolParameterTypeInteger, Description: "任务 ID", Required: true},
				{Name: "execution_id", Type: aidomain.ToolParameterTypeInteger, Description: "执行 ID", Required: true},
				{Name: "page", Type: aidomain.ToolParameterTypeInteger, Description: "页码，默认 1"},
				{Name: "page_size", Type: aidomain.ToolParameterTypeInteger, Description: "分页大小，默认 20"},
				{Name: "all_completed", Type: aidomain.ToolParameterTypeBoolean, Description: "是否只看已全部完成的用户"},
				{Name: "username", Type: aidomain.ToolParameterTypeString, Description: "用户名关键字"},
			},
		},
		// policy 声明这是组织能力类工具。
		policy: newAIOrgCapabilityPolicy(consts.CapabilityCodeOJTaskManage),
		// call 负责按任务执行分页查询用户结果。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析任务标识和过滤条件。
			var args aiTaskExecutionUsersArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 先拿任务详情，复用现有任务可见性校验。
			detail, err := taskSvc.GetTaskDetail(ctx, callCtx.Principal.UserID, args.TaskID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			// 再按任务关联组织做 capability 收口。
			if err := requireAITaskOrgCapability(
				ctx,
				authorization,
				callCtx.Principal,
				detail,
				consts.CapabilityCodeOJTaskManage,
			); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 能力通过后，按分页和筛选条件查询执行用户列表。
			out, err := taskSvc.GetTaskExecutionUsers(ctx, callCtx.Principal.UserID, args.TaskID, args.ExecutionID, &request.OJTaskExecutionUserListReq{
				Page:         args.Page,
				PageSize:     args.PageSize,
				AllCompleted: args.AllCompleted,
				Username:     strings.TrimSpace(args.Username),
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回任务执行用户列表")
		},
	}
}

// aiTaskExecutionUserDetailArgs 表示任务执行用户详情工具的输入参数。
type aiTaskExecutionUserDetailArgs struct {
	// TaskID 表示目标任务 ID。
	TaskID uint `json:"task_id"`
	// ExecutionID 表示目标执行 ID。
	ExecutionID uint `json:"execution_id"`
	// TargetUserID 表示要查看详情的目标用户 ID。
	TargetUserID uint `json:"target_user_id"`
}

// newAIGetTaskExecutionUserDetailTool 创建任务执行用户详情工具。
func newAIGetTaskExecutionUserDetailTool(
	taskSvc aiOJTaskService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// 用户详情工具与任务摘要工具共享同一套权限收口思路。
	return &aiServiceTool{
		// spec 要求模型给出 task、execution 和 target user 三个关键标识。
		spec: aidomain.ToolSpec{
			Name:        "get_task_execution_user_detail",
			Description: "获取指定任务执行中某个用户的详细结果，需要任务可见性和组织能力。",
			Parameters: []aidomain.ToolParameter{
				{Name: "task_id", Type: aidomain.ToolParameterTypeInteger, Description: "任务 ID", Required: true},
				{Name: "execution_id", Type: aidomain.ToolParameterTypeInteger, Description: "执行 ID", Required: true},
				{Name: "target_user_id", Type: aidomain.ToolParameterTypeInteger, Description: "目标用户 ID", Required: true},
			},
		},
		// policy 仍然是 OJ 任务管理能力。
		policy: newAIOrgCapabilityPolicy(consts.CapabilityCodeOJTaskManage),
		// call 负责查询指定执行里某个用户的详细结果。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析任务、执行和目标用户参数。
			var args aiTaskExecutionUserDetailArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 先用任务详情复用既有任务可见性逻辑。
			detail, err := taskSvc.GetTaskDetail(ctx, callCtx.Principal.UserID, args.TaskID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			// 再按任务关联组织做执行前 capability 收口。
			if err := requireAITaskOrgCapability(
				ctx,
				authorization,
				callCtx.Principal,
				detail,
				consts.CapabilityCodeOJTaskManage,
			); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 授权通过后查询目标用户在该执行中的详细结果。
			out, err := taskSvc.GetTaskExecutionUserDetail(
				ctx,
				callCtx.Principal.UserID,
				args.TaskID,
				args.ExecutionID,
				args.TargetUserID,
			)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回执行用户详情")
		},
	}
}

// aiAnalyzeTaskItemArgs 表示单个题目分析项。
type aiAnalyzeTaskItemArgs struct {
	// Platform 表示题目所属 OJ 平台。
	Platform string `json:"platform"`
	// Title 表示原始题目标题。
	Title string `json:"title"`
}

// aiAnalyzeTaskTitlesArgs 表示题目标题分析工具的输入参数。
type aiAnalyzeTaskTitlesArgs struct {
	// OrgID 表示目标组织 ID，省略时默认当前组织。
	OrgID *uint `json:"org_id,omitempty"`
	// Items 表示待分析的题目列表。
	Items []aiAnalyzeTaskItemArgs `json:"items"`
}

// newAIAnalyzeTaskTitlesTool 创建题目标题分析工具。
func newAIAnalyzeTaskTitlesTool(
	taskSvc aiOJTaskService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// itemParam 描述数组元素的对象结构，让模型知道 items 内每项字段。
	itemParam := aidomain.ToolParameter{
		Type: aidomain.ToolParameterTypeObject,
		Properties: []aidomain.ToolParameter{
			{Name: "platform", Type: aidomain.ToolParameterTypeString, Description: "OJ 平台", Required: true, Enum: []string{"luogu", "leetcode", "lanqiao"}},
			{Name: "title", Type: aidomain.ToolParameterTypeString, Description: "题目标题", Required: true},
		},
	}
	return &aiServiceTool{
		// spec 描述按组织上下文分析一组题目标题的能力。
		spec: aidomain.ToolSpec{
			Name:        "analyze_task_titles",
			Description: "分析一组 OJ 题目标题并返回可解析结果，需要指定组织并具备 OJ 任务管理能力。",
			Parameters: []aidomain.ToolParameter{
				{Name: "org_id", Type: aidomain.ToolParameterTypeInteger, Description: "目标组织 ID；省略时默认当前组织"},
				{Name: "items", Type: aidomain.ToolParameterTypeArray, Description: "待分析题目列表", Required: true, Items: &itemParam},
			},
		},
		// policy 声明该工具需要组织能力。
		policy: newAIOrgCapabilityPolicy(consts.CapabilityCodeOJTaskManage),
		// call 负责解析题目列表、校验组织能力并调用任务分析服务。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析组织和题目数组参数。
			var args aiAnalyzeTaskTitlesArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 组织 ID 允许省略，但最终必须解析成一个确定组织。
			orgID, err := resolveAIOrgID(args.OrgID, callCtx.Principal.CurrentOrgID)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			// 执行前按目标组织做真实能力鉴权。
			if err := requireAIOrgCapability(
				ctx,
				authorization,
				callCtx.Principal,
				orgID,
				consts.CapabilityCodeOJTaskManage,
			); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 把模型输入转成已有 Service 所需的请求 DTO。
			items := make([]request.AnalyzeOJTaskTitleItemReq, 0, len(args.Items))
			for _, item := range args.Items {
				items = append(items, request.AnalyzeOJTaskTitleItemReq{
					Platform: strings.TrimSpace(item.Platform),
					Title:    strings.TrimSpace(item.Title),
				})
			}

			// 调 OJTaskService 复用现有标题分析能力。
			out, err := taskSvc.AnalyzeTaskTitles(ctx, &request.AnalyzeOJTaskTitlesReq{Items: items})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回题目分析结果")
		},
	}
}

// aiTraceDetailArgs 表示 trace 详情工具的输入参数。
type aiTraceDetailArgs struct {
	// RequestID 表示要查询的 request_id。
	RequestID string `json:"request_id"`
	// Limit 表示返回条数上限。
	Limit int `json:"limit,omitempty"`
	// Offset 表示分页偏移量。
	Offset int `json:"offset,omitempty"`
	// IncludePayload 表示是否返回请求响应摘要。
	IncludePayload bool `json:"include_payload,omitempty"`
	// IncludeErrorDetail 表示是否返回错误详情。
	IncludeErrorDetail bool `json:"include_error_detail,omitempty"`
}

// newAIQueryTraceDetailByRequestIDTool 创建 request_id 维度的 trace 详情工具。
func newAIQueryTraceDetailByRequestIDTool(
	obsSvc aiObservabilityService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// trace 详情属于观测类工具，只允许超级管理员使用。
	return &aiServiceTool{
		// spec 描述按 request_id 查询链路详情的能力。
		spec: aidomain.ToolSpec{
			Name:        "query_trace_detail_by_request_id",
			Description: "按 request_id 查询链路详情，仅超级管理员可用。",
			Parameters: []aidomain.ToolParameter{
				{Name: "request_id", Type: aidomain.ToolParameterTypeString, Description: "请求 ID", Required: true},
				{Name: "limit", Type: aidomain.ToolParameterTypeInteger, Description: "返回条数，默认 100"},
				{Name: "offset", Type: aidomain.ToolParameterTypeInteger, Description: "偏移量，默认 0"},
				{Name: "include_payload", Type: aidomain.ToolParameterTypeBoolean, Description: "是否包含请求/响应摘要"},
				{Name: "include_error_detail", Type: aidomain.ToolParameterTypeBoolean, Description: "是否包含错误详情"},
			},
		},
		// policy 声明该工具只对超级管理员可见。
		policy: newAISuperAdminOnlyPolicy(),
		// call 负责执行超级管理员校验并查询 trace 详情。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析 request_id 和分页参数。
			var args aiTraceDetailArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 观测类工具执行前再次强制校验超级管理员权限。
			if err := requireAISuperAdmin(ctx, authorization, callCtx.Principal); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调观测服务按 request_id 拉取链路详情。
			out, err := obsSvc.QueryTraceDetail(
				ctx,
				strings.TrimSpace(args.RequestID),
				request.TraceDetailIDTypeRequest,
				args.Limit,
				args.Offset,
				args.IncludePayload,
				args.IncludeErrorDetail,
			)
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回请求链路详情")
		},
	}
}

// aiTraceSummaryArgs 表示 trace 摘要列表工具的输入参数。
type aiTraceSummaryArgs struct {
	// TraceID 表示 trace_id 过滤条件。
	TraceID string `json:"trace_id,omitempty"`
	// RequestID 表示 request_id 过滤条件。
	RequestID string `json:"request_id,omitempty"`
	// Service 表示服务名过滤条件。
	Service string `json:"service,omitempty"`
	// Status 表示链路状态过滤条件。
	Status string `json:"status,omitempty"`
	// RootStage 表示根阶段过滤条件。
	RootStage string `json:"root_stage,omitempty"`
	// StartAt 表示查询时间窗口开始时间。
	StartAt string `json:"start_at,omitempty"`
	// EndAt 表示查询时间窗口结束时间。
	EndAt string `json:"end_at,omitempty"`
	// Limit 表示返回条数。
	Limit int `json:"limit,omitempty"`
	// Offset 表示分页偏移量。
	Offset int `json:"offset,omitempty"`
}

// newAIQueryTraceSummaryTool 创建 trace 摘要列表工具。
func newAIQueryTraceSummaryTool(
	obsSvc aiObservabilityService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// trace 摘要属于观测类工具，只允许超级管理员使用。
	return &aiServiceTool{
		// spec 描述 trace 列表支持的过滤字段。
		spec: aidomain.ToolSpec{
			Name:        "query_trace_summary",
			Description: "查询链路摘要列表，仅超级管理员可用。",
			Parameters: []aidomain.ToolParameter{
				{Name: "trace_id", Type: aidomain.ToolParameterTypeString, Description: "trace_id"},
				{Name: "request_id", Type: aidomain.ToolParameterTypeString, Description: "request_id"},
				{Name: "service", Type: aidomain.ToolParameterTypeString, Description: "服务名"},
				{Name: "status", Type: aidomain.ToolParameterTypeString, Description: "状态"},
				{Name: "root_stage", Type: aidomain.ToolParameterTypeString, Description: "root stage", Enum: []string{request.TraceRootStageHTTP, request.TraceRootStageTask, request.TraceRootStageAll}},
				{Name: "start_at", Type: aidomain.ToolParameterTypeString, Description: "开始时间"},
				{Name: "end_at", Type: aidomain.ToolParameterTypeString, Description: "结束时间"},
				{Name: "limit", Type: aidomain.ToolParameterTypeInteger, Description: "返回条数"},
				{Name: "offset", Type: aidomain.ToolParameterTypeInteger, Description: "偏移量"},
			},
		},
		// policy 声明该工具只对超级管理员可见。
		policy: newAISuperAdminOnlyPolicy(),
		// call 负责执行超级管理员鉴权并查询 trace 摘要列表。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析各种 trace 过滤条件。
			var args aiTraceSummaryArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 执行前再次验证调用者是否为超级管理员。
			if err := requireAISuperAdmin(ctx, authorization, callCtx.Principal); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调观测服务按过滤条件查询 trace 摘要列表。
			out, err := obsSvc.QueryTrace(ctx, &request.ObservabilityTraceQueryReq{
				TraceID:   strings.TrimSpace(args.TraceID),
				RequestID: strings.TrimSpace(args.RequestID),
				Service:   strings.TrimSpace(args.Service),
				Status:    strings.TrimSpace(args.Status),
				RootStage: strings.TrimSpace(args.RootStage),
				StartAt:   strings.TrimSpace(args.StartAt),
				EndAt:     strings.TrimSpace(args.EndAt),
				Limit:     args.Limit,
				Offset:    args.Offset,
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回链路摘要列表")
		},
	}
}

// aiRuntimeMetricsArgs 表示运行时指标工具的输入参数。
type aiRuntimeMetricsArgs struct {
	// Metric 表示要查询的指标名。
	Metric string `json:"metric"`
	// StartAt 表示时间窗口开始时间。
	StartAt string `json:"start_at,omitempty"`
	// EndAt 表示时间窗口结束时间。
	EndAt string `json:"end_at,omitempty"`
	// Granularity 表示聚合粒度。
	Granularity string `json:"granularity,omitempty"`
	// TaskName 表示任务名过滤条件。
	TaskName string `json:"task_name,omitempty"`
	// Topic 表示 topic 过滤条件。
	Topic string `json:"topic,omitempty"`
	// Status 表示状态过滤条件。
	Status string `json:"status,omitempty"`
	// Limit 表示返回条数。
	Limit int `json:"limit,omitempty"`
}

// newAIQueryRuntimeMetricsTool 创建运行时指标查询工具。
func newAIQueryRuntimeMetricsTool(
	obsSvc aiObservabilityService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// 运行时指标属于观测类工具，只允许超级管理员使用。
	return &aiServiceTool{
		// spec 描述指标查询支持的过滤参数。
		spec: aidomain.ToolSpec{
			Name:        "query_runtime_metrics",
			Description: "查询运行时指标，仅超级管理员可用。",
			Parameters: []aidomain.ToolParameter{
				{Name: "metric", Type: aidomain.ToolParameterTypeString, Description: "指标名称", Required: true},
				{Name: "start_at", Type: aidomain.ToolParameterTypeString, Description: "开始时间"},
				{Name: "end_at", Type: aidomain.ToolParameterTypeString, Description: "结束时间"},
				{Name: "granularity", Type: aidomain.ToolParameterTypeString, Description: "粒度"},
				{Name: "task_name", Type: aidomain.ToolParameterTypeString, Description: "任务名"},
				{Name: "topic", Type: aidomain.ToolParameterTypeString, Description: "topic"},
				{Name: "status", Type: aidomain.ToolParameterTypeString, Description: "状态"},
				{Name: "limit", Type: aidomain.ToolParameterTypeInteger, Description: "返回条数"},
			},
		},
		// policy 声明该工具只对超级管理员可见。
		policy: newAISuperAdminOnlyPolicy(),
		// call 负责鉴权并查询运行时指标。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析指标查询参数。
			var args aiRuntimeMetricsArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 观测类工具执行前统一校验超级管理员权限。
			if err := requireAISuperAdmin(ctx, authorization, callCtx.Principal); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调观测服务查询运行时指标。
			out, err := obsSvc.QueryRuntimeMetrics(ctx, &request.ObservabilityRuntimeMetricQueryReq{
				Metric:      strings.TrimSpace(args.Metric),
				StartAt:     strings.TrimSpace(args.StartAt),
				EndAt:       strings.TrimSpace(args.EndAt),
				Granularity: strings.TrimSpace(args.Granularity),
				TaskName:    strings.TrimSpace(args.TaskName),
				Topic:       strings.TrimSpace(args.Topic),
				Status:      strings.TrimSpace(args.Status),
				Limit:       args.Limit,
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回运行时指标")
		},
	}
}

// aiObservabilityMetricsArgs 表示 HTTP 观测指标工具的输入参数。
type aiObservabilityMetricsArgs struct {
	// Granularity 表示聚合粒度。
	Granularity string `json:"granularity"`
	// StartAt 表示查询时间窗口开始时间。
	StartAt string `json:"start_at"`
	// EndAt 表示查询时间窗口结束时间。
	EndAt string `json:"end_at"`
	// Service 表示服务名过滤条件。
	Service string `json:"service,omitempty"`
	// RouteTemplate 表示路由模板过滤条件。
	RouteTemplate string `json:"route_template,omitempty"`
	// Method 表示 HTTP 方法过滤条件。
	Method string `json:"method,omitempty"`
	// StatusClass 表示状态码段过滤条件。
	StatusClass int `json:"status_class,omitempty"`
	// ErrorCode 表示错误码过滤条件。
	ErrorCode *string `json:"error_code,omitempty"`
	// Limit 表示返回条数。
	Limit int `json:"limit,omitempty"`
}

// newAIQueryObservabilityMetricsTool 创建 HTTP 观测指标查询工具。
func newAIQueryObservabilityMetricsTool(
	obsSvc aiObservabilityService,
	authorization aiAuthorizationService,
) *aiServiceTool {
	// HTTP 观测指标属于观测类工具，只允许超级管理员使用。
	return &aiServiceTool{
		// spec 描述 HTTP 指标查询需要的时间窗口和过滤参数。
		spec: aidomain.ToolSpec{
			Name:        "query_observability_metrics",
			Description: "查询 HTTP 观测指标，仅超级管理员可用。",
			Parameters: []aidomain.ToolParameter{
				{Name: "granularity", Type: aidomain.ToolParameterTypeString, Description: "聚合粒度", Required: true},
				{Name: "start_at", Type: aidomain.ToolParameterTypeString, Description: "开始时间", Required: true},
				{Name: "end_at", Type: aidomain.ToolParameterTypeString, Description: "结束时间", Required: true},
				{Name: "service", Type: aidomain.ToolParameterTypeString, Description: "服务名"},
				{Name: "route_template", Type: aidomain.ToolParameterTypeString, Description: "路由模板"},
				{Name: "method", Type: aidomain.ToolParameterTypeString, Description: "HTTP 方法"},
				{Name: "status_class", Type: aidomain.ToolParameterTypeInteger, Description: "状态码段，例如 2 / 4 / 5"},
				{Name: "error_code", Type: aidomain.ToolParameterTypeString, Description: "错误码"},
				{Name: "limit", Type: aidomain.ToolParameterTypeInteger, Description: "返回条数"},
			},
		},
		// policy 声明该工具只对超级管理员可见。
		policy: newAISuperAdminOnlyPolicy(),
		// call 负责鉴权并查询 HTTP 观测指标。
		call: func(ctx context.Context, call aidomain.ToolCall, callCtx aidomain.ToolCallContext) (aidomain.ToolResult, error) {
			// 先解析时间窗口和过滤参数。
			var args aiObservabilityMetricsArgs
			if err := decodeAIToolArgs(call, &args); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 执行前再次校验超级管理员权限。
			if err := requireAISuperAdmin(ctx, authorization, callCtx.Principal); err != nil {
				return aidomain.ToolResult{}, err
			}

			// 调观测服务查询 HTTP 指标聚合结果。
			out, err := obsSvc.QueryMetrics(ctx, &request.ObservabilityMetricsQueryReq{
				Granularity:   strings.TrimSpace(args.Granularity),
				StartAt:       strings.TrimSpace(args.StartAt),
				EndAt:         strings.TrimSpace(args.EndAt),
				Service:       strings.TrimSpace(args.Service),
				RouteTemplate: strings.TrimSpace(args.RouteTemplate),
				Method:        strings.TrimSpace(args.Method),
				StatusClass:   args.StatusClass,
				ErrorCode:     args.ErrorCode,
				Limit:         args.Limit,
			})
			if err != nil {
				return aidomain.ToolResult{}, err
			}
			return buildAIToolResult(out, "已返回观测指标")
		},
	}
}

// resolveAIOrgID 负责解析工具执行时真正要使用的组织 ID。
func resolveAIOrgID(orgID *uint, currentOrgID *uint) (uint, error) {
	// 参数里显式给了 org_id 时优先使用参数值。
	if orgID != nil && *orgID > 0 {
		return *orgID, nil
	}
	// 否则回退到当前用户上下文里的组织。
	if currentOrgID != nil && *currentOrgID > 0 {
		return *currentOrgID, nil
	}
	// 两者都没有时无法继续执行组织能力类工具。
	return 0, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "缺少可用的 org_id")
}

// defaultString 在空字符串时返回兜底值。
func defaultString(value string, fallback string) string {
	// 先去掉首尾空白，避免把空格当成有效值。
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
