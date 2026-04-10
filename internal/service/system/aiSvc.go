package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"personal_assistant/global"
	streamsse "personal_assistant/internal/infrastructure/sse"
	"personal_assistant/internal/model/dto/request"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"

	"go.uber.org/zap"
)

const (
	// aiMessageStatusIdle 表示消息已进入等待用户确认或等待后续动作的空闲态。
	aiMessageStatusIdle = "idle"

	// aiMessageStatusLoading 表示消息仍在持续生成中。
	aiMessageStatusLoading = "loading"

	// aiMessageStatusSuccess 表示消息已完整生成成功。
	aiMessageStatusSuccess = "success"

	// aiMessageStatusError 表示消息在生成过程中失败。
	aiMessageStatusError = "error"

	// aiMessageStatusStopped 表示消息因取消、超时或撤销被中断。
	aiMessageStatusStopped = "stopped"

	// aiInterruptStatusAwaiting 表示 interrupt 已创建，等待用户明确决策。
	aiInterruptStatusAwaiting = "awaiting_confirmation"

	// aiInterruptStatusDecision 表示用户决策已提交，运行时可以继续推进。
	aiInterruptStatusDecision = "decision_received"

	// aiInterruptStatusDone 表示需要确认的工具已经执行完成。
	aiInterruptStatusDone = "completed"

	// aiInterruptStatusSkipped 表示用户显式选择跳过该工具。
	aiInterruptStatusSkipped = "skipped"
)

// AIService 负责编排 AI 会话、消息、interrupt 与流式运行时之间的业务流程。
// 它本身不直接操作 HTTP，也不直连数据库；所有持久化都通过 Repository 完成。
type AIService struct {
	txRunner repository.TxRunner
	aiRepo   interfaces.AIRepository
	userRepo interfaces.UserRepository
	runtime  AIRuntime
	policy   streamsse.ConnectionPolicy
}

// NewAIService 负责组装 AIService 所需依赖。
// 参数：
//   - repositoryGroup：仓储聚合对象，提供事务执行器和 AI 相关仓储。
//
// 返回值：
//   - *AIService：已经绑定仓储与本地运行时的服务实例。
//
// 核心流程：
//  1. 优先读取全局 SSE 基础设施中的连接策略。
//  2. 归一化策略后创建本地 AIRuntime。
//  3. 从仓储聚合中提取 AI 会话与用户仓储。
//
// 注意事项：
//   - 这里不直接依赖 HTTP 层，而是只保留运行时策略，方便同一业务逻辑被不同入口复用。
func NewAIService(repositoryGroup *repository.Group) *AIService {
	policy := streamsse.ConnectionPolicy{}
	if global.StreamInfra != nil {
		policy = global.StreamInfra.Policy
	}

	return &AIService{
		txRunner: repositoryGroup,
		aiRepo:   repositoryGroup.SystemRepositorySupplier.GetAIRepository(),
		userRepo: repositoryGroup.SystemRepositorySupplier.GetUserRepository(),
		runtime:  NewLocalAIRuntime(policy.Normalize().HeartbeatInterval),
		policy:   policy.Normalize(),
	}
}

// CreateConversation 负责为当前用户创建一个新的 AI 会话。
// 作用：给当前用户创建一个新的 AI 会话。
func (s *AIService) CreateConversation(
	ctx context.Context,
	userID uint,
	req *request.CreateAssistantConversationReq,
) (*resp.AssistantConversationResp, error) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	now := time.Now()
	conversation := &entity.AIConversation{
		ID:           newAIID("conv"),
		UserID:       userID,
		Title:        "新建会话",
		Preview:      "",
		IsGenerating: false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 标题在这里做长度截断，是为了避免后续持久化层再承担展示字段清洗逻辑。
	if req != nil && strings.TrimSpace(req.Title) != "" {
		conversation.Title = truncateRunes(req.Title, 100)
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if user == nil {
		return nil, bizerrors.New(bizerrors.CodeUserNotFound)
	}

	conversation.OrgID = user.CurrentOrgID
	if err := s.aiRepo.CreateConversation(ctx, conversation); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return conversationToResp(conversation), nil
}

// ListConversations 负责返回当前用户的会话列表。
func (s *AIService) ListConversations(ctx context.Context, userID uint) ([]*resp.AssistantConversationResp, error) {
	conversations, err := s.aiRepo.ListConversationsByUser(ctx, userID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	items := make([]*resp.AssistantConversationResp, 0, len(conversations))
	for _, conversation := range conversations {
		items = append(items, conversationToResp(conversation))
	}
	return items, nil
}

// ListMessages 负责返回指定会话下的消息列表。
func (s *AIService) ListMessages(ctx context.Context, userID uint, conversationID string) ([]*resp.AssistantMessageResp, error) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	conversation, err := s.requireConversationOwner(ctx, userID, conversationID)
	if err != nil {
		return nil, err
	}

	messages, err := s.aiRepo.ListMessagesByConversation(ctx, conversation.ID)
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}

	items := make([]*resp.AssistantMessageResp, 0, len(messages))
	for _, message := range messages {
		item, mapErr := messageToResp(message)
		if mapErr != nil {
			return nil, bizerrors.Wrap(bizerrors.CodeInternalError, mapErr)
		}
		items = append(items, item)
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return items, nil
}

// DeleteConversation 负责删除当前用户拥有的会话。
func (s *AIService) DeleteConversation(ctx context.Context, userID uint, conversationID string) error {
	if _, err := s.requireConversationOwner(ctx, userID, conversationID); err != nil {
		return err
	}
	if err := s.aiRepo.DeleteConversationCascade(ctx, conversationID); err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return nil
}

// StreamConversation 负责启动一次完整的 AI 流式会话生成流程。
// 参数：
//   - ctx：本次流式请求的上下文；取消时会中断运行时和持久化收尾。
//   - userID：当前用户 ID。
//   - conversationID：目标会话 ID。
//   - req：流式消息请求。
//   - writer：SSE 输出器。
// 核心流程：
//  1. 先校验请求参数、会话归属和会话忙碌状态。
//  2. 调用运行时生成 Plan，并准备用户消息、AI 消息和可选 interrupt。
//  3. 事务化落库会话状态与初始消息，确保流开始前数据库状态完整。
//  4. 创建 sink 执行运行时，并在结束后统一做收尾。
func (s *AIService) StreamConversation(
	ctx context.Context,
	userID uint,
	conversationID string,
	req *request.StreamAssistantMessageReq,
	writer streamsse.StreamWriter,
) error {
	// 第一阶段：先挡住明显非法输入，避免后续进入昂贵的数据库与运行时链路。
	if req == nil {
		return bizerrors.New(bizerrors.CodeInvalidParams)
	}
	if strings.TrimSpace(req.ConversationID) != conversationID {
		return bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "conversation_id 与路径参数不一致")
	}
	if writer == nil {
		return bizerrors.New(bizerrors.CodeAIStreamingUnsupported)
	}

	// 第二阶段：读取会话与用户上下文，保证本次流式执行建立在合法归属和可用会话之上。
	conversation, err := s.requireConversationOwner(ctx, userID, conversationID)
	if err != nil {
		return err
	}
	if conversation.IsGenerating {
		return bizerrors.New(bizerrors.CodeAIConversationBusy)
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if user == nil {
		return bizerrors.New(bizerrors.CodeUserNotFound)
	}

	// 先做 Plan，是为了把“需要哪些工具、是否要 interrupt”在真正写流之前确定下来。
	plan, err := s.runtime.Plan(ctx, AIRuntimePlanInput{Conversation: conversation, Request: req})
	if err != nil {
		return bizerrors.WrapWithMsg(bizerrors.CodeAIRequestRejected, "AI 请求无法解析", err)
	}

	// 第三阶段：构造本次对话会产生的持久化对象，确保运行时开始前有可追踪的消息骨架。
	now := time.Now()
	userMessage := &entity.AIMessage{
		ID:             newAIID("msg_user"),
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        strings.TrimSpace(req.Content),
		Status:         aiMessageStatusSuccess,
		TraceItemsJSON: "[]",
		UIBlocksJSON:   "[]",
		ScopeJSON:      "{}",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	assistantMessage := &entity.AIMessage{
		ID:             newAIID("msg_ai"),
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: "[]",
		UIBlocksJSON:   "[]",
		ScopeJSON:      "{}",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 只有声明了需要确认的工具时才创建 interrupt，避免所有请求都落无意义的等待记录。
	var interrupt *entity.AIInterrupt
	if plan.DocTool != nil && plan.DocTool.RequiresConfirmation {
		interrupt = &entity.AIInterrupt{
			InterruptID:      newAIID("intr"),
			ConversationID:   conversation.ID,
			MessageID:        assistantMessage.ID,
			UserID:           userID,
			Status:           aiInterruptStatusAwaiting,
			ToolKey:          plan.DocTool.Key,
			RuntimeStateJSON: encodeJSON(map[string]any{"kind": plan.DocTool.Kind}, "{}"),
			OwnerNodeID:      s.runtime.NodeID(),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	// 第四阶段：事务化写入起始状态，确保“会话进入生成中”与“消息骨架落库”具备一致性。
	if err := s.persistStreamStart(ctx, conversation, user, userMessage, assistantMessage, interrupt, now); err != nil {
		return err
	}

	// Sink 负责把运行时事件同步到 SSE 与数据库消息状态，两条链路共用同一份状态机。
	sink := newAIStreamSink(s.aiRepo, writer, assistantMessage, interrupt)
	execErr := s.runtime.Execute(ctx, AIRuntimeExecutionInput{
		UserID:         userID,
		Conversation:   conversation,
		Request:        req,
		Plan:           plan,
		Interrupt:      interrupt,
		AssistantMsgID: assistantMessage.ID,
	}, sink)

	// 所有已开始的流式请求都统一走 finishStream 收尾，避免成功和失败路径各自写一套状态处理逻辑。
	return s.finishStream(ctx, conversation, sink, execErr)
}

// SubmitDecision 负责接收用户对 interrupt 的决策并推进状态机。
// 作用：接收用户对 interrupt 的决策，并推进状态机。
func (s *AIService) SubmitDecision(
	ctx context.Context,
	userID uint,
	conversationID,
	interruptID string,
	req *request.SubmitAssistantDecisionReq,
) (*resp.AssistantInterruptDecisionAcceptedResp, error) {
	if req == nil {
		return nil, bizerrors.New(bizerrors.CodeInvalidParams)
	}
	if req.ConversationID != conversationID || req.InterruptID != interruptID {
		return nil, bizerrors.NewWithMsg(bizerrors.CodeInvalidParams, "conversation_id 或 interrupt_id 与路径参数不一致")
	}

	// 先确认当前用户确实拥有这次会话，避免借 interrupt ID 越权推进他人流程。
	if _, err := s.requireConversationOwner(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	interrupt, err := s.aiRepo.GetInterruptByID(ctx, interruptID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if interrupt == nil || interrupt.ConversationID != conversationID || interrupt.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeAIInterruptNotFound)
	}
	if interrupt.Status != aiInterruptStatusAwaiting {
		return nil, bizerrors.New(bizerrors.CodeAIInterruptConflict)
	}

	// 运行时如果已经不再等待该 interrupt，会返回 unavailable；这时数据库不能继续盲目推进状态。
	ok, submitErr := s.runtime.SubmitDecision(ctx, AIRuntimeDecisionCommand{
		UserID:         userID,
		ConversationID: conversationID,
		InterruptID:    interruptID,
		Decision:       req.Decision,
		Reason:         req.Reason,
	})
	if submitErr != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeInternalError, submitErr)
	}
	if !ok {
		return nil, bizerrors.New(bizerrors.CodeAIInterruptUnavailable)
	}

	// 决策被运行时接收后，再把数据库状态推进为“已收到决策”，保证线上状态与持久化状态一致。
	interrupt.Decision = req.Decision
	interrupt.Reason = strings.TrimSpace(req.Reason)
	interrupt.Status = aiInterruptStatusDecision
	interrupt.UpdatedAt = time.Now()
	if err := s.aiRepo.UpdateInterrupt(ctx, interrupt); err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	return &resp.AssistantInterruptDecisionAcceptedResp{
		Accepted:       true,
		ConversationID: conversationID,
		InterruptID:    interruptID,
		Decision:       req.Decision,
	}, nil
}

// RevokeUserSessions 负责撤销某个用户当前节点上的运行中会话。
// 参数：
//   - ctx：链路上下文。
//   - userID：目标用户 ID。
//   - reason：撤销原因。
//
// 返回值：
//   - int：实际被撤销的会话数量。
//
// 核心流程：
//  1. 直接委托运行时做用户级别的会话撤销。
//
// 注意事项：
//   - 这里不直接更新数据库，是因为撤销后的消息状态和会话状态要由运行时收尾链路统一落库。
func (s *AIService) RevokeUserSessions(ctx context.Context, userID uint, reason string) int {
	return s.runtime.RevokeUser(ctx, userID, reason)
}

// requireConversationOwner 负责校验当前用户是否拥有指定会话。
// 作用：撤销某个用户当前节点上的运行中会话。
func (s *AIService) requireConversationOwner(ctx context.Context, userID uint, conversationID string) (*entity.AIConversation, error) {
	conversation, err := s.aiRepo.GetConversationByID(ctx, conversationID)
	if err != nil {
		return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
	}
	if conversation == nil || conversation.UserID != userID {
		return nil, bizerrors.New(bizerrors.CodeAIConversationNotFound)
	}
	return conversation, nil
}

// persistStreamStart 负责把流式会话起始状态一次性写入数据库。
// 作用：校验当前用户是不是某个会话的拥有者。
func (s *AIService) persistStreamStart(
	ctx context.Context,
	conversation *entity.AIConversation,
	user *entity.User,
	userMessage *entity.AIMessage,
	assistantMessage *entity.AIMessage,
	interrupt *entity.AIInterrupt,
	now time.Time,
) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	conversation.Title = deriveConversationTitle(conversation.Title, userMessage.Content)
	conversation.Preview = buildConversationPreview(userMessage.Content)
	conversation.IsGenerating = true
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	conversation.LastMessageAt = &now
	conversation.UpdatedAt = now
	conversation.OrgID = user.CurrentOrgID

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return s.txRunner.InTx(ctx, func(tx any) error {
		txAI := s.aiRepo.WithTx(tx)

		// 会话状态先更新，是为了保证后续若消息已落库，列表页也能立即看到“生成中”态。
		if err := txAI.UpdateConversation(ctx, conversation); err != nil {
			return err
		}
		if err := txAI.CreateMessage(ctx, userMessage); err != nil {
			return err
		}
		if err := txAI.CreateMessage(ctx, assistantMessage); err != nil {
			return err
		}

		// interrupt 只在需要确认工具时创建，避免普通问答链路出现无意义等待记录。
		if interrupt != nil {
			if err := txAI.CreateInterrupt(ctx, interrupt); err != nil {
				return err
			}
		}
		return nil
	})
}

// finishStream 负责统一收尾一次流式会话执行。
//
// 核心流程：
//  1. 无论成功失败，都先把会话从“生成中”切回非生成状态。
//  2. 若运行成功则直接结束。
//  3. 若流尚未开始，直接把执行错误返回给上层。
//  4. 若是取消/超时，标记为 stopped；其他错误则写入 error 事件和 done 事件。
//
// 注意事项：
//   - 这里优先保证客户端已经打开的 SSE 流能收到终态，而不是简单把错误向上返回后中断连接。
func (s *AIService) finishStream(
	ctx context.Context,
	conversation *entity.AIConversation,
	sink *aiStreamSink,
	execErr error,
) error {
	now := time.Now()
	conversation.IsGenerating = false
	conversation.LastMessageAt = &now
	conversation.UpdatedAt = now

	// 收尾状态更新失败要记录日志，但如果主流程已经出错，不再让收尾错误覆盖原始执行错误。
	if err := s.aiRepo.UpdateConversation(ctx, conversation); err != nil {
		global.Log.Error("更新 AI 会话收尾状态失败", zap.String("conversation_id", conversation.ID), zap.Error(err))
		if execErr == nil {
			return bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
	}
	if execErr == nil {
		return nil
	}

	// 如果流还没真正开始，把错误直接抛回控制器，让控制器返回标准 JSON 失败响应。
	if !streamWriterStarted(sink.writer) {
		return execErr
	}

	// 取消或超时属于“被中断”而不是“系统故障”，因此只标 stopped，不再额外发错误提示。
	if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
		sink.setStopped()
		_ = sink.persistMessage(ctx)
		return nil
	}

	// 其他错误需要同时写日志、更新消息错误状态，并主动向客户端补发 error/done 终态事件。
	global.Log.Error("AI 流式运行失败", zap.String("conversation_id", conversation.ID), zap.Error(execErr))
	message := "生成失败，请稍后重试。"
	if bizErr := bizerrors.FromError(execErr); bizErr != nil && strings.TrimSpace(bizErr.Message) != "" {
		message = bizErr.Message
	}

	sink.setError(message)
	_ = sink.Emit(ctx, "error", resp.AssistantErrorPayload{Message: message})
	_ = sink.Emit(ctx, "done", map[string]any{})
	return nil
}

// startedWriter 抽象支持 Started 状态探测的流写出器。
// 之所以单独定义接口，是为了避免 AIService 直接依赖具体 HTTP writer 实现。
type startedWriter interface {
	Started() bool
}

// streamWriterStarted 负责判断某个流写出器是否已经真正开始写流。
// 参数：
//   - writer：当前请求使用的流写出器。
//
// 返回值：
//   - bool：true 表示响应头或事件内容已经开始发送。
//
// 核心流程：
//  1. 尝试按 startedWriter 做能力断言。
//  2. 不支持探测时默认认为流已开始，以避免误向客户端回写普通 JSON。
//
// 注意事项：
//   - 默认返回 true 看起来更保守，但它能防止“流式 writer 已写出，控制器却误判还能回 JSON”这种协议层错误。
func streamWriterStarted(writer streamsse.StreamWriter) bool {
	if sw, ok := writer.(startedWriter); ok {
		return sw.Started()
	}
	return true
}
