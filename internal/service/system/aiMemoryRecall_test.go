package system

import (
	"context"
	"encoding/json"
	stderrors "errors"
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

func TestAIMemoryRecallMessagesInjectsRAGDocumentsInScoreOrder(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	service.embedder = &fakeMemoryEmbedder{vectors: [][]float32{{0.1, 0.2, 0.3}}}
	vectorSearcher := &fakeMemoryVectorStore{
		searchResults: []aidomain.MemoryVectorSearchResult{
			{QdrantPointID: "22222222-2222-2222-2222-222222222222", Score: 0.91},
			{QdrantPointID: "11111111-1111-1111-1111-111111111111", Score: 0.82},
			{QdrantPointID: "33333333-3333-3333-3333-333333333333", Score: 0.42},
		},
	}
	service.vectorSearcher = vectorSearcher
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableLongTermMemory: true,
		RecallTopK:           3,
		RecallMaxChars:       4000,
		RecallMinScore:       0.5,
		RAGMaxChars:          2000,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	ctx := context.Background()
	userID := uint(16)
	upsertAIMemoryRecallDocumentChunk(
		t,
		service,
		userID,
		"doc-rag-1",
		"chunk-rag-1",
		"11111111-1111-1111-1111-111111111111",
		"较低分但仍然命中的长期知识片段。",
	)
	upsertAIMemoryRecallDocumentChunk(
		t,
		service,
		userID,
		"doc-rag-2",
		"chunk-rag-2",
		"22222222-2222-2222-2222-222222222222",
		"最高分的长期知识片段，应排在前面。",
	)
	upsertAIMemoryRecallDocumentChunk(
		t,
		service,
		userID,
		"doc-rag-low-score",
		"chunk-rag-low-score",
		"33333333-3333-3333-3333-333333333333",
		"低分片段不应注入。",
	)

	messages, err := service.RecallMessages(ctx, aiMemoryRecallInput{
		ConversationID: "conv-rag-recall",
		UserID:         userID,
		Query:          "RAG 召回如何做？",
	})
	if err != nil {
		t.Fatalf("RecallMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("RecallMessages() len = %d, want 1", len(messages))
	}
	content := messages[0].Content
	assertAIMemoryRecallContains(t, content, "## Long-term Documents")
	assertAIMemoryRecallContains(t, content, "最高分的长期知识片段")
	assertAIMemoryRecallContains(t, content, "较低分但仍然命中的长期知识片段")
	if strings.Contains(content, "低分片段不应注入") {
		t.Fatalf("low score RAG content was injected:\n%s", content)
	}
	first := strings.Index(content, "最高分的长期知识片段")
	second := strings.Index(content, "较低分但仍然命中的长期知识片段")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("RAG order not preserved by Qdrant score:\n%s", content)
	}
	if vectorSearcher.searchInput.ScopeKey != aidomain.BuildSelfMemoryScopeKey(userID) ||
		vectorSearcher.searchInput.Visibility != string(aidomain.MemoryVisibilitySelf) ||
		vectorSearcher.searchInput.UserID != userID ||
		vectorSearcher.searchInput.Limit != 3 ||
		vectorSearcher.searchInput.MinScore != 0.5 {
		t.Fatalf("search input = %+v", vectorSearcher.searchInput)
	}
}

func TestAIMemoryRecallMessagesRAGFailureKeepsSummary(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	service.embedder = &fakeMemoryEmbedder{err: stderrors.New("embedding failed")}
	service.vectorSearcher = &fakeMemoryVectorStore{}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableLongTermMemory: true,
		RecallMaxChars:       4000,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	userID := uint(17)
	conversationID := "conv-rag-fail-open"
	upsertAIMemoryRecallSummary(t, service, conversationID, userID, "RAG 失败时仍应保留摘要。")

	messages, err := service.RecallMessages(context.Background(), aiMemoryRecallInput{
		ConversationID: conversationID,
		UserID:         userID,
		Query:          "触发 RAG 召回",
	})
	if err != nil {
		t.Fatalf("RecallMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("RecallMessages() len = %d, want 1", len(messages))
	}
	assertAIMemoryRecallContains(t, messages[0].Content, "RAG 失败时仍应保留摘要。")
	if strings.Contains(messages[0].Content, "## Long-term Documents") {
		t.Fatalf("RAG section should be empty on fail-open:\n%s", messages[0].Content)
	}
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

func upsertAIMemoryRecallDocumentChunk(
	t *testing.T,
	service *AIMemoryService,
	userID uint,
	documentID string,
	chunkID string,
	pointID string,
	content string,
) {
	t.Helper()
	now := time.Now()
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)
	doc := &entity.AIMemoryDocument{
		ID:          documentID,
		ScopeKey:    scopeKey,
		ScopeType:   string(aidomain.MemoryScopeSelf),
		Visibility:  string(aidomain.MemoryVisibilitySelf),
		UserID:      &userID,
		MemoryType:  string(aidomain.MemoryTypeSemantic),
		Topic:       "rag",
		Title:       "rag",
		Summary:     "rag summary",
		ContentText: content,
		SourceKind:  string(aidomain.MemorySourceModelInferred),
		SourceID:    chunkID,
	}
	if err := service.repo.BatchUpsertDocuments(context.Background(), []*entity.AIMemoryDocument{doc}); err != nil {
		t.Fatalf("BatchUpsertDocuments() error = %v", err)
	}
	if err := service.repo.ReplaceDocumentChunks(context.Background(), documentID, []*entity.AIMemoryDocumentChunk{
		{
			ID:                 chunkID,
			DocumentID:         documentID,
			ScopeKey:           scopeKey,
			ScopeType:          string(aidomain.MemoryScopeSelf),
			Visibility:         string(aidomain.MemoryVisibilitySelf),
			UserID:             &userID,
			MemoryType:         string(aidomain.MemoryTypeSemantic),
			Topic:              "rag",
			ChunkIndex:         0,
			ContentText:        content,
			ContentHash:        aidomain.BuildMemoryDocumentContentHash(content),
			EmbeddingModel:     "qwen3-vl-embedding",
			EmbeddingDimension: 3,
			QdrantPointID:      pointID,
			IndexedAt:          &now,
		},
	}); err != nil {
		t.Fatalf("ReplaceDocumentChunks() error = %v", err)
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
