package local

import (
	"context"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

type captureSink struct {
	events []aidomain.Event
}

func (s *captureSink) Emit(_ context.Context, event aidomain.Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *captureSink) Heartbeat(context.Context) error {
	return nil
}

func TestRuntimeStreamEmitsMinimalEvents(t *testing.T) {
	runtime := NewRuntime(0)
	sink := &captureSink{}

	result, err := runtime.Stream(context.Background(), aidomain.StreamInput{Content: "你好"}, sink)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.Content == "" {
		t.Fatal("Stream() result content is empty")
	}
	if len(sink.events) < 4 {
		t.Fatalf("events len = %d, want at least 4", len(sink.events))
	}
	if sink.events[0].Name != aidomain.EventConversationStarted {
		t.Fatalf("first event = %q", sink.events[0].Name)
	}
	if sink.events[len(sink.events)-1].Name != aidomain.EventDone {
		t.Fatalf("last event = %q", sink.events[len(sink.events)-1].Name)
	}
}
