package system

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	resp "personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

type projectorRepoStub struct {
	updateCalls int
	lastMessage *entity.AIMessage
}

var _ interfaces.AIRepository = (*projectorRepoStub)(nil)

func (s *projectorRepoStub) CreateConversation(context.Context, *entity.AIConversation) error { return nil }
func (s *projectorRepoStub) GetConversationByID(context.Context, string) (*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) GetConversationByIDForUpdate(context.Context, string) (*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) ListConversationsByUser(context.Context, uint) ([]*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) UpdateConversation(context.Context, *entity.AIConversation) error { return nil }
func (s *projectorRepoStub) DeleteConversationCascade(context.Context, string) error { return nil }
func (s *projectorRepoStub) CreateMessage(context.Context, *entity.AIMessage) error { return nil }
func (s *projectorRepoStub) UpdateMessage(_ context.Context, message *entity.AIMessage) error {
	s.updateCalls++
	cloned := *message
	s.lastMessage = &cloned
	return nil
}
func (s *projectorRepoStub) ListMessagesByConversation(context.Context, string) ([]*entity.AIMessage, error) {
	return nil, nil
}
func (s *projectorRepoStub) CreateInterrupt(context.Context, *entity.AIInterrupt) error { return nil }
func (s *projectorRepoStub) GetInterruptByID(context.Context, string) (*entity.AIInterrupt, error) {
	return nil, nil
}
func (s *projectorRepoStub) GetInterruptByIDForUpdate(context.Context, string) (*entity.AIInterrupt, error) {
	return nil, nil
}
func (s *projectorRepoStub) ListInterruptsByUserAndStatuses(context.Context, uint, []string) ([]*entity.AIInterrupt, error) {
	return nil, nil
}
func (s *projectorRepoStub) ListInterruptsForRecovery(context.Context, []string, time.Time, int) ([]*entity.AIInterrupt, error) {
	return nil, nil
}
func (s *projectorRepoStub) UpdateInterrupt(context.Context, *entity.AIInterrupt) error { return nil }
func (s *projectorRepoStub) WithTx(any) interfaces.AIRepository                          { return s }

func TestAIMessageProjectorPersistsThinkingTrace(t *testing.T) {
	repo := &projectorRepoStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_1",
		ConversationID: "conv_1",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: "[]",
		UIBlocksJSON:   "[]",
		ScopeJSON:      "{}",
	}
	projector := newAIMessageProjector(repo, message)

	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventThinkingStarted,
		Payload: aidomain.ThinkingStartedPayload{Title: "深度思考"},
	})
	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventThinkingDelta,
		Payload: aidomain.ThinkingDeltaPayload{Delta: "正在拆解问题。"},
	})
	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventThinkingCompleted,
		Payload: aidomain.ThinkingCompletedPayload{Content: "正在拆解问题。\n下一步会组织正式回答。"},
	})

	if err := projector.persistMessage(context.Background()); err != nil {
		t.Fatalf("persistMessage() error = %v", err)
	}

	traceItems := decodeTraceItemsForTest(t, repo.lastMessage.TraceItemsJSON)
	if len(traceItems) != 1 {
		t.Fatalf("trace items len = %d, want 1", len(traceItems))
	}
	if traceItems[0].Key != thinkingTraceKey {
		t.Fatalf("trace key = %q", traceItems[0].Key)
	}
	if traceItems[0].Status != aiMessageStatusSuccess {
		t.Fatalf("trace status = %q", traceItems[0].Status)
	}
	if traceItems[0].Content != "正在拆解问题。\n下一步会组织正式回答。" {
		t.Fatalf("trace content = %q", traceItems[0].Content)
	}
}

func TestAIMessageProjectorMarksThinkingStopped(t *testing.T) {
	repo := &projectorRepoStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_2",
		ConversationID: "conv_2",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: "[]",
	}
	projector := newAIMessageProjector(repo, message)
	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventThinkingDelta,
		Payload: aidomain.ThinkingDeltaPayload{Delta: "正在归纳重点。"},
	})
	projector.setStopped()

	if err := projector.persistMessage(context.Background()); err != nil {
		t.Fatalf("persistMessage() error = %v", err)
	}

	traceItems := decodeTraceItemsForTest(t, repo.lastMessage.TraceItemsJSON)
	if len(traceItems) != 1 {
		t.Fatalf("trace items len = %d, want 1", len(traceItems))
	}
	if traceItems[0].Status != aiMessageStatusStopped {
		t.Fatalf("trace status = %q, want %q", traceItems[0].Status, aiMessageStatusStopped)
	}
}

func decodeTraceItemsForTest(t *testing.T, raw string) []resp.AssistantTraceItem {
	t.Helper()
	items := make([]resp.AssistantTraceItem, 0)
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("unmarshal trace items error = %v", err)
	}
	return items
}
