package system

import (
	"strings"

	"personal_assistant/global"
	streamsse "personal_assistant/internal/infrastructure/sse"
	"personal_assistant/internal/model/dto/request"
	serviceContract "personal_assistant/internal/service/contract"
	bizerrors "personal_assistant/pkg/errors"
	"personal_assistant/pkg/jwt"
	"personal_assistant/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AICtrl 封装 AI 会话相关 HTTP 入口。
// Controller 只负责参数接收、调用 Service 和统一响应，不在这里落业务编排。
type AICtrl struct {
	aiService serviceContract.AIServiceContract
}

// CreateConversation 负责创建新的 AI 会话。
// 参数：
//   - c：Gin 请求上下文，承载请求体、响应写出器和鉴权结果。
//
// 返回值：无。
// 核心流程：
//  1. 绑定请求体，尽早拦截格式错误，避免无意义进入 Service。
//  2. 从 JWT 中提取当前用户 ID，并调用 Service 完成会话创建。
//  3. 统一把成功结果包装成标准响应。
//
// 注意事项：
//   - 参数绑定失败时直接返回统一错误响应，是为了把输入错误和业务错误明确区分开。
func (ctrl *AICtrl) CreateConversation(c *gin.Context) {
	var req request.CreateAssistantConversationReq

	// 先做请求体绑定，避免后续业务层再处理结构不完整的输入。
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("AI 创建会话参数绑定失败", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
		return
	}

	// Controller 只负责透传用户身份和请求参数，真正的创建逻辑由 Service 决定。
	data, err := ctrl.aiService.CreateConversation(c.Request.Context(), jwt.GetUserID(c), &req)
	if err != nil {
		global.Log.Error("AI 创建会话失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithDetailed(data, "操作成功", c)
}

// ListConversations 负责返回当前用户的会话列表。
// 参数：
//   - c：Gin 请求上下文。
//
// 返回值：无。
// 核心流程：
//  1. 从鉴权上下文读取用户 ID。
//  2. 调用 Service 查询该用户可见的会话集合。
//  3. 统一输出标准成功或失败响应。
//
// 注意事项：
//   - 列表查询不在 Controller 层做任何过滤拼装，避免和 Service 的权限边界混淆。
func (ctrl *AICtrl) ListConversations(c *gin.Context) {
	data, err := ctrl.aiService.ListConversations(c.Request.Context(), jwt.GetUserID(c))
	if err != nil {
		global.Log.Error("AI 获取会话列表失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithDetailed(data, "获取成功", c)
}

// ListMessages 负责返回指定会话下的消息列表。
// 参数：
//   - c：Gin 请求上下文。
//
// 返回值：无。
// 核心流程：
//  1. 读取路径参数中的会话 ID。
//  2. 调用 Service 校验归属并查询消息列表。
//  3. 把 DTO 列表通过统一响应返回。
//
// 注意事项：
//   - 会话归属校验放在 Service 层，Controller 不重复实现权限判断，保持分层清晰。
func (ctrl *AICtrl) ListMessages(c *gin.Context) {
	data, err := ctrl.aiService.ListMessages(c.Request.Context(), jwt.GetUserID(c), c.Param("id"))
	if err != nil {
		global.Log.Error("AI 获取消息列表失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithDetailed(data, "获取成功", c)
}

// DeleteConversation 负责删除指定会话。
// 参数：
//   - c：Gin 请求上下文。
//
// 返回值：无。
// 核心流程：
//  1. 读取当前用户与目标会话 ID。
//  2. 委托 Service 完成归属校验和级联删除。
//  3. 删除成功后返回统一确认消息。
//
// 注意事项：
//   - 删除操作不在 Controller 层预读数据库，是为了避免重复查询和边界穿透。
func (ctrl *AICtrl) DeleteConversation(c *gin.Context) {
	if err := ctrl.aiService.DeleteConversation(c.Request.Context(), jwt.GetUserID(c), c.Param("id")); err != nil {
		global.Log.Error("AI 删除会话失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}
	response.BizOkWithMessage("删除成功", c)
}

// StreamConversation 负责发起 AI 流式对话输出。
// 参数：
//   - c：Gin 请求上下文，同时也承载 HTTP 流式响应写出能力。
//
// 返回值：无。
// 核心流程：
//  1. 先拒绝 query token，防止认证信息暴露在 URL。
//  2. 绑定流式请求参数，并创建 HTTP SSE writer。
//  3. 调用 Service 执行完整的流式会话流程。
//  4. 如果流尚未真正开始，再退回普通 JSON 错误响应。
//
// 注意事项：
//   - 一旦 SSE 已经写出响应头，就不能再回写普通 JSON；因此这里必须通过 writer.Started() 做分支判断。
func (ctrl *AICtrl) StreamConversation(c *gin.Context) {
	// SSE 明确禁止 query token，是为了避免令牌进入访问日志和浏览器历史。
	if strings.TrimSpace(c.Query("token")) != "" {
		response.BizFailWithCodeMsg(bizerrors.CodeInvalidParams, "SSE 不接受 query token", c)
		return
	}

	var req request.StreamAssistantMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("AI SSE 参数绑定失败", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
		return
	}

	// Writer 在 Controller 层创建，是因为它直接依赖 HTTP ResponseWriter。
	writer := streamsse.NewHTTPStreamWriter(c.Writer, resolveSSEPolicy())
	err := ctrl.aiService.StreamConversation(c.Request.Context(), jwt.GetUserID(c), c.Param("id"), &req, writer)
	if err == nil {
		return
	}

	// 无论流是否已经开始，都先记录日志，方便排查长链路问题。
	global.Log.Error("AI SSE 流执行失败", zap.Error(err))

	// 只有在还没开始写流时，才能安全退回标准错误响应。
	if !writer.Started() {
		response.BizFailWithError(err, c)
	}
}

// SubmitDecision 负责提交 interrupt 决策结果。
// 参数：
//   - c：Gin 请求上下文。
//
// 返回值：无。
// 核心流程：
//  1. 绑定 confirm/skip 等决策参数。
//  2. 调用 Service 校验 interrupt 状态并写入用户决策。
//  3. 返回“已接受”的标准响应。
//
// 注意事项：
//   - Controller 只负责把决策原样转交 Service，不在这里解释 interrupt 状态机。
func (ctrl *AICtrl) SubmitDecision(c *gin.Context) {
	var req request.SubmitAssistantDecisionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		global.Log.Error("AI interrupt 决策参数绑定失败", zap.Error(err))
		response.BizFailWithCodeMsg(bizerrors.CodeBindFailed, "参数绑定失败", c)
		return
	}

	data, err := ctrl.aiService.SubmitDecision(c.Request.Context(), jwt.GetUserID(c), c.Param("id"), c.Param("interrupt_id"), &req)
	if err != nil {
		global.Log.Error("AI interrupt 决策失败", zap.Error(err))
		response.BizFailWithError(err, c)
		return
	}

	response.BizOkWithDetailed(data, "操作成功", c)
}

// resolveSSEPolicy 负责为当前请求解析可用的 SSE 连接策略。
// 参数：无。
// 返回值：
//   - streamsse.ConnectionPolicy：当前节点应使用的 SSE 策略副本。
//
// 核心流程：
//  1. 优先读取全局初始化阶段已经构建好的 SSE 基础设施配置。
//  2. 若基础设施未启用，则回退到一套可运行的默认策略。
//
// 注意事项：
//   - 这里返回标准化后的默认值，而不是空结构体，是为了保证未启用全局基础设施时本地流式能力仍然可用。
func resolveSSEPolicy() streamsse.ConnectionPolicy {
	if global.StreamInfra != nil {
		return global.StreamInfra.Policy
	}
	return streamsse.ConnectionPolicy{}.Normalize()
}
