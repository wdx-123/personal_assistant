package system

import (
	"testing"
	"time"

	"personal_assistant/internal/model/entity"
)

func TestMessageToRespPreservesStoredTraceItemOrder(t *testing.T) {
	message := &entity.AIMessage{
		ID:             "msg_ai_order",
		ConversationID: "conv_order",
		Role:           "assistant",
		Content:        "最终正文",
		Status:         aiMessageStatusSuccess,
		TraceItemsJSON: `[{"key":"tool_call_2","title":"第二步","description":"第二步描述","status":"success"},{"key":"tool_call_10","title":"第十步","description":"第十步描述","status":"failed"}]`,
		ScopeJSON:      "{}",
		CreatedAt:      time.Unix(1713859200, 0),
	}

	item, err := messageToResp(message)
	if err != nil {
		t.Fatalf("messageToResp() error = %v", err)
	}
	if len(item.TraceItems) != 2 {
		t.Fatalf("trace item len = %d, want 2", len(item.TraceItems))
	}
	if item.TraceItems[0].Key != "tool_call_2" {
		t.Fatalf("trace item[0] key = %q", item.TraceItems[0].Key)
	}
	if item.TraceItems[1].Key != "tool_call_10" {
		t.Fatalf("trace item[1] key = %q", item.TraceItems[1].Key)
	}
}
