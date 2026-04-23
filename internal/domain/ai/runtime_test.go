package ai

import "testing"

func TestEventNamesAreStable(t *testing.T) {
	if EventAssistantToken != "assistant_token" {
		t.Fatalf("EventAssistantToken = %q", EventAssistantToken)
	}
	if EventMessageCompleted != "message_completed" {
		t.Fatalf("EventMessageCompleted = %q", EventMessageCompleted)
	}
	if EventToolCallStarted != "tool_call_started" {
		t.Fatalf("EventToolCallStarted = %q", EventToolCallStarted)
	}
	if EventToolCallFinished != "tool_call_finished" {
		t.Fatalf("EventToolCallFinished = %q", EventToolCallFinished)
	}
}
