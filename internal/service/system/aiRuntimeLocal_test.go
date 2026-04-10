package system

import (
	"context"
	"sync"
	"testing"
	"time"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
)

// runtimeTestSink 是 LocalAIRuntime 测试用的最小 sink 实现。
// 它只记录事件名，不关心 payload 细节，适合验证事件顺序和关键阶段是否发生。
type runtimeTestSink struct {
	mu         sync.Mutex
	eventNames []string
}

// Emit 负责记录测试期间收到的事件名。
// 参数：
//   - _：测试场景下忽略上下文本体，只保留接口签名一致性。
//   - eventName：当前收到的事件名。
//   - _：测试场景下忽略 payload 详情。
//
// 返回值：
//   - error：当前实现始终返回 nil。
//
// 核心流程：
//  1. 在锁内把事件名追加到切片。
//
// 注意事项：
//   - 使用互斥锁是因为 Execute 在 goroutine 中运行，事件记录与断言读取会并发发生。
func (s *runtimeTestSink) Emit(_ context.Context, eventName string, _ any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventNames = append(s.eventNames, eventName)
	return nil
}

// Heartbeat 负责把心跳事件也纳入测试观察范围。
// 参数：
//   - _：测试中未使用的上下文。
//
// 返回值：
//   - error：当前实现始终返回 nil。
//
// 核心流程：
//  1. 在锁内追加一个固定的 heartbeat 标记。
//
// 注意事项：
//   - 把心跳也记下来，可以帮助排查等待决策阶段是否真的持续保活。
func (s *runtimeTestSink) Heartbeat(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventNames = append(s.eventNames, "heartbeat")
	return nil
}

// snapshot 负责复制当前已经记录到的事件列表。
// 参数：无。
// 返回值：
//   - []string：事件名快照副本。
//
// 核心流程：
//  1. 在锁内复制底层切片，避免把内部可变状态直接暴露给断言逻辑。
//
// 注意事项：
//   - 返回副本而不是原切片，是为了避免调用方无意修改测试 sink 的内部状态。
func (s *runtimeTestSink) snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.eventNames))
	copy(out, s.eventNames)
	return out
}

// TestLocalAIRuntimeExecute_ResumeAfterDecision 验证 interrupt 确认后运行时能够继续执行并输出终态事件。
// 参数：
//   - t：Go 测试上下文。
//
// 返回值：无。
// 核心流程：
//  1. 先构造会触发文档工具确认的请求并生成计划。
//  2. 在 goroutine 中启动 Execute，模拟真实等待用户决策的异步流程。
//  3. 轮询观察 waiting_confirmation 事件，确认运行时已经进入等待态。
//  4. 提交 confirm 决策，并断言最终关键事件都出现。
//
// 注意事项：
//   - 这里显式等待 `tool_call_waiting_confirmation` 后再提交决策，是为了避免测试把“运行时尚未进入等待态”的竞态误当成业务失败。
func TestLocalAIRuntimeExecute_ResumeAfterDecision(t *testing.T) {
	runtime := NewLocalAIRuntime(10 * time.Millisecond)
	req := &request.StreamAssistantMessageReq{
		ConversationID:  "conv_1",
		Content:         "帮我整理一版任务汇报并说明助手页面定位。",
		ContextUserName: "李雷",
		ContextOrgName:  "算法训练营",
	}

	// 第一阶段：先生成计划，并确认该输入确实会命中需要确认的文档工具。
	plan, err := runtime.Plan(context.Background(), AIRuntimePlanInput{
		Conversation: &entity.AIConversation{ID: "conv_1", Title: "新建会话"},
		Request:      req,
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.DocTool == nil {
		t.Fatalf("plan.DocTool = nil, want confirmation tool")
	}

	// 第二阶段：异步启动 Execute，模拟真实请求中“运行时等待决策、外部再提交决策”的并发过程。
	sink := &runtimeTestSink{}
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- runtime.Execute(context.Background(), AIRuntimeExecutionInput{
			UserID:       1,
			Conversation: &entity.AIConversation{ID: "conv_1", Title: "新建会话"},
			Request:      req,
			Plan:         plan,
			Interrupt:    &entity.AIInterrupt{InterruptID: "intr_1"},
		}, sink)
	}()

	// 第三阶段：持续观察事件流，直到确认运行时已经进入 waiting_confirmation 状态。
	deadline := time.Now().Add(2 * time.Second)
	for {
		events := sink.snapshot()
		found := false
		for _, eventName := range events {
			if eventName == "tool_call_waiting_confirmation" {
				found = true
				break
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting_confirmation event not observed, got %v", events)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 第四阶段：提交 confirm 决策，并等待 Execute 完整结束。
	ok, err := runtime.SubmitDecision(context.Background(), AIRuntimeDecisionCommand{
		UserID:         1,
		ConversationID: "conv_1",
		InterruptID:    "intr_1",
		Decision:       "confirm",
	})
	if err != nil {
		t.Fatalf("SubmitDecision() error = %v", err)
	}
	if !ok {
		t.Fatalf("SubmitDecision() ok = false, want true")
	}
	if err := <-doneCh; err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// 第五阶段：校验关键阶段事件已经全部出现，证明等待、确认和收尾链路都已打通。
	events := sink.snapshot()
	expected := []string{
		"conversation_started",
		"tool_call_waiting_confirmation",
		"tool_call_confirmation_result",
		"message_completed",
		"done",
	}
	for _, want := range expected {
		found := false
		for _, got := range events {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("event %q not found in %v", want, events)
		}
	}
}
