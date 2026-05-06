package memory

import (
	"context"
	"strings"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestRuleExtractorExtractsExplicitSelfFact(t *testing.T) {
	extractor := NewRuleExtractor(Options{})

	result, err := extractor.Extract(context.Background(), aidomain.MemoryExtractionInput{
		ConversationID: "conv-1",
		UserID:         7,
		UserMessage: aidomain.Message{
			ID:      "msg-user-1",
			Role:    aidomain.RoleUser,
			Content: "请记住以后请用更简洁的方式回答我。",
		},
		AssistantMessage: aidomain.Message{
			ID:      "msg-ai-1",
			Role:    aidomain.RoleAssistant,
			Content: "好的，我会尽量简洁。",
		},
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(result.Facts) != 1 {
		t.Fatalf("facts len = %d, want 1", len(result.Facts))
	}
	fact := result.Facts[0]
	if fact.ScopeType != aidomain.MemoryScopeSelf {
		t.Fatalf("fact scope = %q, want self", fact.ScopeType)
	}
	if fact.Namespace != aidomain.MemoryNamespaceUserPreference {
		t.Fatalf("namespace = %q, want %q", fact.Namespace, aidomain.MemoryNamespaceUserPreference)
	}
	if fact.FactKey != "answer_preference" {
		t.Fatalf("fact_key = %q, want answer_preference", fact.FactKey)
	}
	if !strings.Contains(fact.FactValueJSON, "更简洁") {
		t.Fatalf("fact value = %s, want captured preference", fact.FactValueJSON)
	}
}

func TestRuleExtractorSkipsCasualChatDocument(t *testing.T) {
	extractor := NewRuleExtractor(Options{})

	result, err := extractor.Extract(context.Background(), aidomain.MemoryExtractionInput{
		ConversationID: "conv-2",
		UserID:         8,
		UserMessage: aidomain.Message{
			ID:      "msg-user-2",
			Role:    aidomain.RoleUser,
			Content: "你好",
		},
		AssistantMessage: aidomain.Message{
			ID:      "msg-ai-2",
			Role:    aidomain.RoleAssistant,
			Content: "你好，有什么可以帮你？",
		},
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(result.Facts) != 0 {
		t.Fatalf("facts len = %d, want 0", len(result.Facts))
	}
	if len(result.Documents) != 0 {
		t.Fatalf("documents len = %d, want 0", len(result.Documents))
	}
}

func TestRuleExtractorBuildsKnowledgeDocument(t *testing.T) {
	extractor := NewRuleExtractor(Options{DocumentMinRunes: 40})

	result, err := extractor.Extract(context.Background(), aidomain.MemoryExtractionInput{
		ConversationID: "conv-3",
		UserID:         9,
		UserMessage: aidomain.Message{
			ID:      "msg-user-3",
			Role:    aidomain.RoleUser,
			Content: "请给我一个 memory writeback hook 的实现方案",
		},
		AssistantMessage: aidomain.Message{
			ID:   "msg-ai-3",
			Role: aidomain.RoleAssistant,
			Content: strings.Repeat(
				"先在流式成功收尾后触发写回，再抽取 summary facts documents，最后经过治理策略写入 MySQL。 ",
				3,
			),
		},
	})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("documents len = %d, want 1", len(result.Documents))
	}
	doc := result.Documents[0]
	if doc.ScopeType != aidomain.MemoryScopeSelf {
		t.Fatalf("doc scope = %q, want self", doc.ScopeType)
	}
	if doc.SourceID != "msg-ai-3" {
		t.Fatalf("source_id = %q, want msg-ai-3", doc.SourceID)
	}
	if doc.Topic != "conversation_knowledge" {
		t.Fatalf("topic = %q, want conversation_knowledge", doc.Topic)
	}
}
