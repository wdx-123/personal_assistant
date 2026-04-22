package ai

import "testing"

func TestEventNamesAreStable(t *testing.T) {
	if EventThinkingStarted != "thinking_started" {
		t.Fatalf("EventThinkingStarted = %q", EventThinkingStarted)
	}
	if EventThinkingDelta != "thinking_delta" {
		t.Fatalf("EventThinkingDelta = %q", EventThinkingDelta)
	}
	if EventThinkingCompleted != "thinking_completed" {
		t.Fatalf("EventThinkingCompleted = %q", EventThinkingCompleted)
	}
	if EventAssistantToken != "assistant_token" {
		t.Fatalf("EventAssistantToken = %q", EventAssistantToken)
	}
	if EventMessageCompleted != "message_completed" {
		t.Fatalf("EventMessageCompleted = %q", EventMessageCompleted)
	}
}
