package system

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/entity"
)

func TestAIMemoryRecallMessagesDisabledReturnsEmpty(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	restore := setAIMemoryTestConfig(t, config.AIMemory{Enabled: false})
	defer restore()

	messages, err := service.RecallMessages(context.Background(), aiMemoryRecallInput{
		ConversationID: "conv-disabled-recall",
		UserID:         12,
		Query:          "帮我恢复上下文",
	})
	if err != nil {
		t.Fatalf("RecallMessages() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("RecallMessages() len = %d, want 0", len(messages))
	}
}

func TestAIMemoryRecallMessagesBuildsSummaryAndFactsContext(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:            true,
		EnableEntityMemory: true,
		RecallTopK:         5,
		RecallMaxChars:     2000,
	})
	defer restore()

	ctx := context.Background()
	userID := uint(13)
	conversationID := "conv-recall"
	upsertAIMemoryRecallSummary(t, service, conversationID, userID, "用户确认采用 summary + recent turns 的上下文恢复方案。")
	upsertAIMemoryRecallFact(t, service, userID, "answer_style", "以后回答尽量简洁，并优先给出可执行步骤。")

	messages, err := service.RecallMessages(ctx, aiMemoryRecallInput{
		ConversationID: conversationID,
		UserID:         userID,
		Query:          "下一步怎么实现压缩？",
		ToolCallCtx: aidomain.ToolCallContext{
			Principal: aidomain.AIToolPrincipal{UserID: userID},
		},
	})
	if err != nil {
		t.Fatalf("RecallMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("RecallMessages() len = %d, want 1", len(messages))
	}
	message := messages[0]
	if message.Role != aidomain.RoleAssistant {
		t.Fatalf("memory role = %q, want assistant", message.Role)
	}
	assertAIMemoryRecallContains(t, message.Content, aiMemoryContextMessageHeader)
	assertAIMemoryRecallContains(t, message.Content, "## Conversation Summary")
	assertAIMemoryRecallContains(t, message.Content, "用户确认采用 summary + recent turns 的上下文恢复方案。")
	assertAIMemoryRecallContains(t, message.Content, "## Stable Facts")
	assertAIMemoryRecallContains(t, message.Content, "user_preference/answer_style")
	assertAIMemoryRecallContains(t, message.Content, "以后回答尽量简洁")
	assertAIMemoryRecallContains(t, message.Content, "## Current Query")
	assertAIMemoryRecallContains(t, message.Content, "下一步怎么实现压缩？")
}

func TestAIMemoryRecallMessagesEmptyWhenNoSummaryOrFacts(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:            true,
		EnableEntityMemory: true,
	})
	defer restore()

	messages, err := service.RecallMessages(context.Background(), aiMemoryRecallInput{
		ConversationID: "conv-empty-recall",
		UserID:         14,
		Query:          "没有记忆时不应注入",
	})
	if err != nil {
		t.Fatalf("RecallMessages() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("RecallMessages() len = %d, want 0", len(messages))
	}
}

func TestAIMemoryCompressMessagesKeepsBelowThresholdHistory(t *testing.T) {
	service := &AIMemoryService{}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:                 true,
		CompressThresholdTokens: 1000,
	})
	defer restore()

	input := []aidomain.Message{
		{ID: "msg-1", Role: aidomain.RoleUser, Content: "第一句"},
		{ID: "msg-2", Role: aidomain.RoleAssistant, Content: "第二句"},
	}
	output, err := service.CompressMessages(context.Background(), aiContextCompressionInput{Messages: input})
	if err != nil {
		t.Fatalf("CompressMessages() error = %v", err)
	}
	if len(output) != len(input) || output[0].ID != "msg-1" || output[1].ID != "msg-2" {
		t.Fatalf("CompressMessages() = %+v, want original order", output)
	}
}

func TestAIMemoryCompressMessagesKeepsMemoryAndRecentTurns(t *testing.T) {
	service := &AIMemoryService{}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:                 true,
		RecentRawTurns:          2,
		CompressThresholdTokens: 1,
	})
	defer restore()

	input := []aidomain.Message{
		{ID: "msg-1", Role: aidomain.RoleUser, Content: "message one content"},
		{ID: "msg-2", Role: aidomain.RoleAssistant, Content: "message two content"},
		{ID: "msg-3", Role: aidomain.RoleUser, Content: "message three content"},
		{ID: "msg-4", Role: aidomain.RoleAssistant, Content: "message four content"},
		{ID: "msg-5", Role: aidomain.RoleUser, Content: "message five content"},
		{ID: "msg-6", Role: aidomain.RoleAssistant, Content: "message six content"},
		{ID: "memory_context_conv", Role: aidomain.RoleAssistant, Content: aiMemoryContextMessageHeader + "\nsummary"},
	}

	output, err := service.CompressMessages(context.Background(), aiContextCompressionInput{Messages: input})
	if err != nil {
		t.Fatalf("CompressMessages() error = %v", err)
	}
	wantIDs := []string{"memory_context_conv", "msg-3", "msg-4", "msg-5", "msg-6"}
	if len(output) != len(wantIDs) {
		t.Fatalf("CompressMessages() len = %d, want %d: %+v", len(output), len(wantIDs), output)
	}
	for i, want := range wantIDs {
		if output[i].ID != want {
			t.Fatalf("CompressMessages()[%d].ID = %q, want %q", i, output[i].ID, want)
		}
	}
}

func TestDefaultAIContextAssemblerRecallsAndCompressesWithAIMemoryService(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:                 true,
		EnableEntityMemory:      true,
		RecentRawTurns:          1,
		CompressThresholdTokens: 1,
		RecallMaxChars:          2000,
	})
	defer restore()

	userID := uint(15)
	conversationID := "conv-assembler-recall"
	upsertAIMemoryRecallSummary(t, service, conversationID, userID, "旧历史已压缩成摘要，关键决策是采用 memory + recent turns。")
	assembler := newAIContextAssembler(AIDeps{
		Memory:     service,
		Compressor: service,
	})

	snapshot, err := assembler.Build(context.Background(), aiContextBuildArgs{
		ConversationID: conversationID,
		UserID:         userID,
		Query:          "继续实现",
		StoredMessages: []*entity.AIMessage{
			{ID: "msg-1", Role: aidomain.RoleUser, Content: "很早的用户消息"},
			{ID: "msg-2", Role: aidomain.RoleAssistant, Content: "很早的助手消息"},
			{ID: "msg-3", Role: aidomain.RoleUser, Content: "最近的用户消息"},
			{ID: "msg-4", Role: aidomain.RoleAssistant, Content: "最近的助手消息"},
		},
		ToolCallCtx: aidomain.ToolCallContext{
			Principal: aidomain.AIToolPrincipal{UserID: userID},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	wantIDs := []string{aiMemoryContextMessageIDPrefix + "_" + conversationID, "msg-3", "msg-4"}
	if len(snapshot.History) != len(wantIDs) {
		t.Fatalf("History len = %d, want %d: %+v", len(snapshot.History), len(wantIDs), snapshot.History)
	}
	for i, want := range wantIDs {
		if snapshot.History[i].ID != want {
			t.Fatalf("History[%d].ID = %q, want %q", i, snapshot.History[i].ID, want)
		}
	}
	assertAIMemoryRecallContains(t, snapshot.History[0].Content, "旧历史已压缩成摘要")
}

func upsertAIMemoryRecallSummary(
	t *testing.T,
	service *AIMemoryService,
	conversationID string,
	userID uint,
	summary string,
) {
	t.Helper()
	now := time.Now()
	if err := service.repo.UpsertConversationSummary(context.Background(), &entity.AIConversationSummary{
		ConversationID: conversationID,
		UserID:         userID,
		ScopeKey:       aidomain.BuildSelfMemoryScopeKey(userID),
		SummaryText:    summary,
		KeyPointsJSON:  "[]",
		OpenLoopsJSON:  "[]",
		TokenEstimate:  estimateAIMemoryTokens([]aidomain.Message{{Content: summary}}),
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}
}

func upsertAIMemoryRecallFact(t *testing.T, service *AIMemoryService, userID uint, factKey string, summary string) {
	t.Helper()
	now := time.Now()
	if err := service.repo.UpsertFact(context.Background(), &entity.AIMemoryFact{
		ScopeKey:      aidomain.BuildSelfMemoryScopeKey(userID),
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilitySelf),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       factKey,
		FactValueJSON: fmtAIMemoryRecallFactValue(summary),
		Summary:       summary,
		Confidence:    0.9,
		SourceKind:    string(aidomain.MemorySourceExplicitUserStatement),
		SourceID:      "msg-fact",
		EffectiveAt:   &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("upsert fact: %v", err)
	}
}

func fmtAIMemoryRecallFactValue(summary string) string {
	payload, _ := json.Marshal(map[string]string{"value": summary})
	return string(payload)
}

func assertAIMemoryRecallContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("value does not contain %q:\n%s", want, value)
	}
}
