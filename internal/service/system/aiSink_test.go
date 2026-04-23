package system

import (
	"context"
	"testing"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	streamsse "personal_assistant/internal/infrastructure/sse"
	"personal_assistant/internal/model/entity"
)

type writerStub struct {
	events []streamsse.StreamEvent
}

func (w *writerStub) WriteEvent(_ context.Context, evt *streamsse.StreamEvent) error {
	if evt != nil {
		w.events = append(w.events, *evt)
	}
	return nil
}

func (w *writerStub) WriteHeartbeat(context.Context) error { return nil }
func (w *writerStub) WriteTerminal(ctx context.Context, evt *streamsse.StreamEvent) error {
	return w.WriteEvent(ctx, evt)
}

func TestAIStreamSinkThrottlesPersistButForceFlushesFinalEvents(t *testing.T) {
	repo := &projectorRepoStub{}
	writer := &writerStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_sink",
		ConversationID: "conv_sink",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: "[]",
		ScopeJSON:      "{}",
	}
	sink := newAIStreamSink(repo, writer, message)
	sink.persistInterval = time.Hour

	if err := sink.Emit(context.Background(), aidomain.Event{
		Name:    aidomain.EventAssistantToken,
		Payload: aidomain.AssistantTokenPayload{Token: "第一段"},
	}); err != nil {
		t.Fatalf("first Emit() error = %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls after first token = %d, want 1", repo.updateCalls)
	}

	if err := sink.Emit(context.Background(), aidomain.Event{
		Name:    aidomain.EventAssistantToken,
		Payload: aidomain.AssistantTokenPayload{Token: "第二段"},
	}); err != nil {
		t.Fatalf("second Emit() error = %v", err)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls after throttled token = %d, want 1", repo.updateCalls)
	}

	if err := sink.Emit(context.Background(), aidomain.Event{
		Name:    aidomain.EventMessageCompleted,
		Payload: aidomain.MessageCompletedPayload{Content: "第一段第二段"},
	}); err != nil {
		t.Fatalf("message_completed Emit() error = %v", err)
	}
	if repo.updateCalls != 2 {
		t.Fatalf("update calls after message_completed = %d, want 2", repo.updateCalls)
	}
	if len(writer.events) != 3 {
		t.Fatalf("writer events len = %d, want 3", len(writer.events))
	}
}

func TestAIStreamSinkDoesNotPersistConversationStarted(t *testing.T) {
	repo := &projectorRepoStub{}
	writer := &writerStub{}
	message := &entity.AIMessage{
		ID:             "msg_ai_sink_step",
		ConversationID: "conv_sink_step",
		Role:           "assistant",
		Status:         aiMessageStatusLoading,
		TraceItemsJSON: "[]",
		ScopeJSON:      "{}",
	}
	sink := newAIStreamSink(repo, writer, message)
	sink.persistInterval = time.Hour

	if err := sink.Emit(context.Background(), aidomain.Event{
		Name:    aidomain.EventConversationStarted,
		Payload: aidomain.ConversationStartedPayload{Title: "新建会话"},
	}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	if repo.updateCalls != 0 {
		t.Fatalf("update calls after conversation_started = %d, want 0", repo.updateCalls)
	}
	if len(writer.events) != 1 {
		t.Fatalf("writer events len = %d, want 1", len(writer.events))
	}
}
