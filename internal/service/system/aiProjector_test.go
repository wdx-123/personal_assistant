package system

import (
	"context"
	"testing"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

type projectorRepoStub struct {
	updateCalls int
	lastMessage *entity.AIMessage
}

var _ interfaces.AIRepository = (*projectorRepoStub)(nil)

func (s *projectorRepoStub) CreateConversation(context.Context, *entity.AIConversation) error {
	return nil
}
func (s *projectorRepoStub) GetConversationByID(context.Context, string) (*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) GetConversationByIDForUpdate(context.Context, string) (*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) ListConversationsByUser(context.Context, uint) ([]*entity.AIConversation, error) {
	return nil, nil
}
func (s *projectorRepoStub) UpdateConversation(context.Context, *entity.AIConversation) error {
	return nil
}
func (s *projectorRepoStub) DeleteConversationCascade(context.Context, string) error { return nil }
func (s *projectorRepoStub) CreateMessage(context.Context, *entity.AIMessage) error  { return nil }
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
func (s *projectorRepoStub) WithTx(any) interfaces.AIRepository                         { return s }

func TestAIMessageProjectorPersistsBasicMessageAndClearsTraceJSON(t *testing.T) {
	repo := &projectorRepoStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_1",
		ConversationID: "conv_1",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: `[{"key":"legacy","title":"legacy"}]`,
		UIBlocksJSON:   `[{"key":"legacy"}]`,
		ScopeJSON:      `{"legacy":true}`,
	}
	projector := newAIMessageProjector(repo, message)

	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventAssistantToken,
		Payload: aidomain.AssistantTokenPayload{Token: "第一段"},
	})
	projector.applyEvent(aidomain.Event{
		Name:    aidomain.EventMessageCompleted,
		Payload: aidomain.MessageCompletedPayload{Content: "第一段第二段"},
	})

	if err := projector.persistMessage(context.Background()); err != nil {
		t.Fatalf("persistMessage() error = %v", err)
	}
	if repo.lastMessage.Content != "第一段第二段" {
		t.Fatalf("content = %q", repo.lastMessage.Content)
	}
	if repo.lastMessage.Status != aiMessageStatusSuccess {
		t.Fatalf("status = %q", repo.lastMessage.Status)
	}
	if repo.lastMessage.TraceItemsJSON != "[]" {
		t.Fatalf("trace json = %q", repo.lastMessage.TraceItemsJSON)
	}
	if repo.lastMessage.UIBlocksJSON != "[]" {
		t.Fatalf("ui blocks json = %q", repo.lastMessage.UIBlocksJSON)
	}
	if repo.lastMessage.ScopeJSON != "{}" {
		t.Fatalf("scope json = %q", repo.lastMessage.ScopeJSON)
	}
}

func TestAIMessageProjectorSetStoppedAndError(t *testing.T) {
	repo := &projectorRepoStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_2",
		ConversationID: "conv_2",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
	}
	projector := newAIMessageProjector(repo, message)

	projector.setStopped()
	if err := projector.persistMessage(context.Background()); err != nil {
		t.Fatalf("persistMessage() error = %v", err)
	}
	if repo.lastMessage.Status != aiMessageStatusStopped {
		t.Fatalf("stopped status = %q", repo.lastMessage.Status)
	}

	projector.setError("模型调用失败")
	if err := projector.persistMessage(context.Background()); err != nil {
		t.Fatalf("persistMessage() error = %v", err)
	}
	if repo.lastMessage.Status != aiMessageStatusError {
		t.Fatalf("error status = %q", repo.lastMessage.Status)
	}
	if repo.lastMessage.ErrorText != "模型调用失败" {
		t.Fatalf("error text = %q", repo.lastMessage.ErrorText)
	}
}
