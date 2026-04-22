package ai

import "testing"

func TestEventNamesAreStable(t *testing.T) {
	if EventAssistantToken != "assistant_token" {
		t.Fatalf("EventAssistantToken = %q", EventAssistantToken)
	}
	if EventMessageCompleted != "message_completed" {
		t.Fatalf("EventMessageCompleted = %q", EventMessageCompleted)
	}
}
