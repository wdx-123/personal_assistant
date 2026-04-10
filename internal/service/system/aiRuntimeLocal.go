package system

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	resp "personal_assistant/internal/model/dto/response"
)

// aiSessionSignal 表示运行时等待 interrupt 决策期间收到的一次会话信号。
// 它既承载 confirm/skip 决策，也承载外部强制撤销信号。
type aiSessionSignal struct {
	Decision string // 用户决策
	Reason   string
	Revoked  bool // 是不是外部强制撤销
}

// aiSessionRegistry 负责管理“等待用户决策”的 interrupt 会话。
// 它通过 interruptID 和 userID 两套索引，分别解决精准唤醒和用户级批量撤销两个场景。
type aiSessionRegistry struct {
	mu          sync.Mutex
	byInterrupt map[string]chan aiSessionSignal
	byUser      map[uint]map[string]struct{}
}	

/*
	一、注册表相关函数
*/

// newAISessionRegistry 负责创建本地会话注册表。
// 作用：创建一个空的会话注册表
func newAISessionRegistry() *aiSessionRegistry {
	return &aiSessionRegistry{
		byInterrupt: make(map[string]chan aiSessionSignal),
		byUser:      make(map[uint]map[string]struct{}),
	}
}

// Register 负责登记一个等待决策的 interrupt，并返回监听通道与清理函数。
// 作用：登记一个“正在等待用户确认”的 interrupt，并返回监听通道和清理函数。
func (r *aiSessionRegistry) Register(userID uint, interruptID string) (<-chan aiSessionSignal, func()) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan aiSessionSignal, 1)
	r.byInterrupt[interruptID] = ch

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	userSessions := r.byUser[userID]
	if userSessions == nil {
		userSessions = make(map[string]struct{})
		r.byUser[userID] = userSessions
	}
	userSessions[interruptID] = struct{}{}

	cleanup := func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		delete(r.byInterrupt, interruptID)
		if sessions := r.byUser[userID]; sessions != nil {
			delete(sessions, interruptID)
			if len(sessions) == 0 {
				delete(r.byUser, userID)
			}
		}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return ch, cleanup
}

// Submit 负责向某个等待中的 interrupt 投递一条决策信号。
// 作用：给某个等待中的 interrupt 投递决策信号。
func (r *aiSessionRegistry) Submit(interruptID string, signal aiSessionSignal) bool {
	r.mu.Lock()
	ch, ok := r.byInterrupt[interruptID]
	r.mu.Unlock()
	if !ok {
		return false
	}

	select {
	case ch <- signal:
		return true
	default:
		return false
	}
}

// RevokeUser 负责向某个用户当前所有等待中的 interrupt 广播撤销信号。
// 作用：撤销某个用户当前节点上的所有等待会话。
func (r *aiSessionRegistry) RevokeUser(userID uint, reason string) int {
	r.mu.Lock()
	interrupts := make([]string, 0, len(r.byUser[userID]))
	for interruptID := range r.byUser[userID] {
		interrupts = append(interrupts, interruptID)
	}
	r.mu.Unlock()

	count := 0
	for _, interruptID := range interrupts {
		if r.Submit(interruptID, aiSessionSignal{Revoked: true, Reason: reason}) {
			count++
		}
	}
	return count
}

// LocalAIRuntime 是当前仓库用于本地演示和联调的 AI 运行时实现。
// 根据上下文推测，它并不直接接外部 LLM，而是用规则化逻辑模拟计划、工具调用和 interrupt 流程。
type LocalAIRuntime struct {
	nodeID            string
	heartbeatInterval time.Duration
	sessionRegistry   *aiSessionRegistry
}

/*
	二、runtime 本身相关函数
*/

// NewLocalAIRuntime 负责创建本地 AI 运行时实例。
// 作用：创建一个本地 runtime 实例。
func NewLocalAIRuntime(heartbeatInterval time.Duration) *LocalAIRuntime {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "local"
	}
	if heartbeatInterval <= 0 {
		heartbeatInterval = 20 * time.Second
	}
	return &LocalAIRuntime{
		nodeID:            host,
		heartbeatInterval: heartbeatInterval,
		sessionRegistry:   newAISessionRegistry(),
	}
}

// NodeID 返回当前运行时实例所属的节点标识。
// 作用：返回当前 runtime 所属节点 ID。
func (r *LocalAIRuntime) NodeID() string {
	return r.nodeID
}

// Plan 负责基于用户输入生成本次 AI 会话的执行计划。
// 作用：根据用户输入，先生成一份执行计划 AIRuntimePlan。
func (r *LocalAIRuntime) Plan(_ context.Context, input AIRuntimePlanInput) (*AIRuntimePlan, error) {
	if input.Request == nil {
		return nil, errors.New("ai stream request is nil")
	}
	content := strings.TrimSpace(input.Request.Content)
	if content == "" {
		return nil, errors.New("ai stream content is empty")
	}

	// 第一阶段：先确定会话标题，避免后续轻量和复杂场景各自重复推导标题。
	title := deriveConversationTitle("", content)
	if input.Conversation != nil {
		title = deriveConversationTitle(input.Conversation.Title, content)
	}

	// 对非常轻量的输入直接生成固定回复，减少不必要的工具规划和 interrupt 开销。
	if isLightweightAIPrompt(content) {
		reply := buildLightweightReply(content)
		return &AIRuntimePlan{
			Title:             title,
			Preview:           buildConversationPreview(content),
			Lightweight:       true,
			LightweightReply:  reply,
			FinalReplyConfirm: reply,
			FinalReplySkip:    reply,
		}, nil
	}

	// 第二阶段：识别意图，并据此决定是否展示思考摘要、scope 和各种工具卡片。
	intent := detectAIIntent(content)
	plan := &AIRuntimePlan{
		Title:               title,
		Preview:             buildConversationPreview(content),
		ShowThinkingSummary: intent.showThinkingSummary,
		FinalReplyConfirm:   buildScenarioFinalReply(content, intent, true),
		FinalReplySkip:      buildScenarioFinalReply(content, intent, false),
	}
	if intent.showScope {
		plan.Scope = buildScopeInfo(input.Request)
	}

	// 任务快照和训练进度属于无需用户确认的只读工具，可直接加入计划。
	if intent.wantsTaskReport {
		plan.TaskTool = &AIToolBlueprint{
			Kind:           "task",
			Key:            "tool_task_snapshot",
			Title:          "读取任务执行快照",
			Description:    "读取完成率、成员覆盖情况和当前阻塞项。",
			DurationMS:     148,
			Content:        "已拿到最新任务快照：完成率 81%，覆盖成员 17 / 21，阻塞项 3 个。",
			DetailMarkdown: "任务快照结果：\n\n- 完成率：81%\n- 覆盖成员：17 / 21\n- 阻塞项：3",
		}
	}
	if intent.wantsProgressInsight {
		plan.ProgressTool = &AIToolBlueprint{
			Kind:           "progress",
			Key:            "tool_progress_snapshot",
			Title:          "读取最近训练进度",
			Description:    "汇总最近 7 天训练节奏、排名变化与建议方向。",
			DurationMS:     126,
			Content:        "近 7 天新增 12 题，排名提升 2 位，建议继续集中中等题突破。",
			DetailMarkdown: "训练进度结果：\n\n- 新增题目：12\n- 排名变化：+2\n- 建议方向：中等题突破",
		}
	}

	// 文档工具需要显式确认，是因为根据上下文推测，读取正式文档属于比纯业务数据更重的外部信息补充动作。
	if intent.wantsDocSupport {
		plan.DocTool = &AIToolBlueprint{
			Kind:                    "doc",
			Key:                     "tool_doc_snapshot",
			Title:                   "读取正式项目文档",
			Description:             "读取 README、架构设计方案和 AI UI 改造说明。",
			DurationMS:              182,
			Content:                 "已补充正式项目文档摘要，可据此确认页面定位和后端接入方式。",
			DetailMarkdown:          "文档工具结果：\n\n- 助手继续挂在控制台 Workbench 中。\n- 后端继续采用单条 SSE 聊天流 + interrupt decision。",
			RequiresConfirmation:    true,
			ConfirmationTitle:       "是否继续使用“项目文档摘要”工具？",
			ConfirmationDescription: "继续后会读取正式文档白名单并补充引用摘要；跳过则只基于已有业务数据继续输出。",
		}
	}
	return plan, nil
}

// Execute 负责按计划驱动本地 AI 运行时输出完整的流式事件序列。
// 作用：按 plan 真正执行，并持续往 sink 发事件。
func (r *LocalAIRuntime) Execute(ctx context.Context, input AIRuntimeExecutionInput, sink AIRuntimeSink) error {
	if input.Plan == nil {
		return errors.New("ai runtime plan is nil")
	}
	if sink == nil {
		return errors.New("ai runtime sink is nil")
	}
	plan := input.Plan

	// 第一阶段：输出基础起始事件和无需等待用户确认的结构化块。
	if err := sink.Emit(ctx, "conversation_started", resp.AssistantConversationStartedPayload{Title: plan.Title}); err != nil {
		return err
	}
	if plan.ShowThinkingSummary {
		if err := sink.Emit(ctx, "structured_block", resp.AssistantStructuredBlockPayload{UIBlock: buildThinkingSummaryBlock(plan)}); err != nil {
			return err
		}
	}
	if plan.TaskTool != nil {
		if err := r.runTool(ctx, sink, plan.TaskTool); err != nil {
			return err
		}
	}
	if plan.ProgressTool != nil {
		if err := r.runTool(ctx, sink, plan.ProgressTool); err != nil {
			return err
		}
	}
	if plan.Scope != nil {
		if err := sink.Emit(ctx, "structured_block", resp.AssistantStructuredBlockPayload{Scope: plan.Scope}); err != nil {
			return err
		}
	}

	finalReply := plan.FinalReplyConfirm
	if plan.DocTool != nil && input.Interrupt != nil {
		// 第二阶段：先把“准备调用工具”和“等待用户确认”的 UI 信息推给前端。
		if err := sink.Emit(ctx, "structured_block", resp.AssistantStructuredBlockPayload{UIBlock: buildToolIntentBlock(plan.DocTool)}); err != nil {
			return err
		}
		if err := sink.Emit(ctx, "structured_block", resp.AssistantStructuredBlockPayload{UIBlock: buildWaitingUserBlock(plan.DocTool)}); err != nil {
			return err
		}

		// 注册等待通道后再发 waiting 事件，避免用户极快提交决策时运行时还没建立监听。
		waitCh, cleanup := r.sessionRegistry.Register(input.UserID, input.Interrupt.InterruptID)
		defer cleanup()
		if err := sink.Emit(ctx, "tool_call_waiting_confirmation", resp.AssistantToolCallWaitingConfirmationPayload{
			InterruptID:             input.Interrupt.InterruptID,
			Key:                     plan.DocTool.Key,
			Title:                   plan.DocTool.Title,
			Description:             plan.DocTool.Description,
			DetailMarkdown:          plan.DocTool.DetailMarkdown,
			ConfirmationTitle:       plan.DocTool.ConfirmationTitle,
			ConfirmationDescription: plan.DocTool.ConfirmationDescription,
			Actions:                 assistantTraceActions(plan.DocTool.Key),
		}); err != nil {
			return err
		}

		signal, err := r.waitDecision(ctx, waitCh, sink)
		if err != nil {
			return err
		}
		if signal.Revoked {
			return context.Canceled
		}

		// 第三阶段：根据用户决策更新 trace 和最终回答分支。
		if signal.Decision == "skip" {
			finalReply = plan.FinalReplySkip
			if err := sink.Emit(ctx, "tool_call_confirmation_result", resp.AssistantToolCallConfirmationResultPayload{
				InterruptID: input.Interrupt.InterruptID,
				Key:         plan.DocTool.Key,
				Decision:    "skip",
				Status:      "skipped",
				Description: "已跳过该工具，接下来将只基于当前已有上下文继续输出。",
			}); err != nil {
				return err
			}
		} else {
			if err := sink.Emit(ctx, "tool_call_confirmation_result", resp.AssistantToolCallConfirmationResultPayload{
				InterruptID: input.Interrupt.InterruptID,
				Key:         plan.DocTool.Key,
				Decision:    "confirm",
				Status:      "pending",
				Description: "已确认继续读取正式项目文档，准备补充最终回答。",
			}); err != nil {
				return err
			}
			if err := sink.Emit(ctx, "tool_call_finished", resp.AssistantToolCallFinishedPayload{
				Key:            plan.DocTool.Key,
				Description:    "已完成正式项目文档摘要读取。",
				DurationMS:     plan.DocTool.DurationMS,
				Status:         "success",
				Content:        plan.DocTool.Content,
				DetailMarkdown: plan.DocTool.DetailMarkdown,
			}); err != nil {
				return err
			}
		}
	}

	// 第四阶段：把最终回复切成 token 块持续输出，模拟真实流式体验。
	for _, chunk := range splitReplyChunks(finalReply, 48) {
		if err := sink.Emit(ctx, "assistant_token", resp.AssistantTokenPayload{Token: chunk}); err != nil {
			return err
		}
	}
	if err := sink.Emit(ctx, "message_completed", resp.AssistantMessageCompletedPayload{Content: finalReply}); err != nil {
		return err
	}
	return sink.Emit(ctx, "done", map[string]any{})
}

/*
	五、等待/决策相关函数
*/

// SubmitDecision 负责向本地会话注册表提交用户决策。
// 作用：向本地 runtime 提交一次用户决策。
func (r *LocalAIRuntime) SubmitDecision(_ context.Context, cmd AIRuntimeDecisionCommand) (bool, error) {
	return r.sessionRegistry.Submit(cmd.InterruptID, aiSessionSignal{
		Decision: cmd.Decision,
		Reason:   cmd.Reason,
	}), nil
}

// RevokeUser 负责撤销某个用户当前节点上的等待会话。
// 作用：撤销某个用户当前节点上的所有等待会话。
func (r *LocalAIRuntime) RevokeUser(_ context.Context, userID uint, reason string) int {
	return r.sessionRegistry.RevokeUser(userID, reason)
}

// waitDecision 负责在等待用户决策期间维持心跳并监听最终信号。
// 作用：在等待用户决策期间，一边等信号，一边持续发心跳。
func (r *LocalAIRuntime) waitDecision(ctx context.Context, waitCh <-chan aiSessionSignal, sink AIRuntimeSink) (aiSessionSignal, error) {
	ticker := time.NewTicker(r.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return aiSessionSignal{}, ctx.Err()
		case signal := <-waitCh:
			return signal, nil
		case <-ticker.C:
			if err := sink.Heartbeat(ctx); err != nil {
				return aiSessionSignal{}, err
			}
		}
	}
}

// runTool 负责输出一个无需等待确认的工具执行事件。
// 作用：执行一个“不需要用户确认”的工具。
func (r *LocalAIRuntime) runTool(ctx context.Context, sink AIRuntimeSink, tool *AIToolBlueprint) error {
	if err := sink.Emit(ctx, "tool_call_started", resp.AssistantToolCallStartedPayload{
		Key:         tool.Key,
		Title:       tool.Title,
		Description: tool.Description,
	}); err != nil {
		return err
	}
	return sink.Emit(ctx, "tool_call_finished", resp.AssistantToolCallFinishedPayload{
		Key:            tool.Key,
		Description:    tool.Description,
		DurationMS:     tool.DurationMS,
		Status:         "success",
		Content:        tool.Content,
		DetailMarkdown: tool.DetailMarkdown,
	})
}

// buildLightweightReply 负责为轻量短消息生成直接回复。
// 参数：
//   - input：用户原始输入文本。
//
// 返回值：
//   - string：适合直接返回的轻量回复。
//
// 核心流程：
//  1. 标准化输入文本。
//  2. 根据感谢、确认等简单意图返回固定短回复。
//
// 注意事项：
//   - 轻量回复场景故意不走工具和复杂计划，是为了压缩交互延迟。
func buildLightweightReply(input string) string {
	normalized := strings.TrimSpace(input)
	switch {
	case strings.Contains(strings.ToLower(normalized), "thank"), strings.Contains(normalized, "谢谢"):
		return "不客气。你可以继续让我总结任务、分析进度，或者解释项目文档。"
	case normalized == "好" || normalized == "好的" || strings.EqualFold(normalized, "ok") || strings.EqualFold(normalized, "okay"):
		return "我在。你可以继续补充任务范围、文档范围，或者直接提出下一个问题。"
	default:
		return "你好。我可以继续帮你整理任务进展、分析个人进度，或者解释项目文档。"
	}
}

// buildScenarioFinalReply 负责根据识别出的意图拼装最终答复模板。
// 参数：
//   - input：用户原始输入。
//   - intent：意图识别结果。
//   - confirmed：是否确认执行了文档工具。
//
// 返回值：
//   - string：最终答复文本。
//
// 核心流程：
//  1. 依次根据任务、进度、文档等意图追加段落。
//  2. 若没有任何结构化场景命中，则回退到直接回复。
//
// 注意事项：
//   - confirm/skip 两条路径在这里统一生成不同文本，能避免 Execute 阶段散落字符串拼接逻辑。
func buildScenarioFinalReply(input string, intent aiIntentProfile, confirmed bool) string {
	sections := make([]string, 0, 4)
	if intent.wantsTaskReport {
		sections = append(sections, "当前任务主线已进入收口阶段，主要阻塞集中在少数成员补齐和回归验证。\n\n- 任务完成率：81%\n- 覆盖成员：17 / 21\n- 阻塞项：3\n\n建议先补齐未完成成员，再做一轮回归验证。")
	}
	if intent.wantsProgressInsight {
		sections = append(sections, "近 7 天训练节奏稳定，新增 12 题，排名提升 2 位。当前更值得投入的是中等题突破。\n\n- 当前排名：#5\n- 近 7 天新增：12 题\n- 重点方向：中等题\n\n建议未来三天集中完成 2 到 3 道中等题，并补齐错因总结。")
	}
	if intent.wantsDocSupport && confirmed {
		sections = append(sections, "补充正式项目文档后，可以确认两点：\n\n- 助手继续挂在控制台 Workbench 中，不额外拆独立站点。\n- 后端接入继续采用单条 SSE 聊天流 + interrupt decision 控制接口。")
	}
	if intent.wantsDocSupport && !confirmed {
		sections = append(sections, "本轮没有读取正式项目文档，因此页面定位和接入方式说明未纳入结果。")
	}
	if len(sections) == 0 {
		sections = append(sections, buildDirectReply(input))
	}
	return strings.Join(sections, "\n\n")
}

// buildDirectReply 负责为未命中结构化意图的输入生成直接回复。
// 参数：
//   - input：用户原始输入。
//
// 返回值：
//   - string：直接回复文本。
//
// 核心流程：
//  1. 先判断是否偏向文档解释需求。
//  2. 否则回退到通用引导回复。
//
// 注意事项：
//   - 这里保留文档提示，是为了在未显式命中文档工具时仍给用户一个明确的下一步指引。
func buildDirectReply(input string) string {
	if aiDocPromptRE.MatchString(strings.TrimSpace(input)) {
		return "如果你要我解释项目文档，直接告诉我要看哪份文档，或者说明你想确认页面定位、协议还是后端接入。"
	}
	return "这条问题目前不需要额外工具。我可以直接回答；如果你要任务、进度或文档结论，请把范围再说具体一点。"
}
