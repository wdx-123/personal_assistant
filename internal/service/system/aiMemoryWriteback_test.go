package system

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	aimemory "personal_assistant/internal/infrastructure/ai/memory"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/entity"
	reposystem "personal_assistant/internal/repository/system"
)

func TestAIMemoryWritebackDisabledDoesNotWrite(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, aimemory.NewRuleExtractor(aimemory.Options{}))
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              false,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
	})
	defer restore()

	createAIWritebackMessages(t, db, "conv-disabled", "msg-user-disabled", "msg-ai-disabled", aiMessageStatusSuccess)

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-disabled",
		UserID:             7,
		UserMessageID:      "msg-user-disabled",
		AssistantMessageID: "msg-ai-disabled",
		Principal:          aidomain.AIToolPrincipal{UserID: 7},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	assertAIMemoryWritebackCount(t, db, &entity.AIConversationSummary{}, 0)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryFact{}, 0)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryDocument{}, 0)
}

func TestAIMemoryWritebackPersistsSummaryFactAndDocument(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, aimemory.NewRuleExtractor(aimemory.Options{DocumentMinRunes: 40}))
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
		EmbedModel:           "text-embedding-test",
	})
	defer restore()

	createAIWritebackMessagesWithContent(
		t,
		db,
		"conv-success",
		"msg-user-success",
		"msg-ai-success",
		"请记住以后请用更简洁的方式回答我，并给我一个 memory writeback hook 的实现方案",
		fmt.Sprintf("实现方案：%s", repeatMemoryText("流式成功收尾后触发写回，抽取 summary facts documents，再经过治理策略落库。", 4)),
		aiMessageStatusSuccess,
	)

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-success",
		UserID:             8,
		UserMessageID:      "msg-user-success",
		AssistantMessageID: "msg-ai-success",
		Principal:          aidomain.AIToolPrincipal{UserID: 8},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	assertAIMemoryWritebackCount(t, db, &entity.AIConversationSummary{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryFact{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryDocument{}, 1)

	var fact entity.AIMemoryFact
	if err := db.First(&fact).Error; err != nil {
		t.Fatalf("load fact: %v", err)
	}
	if fact.ScopeKey != aidomain.BuildSelfMemoryScopeKey(8) {
		t.Fatalf("fact scope_key = %q", fact.ScopeKey)
	}
	if fact.Namespace != aidomain.MemoryNamespaceUserPreference {
		t.Fatalf("fact namespace = %q", fact.Namespace)
	}

	var doc entity.AIMemoryDocument
	if err := db.First(&doc).Error; err != nil {
		t.Fatalf("load document: %v", err)
	}
	if doc.EmbeddingModel != "text-embedding-test" {
		t.Fatalf("embedding_model = %q, want text-embedding-test", doc.EmbeddingModel)
	}
	if doc.DedupKey == "" {
		t.Fatal("document dedup_key is empty")
	}
}

func TestAIMemoryWritebackPersistsModelInferredCandidates(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, staticMemoryExtractor{})
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
		EmbedModel:           "text-embedding-test",
	})
	defer restore()

	createAIWritebackMessagesWithContent(
		t,
		db,
		"conv-llm-success",
		"msg-user-llm-success",
		"msg-ai-llm-success",
		"请记住我的回答偏好",
		"以后可以更简洁地回答。",
		aiMessageStatusSuccess,
	)

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-llm-success",
		UserID:             18,
		UserMessageID:      "msg-user-llm-success",
		AssistantMessageID: "msg-ai-llm-success",
		Principal:          aidomain.AIToolPrincipal{UserID: 18},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	assertAIMemoryWritebackCount(t, db, &entity.AIConversationSummary{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryFact{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryDocument{}, 1)

	var fact entity.AIMemoryFact
	if err := db.First(&fact).Error; err != nil {
		t.Fatalf("load fact: %v", err)
	}
	if fact.SourceKind != string(aidomain.MemorySourceModelInferred) {
		t.Fatalf("fact source_kind = %q, want model_inferred", fact.SourceKind)
	}
	if fact.Confidence < 0.909 || fact.Confidence > 0.911 {
		t.Fatalf("fact confidence = %v, want 0.91", fact.Confidence)
	}
}

func TestAIMemoryWritebackAppliesTTLHintThroughPolicy(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, ttlHintMemoryExtractor{})
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:            true,
		EnableEntityMemory: true,
	})
	defer restore()

	createAIWritebackMessagesWithContent(
		t,
		db,
		"conv-ttl-hint",
		"msg-user-ttl-hint",
		"msg-ai-ttl-hint",
		"我最近 30 天目标是刷 200 道题。",
		"已记录你的阶段目标。",
		aiMessageStatusSuccess,
	)
	before := time.Now()

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-ttl-hint",
		UserID:             28,
		UserMessageID:      "msg-user-ttl-hint",
		AssistantMessageID: "msg-ai-ttl-hint",
		Principal:          aidomain.AIToolPrincipal{UserID: 28},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	var fact entity.AIMemoryFact
	if err := db.First(&fact).Error; err != nil {
		t.Fatalf("load fact: %v", err)
	}
	if fact.ExpiresAt == nil {
		t.Fatal("fact expires_at = nil, want ttl hint expiration")
	}
	assertAIMemoryWritebackTTLApproxDays(t, before, *fact.ExpiresAt, 30)
}

func TestAIMemoryWritebackFallsBackWhenLLMExtractorFails(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, aiMemoryFallbackExtractor{
		primary:  failingMemoryExtractor{err: stderrors.New("llm failed")},
		fallback: aimemory.NewRuleExtractor(aimemory.Options{DocumentMinRunes: 40}),
	})
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
		EmbedModel:           "text-embedding-test",
	})
	defer restore()

	createAIWritebackMessagesWithContent(
		t,
		db,
		"conv-llm-fallback",
		"msg-user-llm-fallback",
		"msg-ai-llm-fallback",
		"请记住以后请用更简洁的方式回答我，并给我一个 memory writeback hook 的实现方案",
		fmt.Sprintf("实现方案：%s", repeatMemoryText("流式成功收尾后触发写回，抽取 summary facts documents，再经过治理策略落库。", 4)),
		aiMessageStatusSuccess,
	)

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-llm-fallback",
		UserID:             19,
		UserMessageID:      "msg-user-llm-fallback",
		AssistantMessageID: "msg-ai-llm-fallback",
		Principal:          aidomain.AIToolPrincipal{UserID: 19},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	assertAIMemoryWritebackCount(t, db, &entity.AIConversationSummary{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryFact{}, 1)
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryDocument{}, 1)
}

func TestAIMemoryWritebackHeadUpdateKeepsSummaryTextAndBoundary(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, staticMemoryExtractor{})
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:                  true,
		EnableEntityMemory:       true,
		EnableLongTermMemory:     true,
		SummaryRefreshEveryTurns: 5,
	})
	defer restore()

	now := time.Now()
	createAIWritebackMessageRows(t, db,
		&entity.AIMessage{
			ID:             "msg-user-prev",
			ConversationID: "conv-head-update",
			Role:           aidomain.RoleUser,
			Content:        "上一轮问题",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		&entity.AIMessage{
			ID:             "msg-ai-prev",
			ConversationID: "conv-head-update",
			Role:           aidomain.RoleAssistant,
			Content:        "上一轮回答",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(time.Millisecond),
			UpdatedAt:      now.Add(time.Millisecond),
		},
		&entity.AIMessage{
			ID:             "msg-user-current",
			ConversationID: "conv-head-update",
			Role:           aidomain.RoleUser,
			Content:        "请继续细化当前方案",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(2 * time.Millisecond),
			UpdatedAt:      now.Add(2 * time.Millisecond),
		},
		&entity.AIMessage{
			ID:             "msg-ai-current",
			ConversationID: "conv-head-update",
			Role:           aidomain.RoleAssistant,
			Content:        "好的，我会继续细化。",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(3 * time.Millisecond),
			UpdatedAt:      now.Add(3 * time.Millisecond),
		},
	)
	if err := service.repo.UpsertConversationSummary(context.Background(), &entity.AIConversationSummary{
		ConversationID:           "conv-head-update",
		UserID:                   38,
		ScopeKey:                 aidomain.BuildSelfMemoryScopeKey(38),
		CompressedUntilMessageID: "msg-ai-prev",
		SummaryText:              "旧摘要正文",
		KeyPointsJSON:            `["旧关键点"]`,
		OpenLoopsJSON:            `["旧待办"]`,
		TokenEstimate:            6,
		CreatedAt:                now,
		UpdatedAt:                now,
	}); err != nil {
		t.Fatalf("upsert previous summary: %v", err)
	}

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-head-update",
		UserID:             38,
		UserMessageID:      "msg-user-current",
		AssistantMessageID: "msg-ai-current",
		Principal:          aidomain.AIToolPrincipal{UserID: 38},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	summary, err := service.repo.GetConversationSummary(context.Background(), aidomain.MemoryConversationSummaryQuery{
		ConversationID: "conv-head-update",
		UserID:         38,
		ScopeKey:       aidomain.BuildSelfMemoryScopeKey(38),
	})
	if err != nil {
		t.Fatalf("GetConversationSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("summary = nil")
	}
	if summary.SummaryText != "旧摘要正文" {
		t.Fatalf("SummaryText = %q, want old summary text kept", summary.SummaryText)
	}
	if summary.CompressedUntilMessageID != "msg-ai-prev" {
		t.Fatalf("CompressedUntilMessageID = %q, want msg-ai-prev", summary.CompressedUntilMessageID)
	}
	if !strings.Contains(summary.KeyPointsJSON, "偏好简洁") || !strings.Contains(summary.KeyPointsJSON, "旧关键点") {
		t.Fatalf("KeyPointsJSON = %s", summary.KeyPointsJSON)
	}
}

func TestAIMemoryWritebackFullRefreshAdvancesBoundary(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, staticMemoryExtractor{})
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:                  true,
		EnableEntityMemory:       true,
		EnableLongTermMemory:     true,
		SummaryRefreshEveryTurns: 1,
	})
	defer restore()

	now := time.Now()
	createAIWritebackMessageRows(t, db,
		&entity.AIMessage{
			ID:             "msg-user-prev",
			ConversationID: "conv-full-refresh",
			Role:           aidomain.RoleUser,
			Content:        "上一轮问题",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		&entity.AIMessage{
			ID:             "msg-ai-prev",
			ConversationID: "conv-full-refresh",
			Role:           aidomain.RoleAssistant,
			Content:        "上一轮回答",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(time.Millisecond),
			UpdatedAt:      now.Add(time.Millisecond),
		},
		&entity.AIMessage{
			ID:             "msg-user-current",
			ConversationID: "conv-full-refresh",
			Role:           aidomain.RoleUser,
			Content:        "请继续细化当前方案",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(2 * time.Millisecond),
			UpdatedAt:      now.Add(2 * time.Millisecond),
		},
		&entity.AIMessage{
			ID:             "msg-ai-current",
			ConversationID: "conv-full-refresh",
			Role:           aidomain.RoleAssistant,
			Content:        "好的，我会继续细化。",
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(3 * time.Millisecond),
			UpdatedAt:      now.Add(3 * time.Millisecond),
		},
	)
	if err := service.repo.UpsertConversationSummary(context.Background(), &entity.AIConversationSummary{
		ConversationID:           "conv-full-refresh",
		UserID:                   39,
		ScopeKey:                 aidomain.BuildSelfMemoryScopeKey(39),
		CompressedUntilMessageID: "msg-ai-prev",
		SummaryText:              "旧摘要正文",
		KeyPointsJSON:            `["旧关键点"]`,
		OpenLoopsJSON:            `["旧待办"]`,
		TokenEstimate:            6,
		CreatedAt:                now,
		UpdatedAt:                now,
	}); err != nil {
		t.Fatalf("upsert previous summary: %v", err)
	}

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-full-refresh",
		UserID:             39,
		UserMessageID:      "msg-user-current",
		AssistantMessageID: "msg-ai-current",
		Principal:          aidomain.AIToolPrincipal{UserID: 39},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}

	summary, err := service.repo.GetConversationSummary(context.Background(), aidomain.MemoryConversationSummaryQuery{
		ConversationID: "conv-full-refresh",
		UserID:         39,
		ScopeKey:       aidomain.BuildSelfMemoryScopeKey(39),
	})
	if err != nil {
		t.Fatalf("GetConversationSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatal("summary = nil")
	}
	if summary.SummaryText != "用户希望回答更简洁。" {
		t.Fatalf("SummaryText = %q, want refreshed summary text", summary.SummaryText)
	}
	if summary.CompressedUntilMessageID != "msg-ai-current" {
		t.Fatalf("CompressedUntilMessageID = %q, want msg-ai-current", summary.CompressedUntilMessageID)
	}
	if summary.KeyPointsJSON != `["偏好简洁"]` {
		t.Fatalf("KeyPointsJSON = %s, want refreshed draft", summary.KeyPointsJSON)
	}
}

func TestAIMemoryWritebackSkipsUnsuccessfulAssistantMessage(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, aimemory.NewRuleExtractor(aimemory.Options{}))
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
	})
	defer restore()

	createAIWritebackMessages(t, db, "conv-error", "msg-user-error", "msg-ai-error", aiMessageStatusError)

	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-error",
		UserID:             9,
		UserMessageID:      "msg-user-error",
		AssistantMessageID: "msg-ai-error",
		Principal:          aidomain.AIToolPrincipal{UserID: 9},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}
	assertAIMemoryWritebackCount(t, db, &entity.AIConversationSummary{}, 0)
}

func TestAIServiceTriggerMemoryWritebackSwallowsHookError(t *testing.T) {
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:        true,
		WritebackAsync: false,
	})
	defer restore()

	hook := &fakeMemoryWritebackHook{err: stderrors.New("writeback failed")}
	service := &AIService{memoryWriteback: hook}
	service.triggerMemoryWriteback(
		context.Background(),
		&entity.AIConversation{ID: "conv-hook", UserID: 10},
		&entity.AIMessage{ID: "msg-user-hook"},
		&entity.AIMessage{ID: "msg-ai-hook"},
		aidomain.AIToolPrincipal{UserID: 10},
	)

	if hook.calls != 1 {
		t.Fatalf("hook calls = %d, want 1", hook.calls)
	}
}

type fakeMemoryWritebackHook struct {
	calls int
	err   error
}

func (f *fakeMemoryWritebackHook) OnTurnCompleted(context.Context, aiMemoryWritebackInput) error {
	f.calls++
	return f.err
}

type failingMemoryExtractor struct {
	err error
}

func (f failingMemoryExtractor) Extract(
	context.Context,
	aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	return aidomain.MemoryExtractionResult{}, f.err
}

type staticMemoryExtractor struct{}

func (staticMemoryExtractor) Extract(
	_ context.Context,
	input aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	userID := input.UserID
	return aidomain.MemoryExtractionResult{
		Summary: &aidomain.ConversationSummaryDraft{
			ConversationID:           input.ConversationID,
			CompressedUntilMessageID: input.AssistantMessage.ID,
			SummaryText:              "用户希望回答更简洁。",
			KeyPointsJSON:            `["偏好简洁"]`,
			OpenLoopsJSON:            `[]`,
			TokenEstimate:            8,
		},
		Facts: []aidomain.MemoryFactCandidate{
			{
				ScopeType:     aidomain.MemoryScopeSelf,
				UserID:        &userID,
				Namespace:     aidomain.MemoryNamespaceUserPreference,
				FactKey:       "answer_style",
				FactValueJSON: `{"value":"更简洁"}`,
				Summary:       "用户偏好简洁回答",
				Confidence:    0.91,
				TTLHint: &aidomain.MemoryTTLHint{
					Kind:       aidomain.MemoryTTLHintPersistent,
					Reason:     "用户表达的是长期回答偏好",
					Confidence: 0.9,
				},
				SourceKind: aidomain.MemorySourceModelInferred,
				SourceID:   input.AssistantMessage.ID,
			},
		},
		Documents: []aidomain.MemoryDocumentCandidate{
			{
				ScopeType:   aidomain.MemoryScopeSelf,
				UserID:      &userID,
				MemoryType:  aidomain.MemoryTypeSemantic,
				Topic:       "memory_governance",
				Title:       "LLM 提议与 Policy 裁决",
				Summary:     "Prompt 是治理意图，Policy 是治理裁决。",
				ContentText: "LLM 只提议候选记忆，Service policy 负责权限、TTL、去重、覆盖和最终落库。",
				Confidence:  0.86,
				SourceKind:  aidomain.MemorySourceModelInferred,
				SourceID:    input.AssistantMessage.ID,
			},
		},
	}, nil
}

type ttlHintMemoryExtractor struct{}

func (ttlHintMemoryExtractor) Extract(
	_ context.Context,
	input aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	userID := input.UserID
	return aidomain.MemoryExtractionResult{
		Facts: []aidomain.MemoryFactCandidate{
			{
				ScopeType:     aidomain.MemoryScopeSelf,
				UserID:        &userID,
				Namespace:     aidomain.MemoryNamespaceOJGoal,
				FactKey:       "current_goal",
				FactValueJSON: `{"goal":"200 problems in 30 days"}`,
				Summary:       "最近 30 天目标是刷 200 道题",
				Confidence:    0.94,
				TTLHint: &aidomain.MemoryTTLHint{
					Kind:       aidomain.MemoryTTLHintDuration,
					Value:      30,
					Unit:       "day",
					Reason:     "用户明确说最近 30 天目标",
					Confidence: 0.93,
				},
				SourceKind: aidomain.MemorySourceModelInferred,
				SourceID:   input.AssistantMessage.ID,
			},
		},
	}, nil
}

func newAIMemoryWritebackTestService(db *gorm.DB, extractor aidomain.MemoryExtractor) *AIMemoryService {
	return &AIMemoryService{
		aiRepo:    reposystem.NewAIRepository(db),
		repo:      reposystem.NewAIMemoryRepository(db),
		policy:    aiMemoryPolicy{},
		extractor: extractor,
	}
}

func newAIMemoryWritebackTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.AIConversation{},
		&entity.AIMessage{},
		&entity.AIMemoryFact{},
		&entity.AIMemoryDocument{},
		&entity.AIMemoryDocumentChunk{},
		&entity.AIConversationSummary{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func setAIMemoryTestConfig(t *testing.T, memory config.AIMemory) func() {
	t.Helper()
	previous := global.Config
	global.Config = &config.Config{AI: config.AI{Memory: memory}}
	return func() {
		global.Config = previous
	}
}

func createAIWritebackMessages(
	t *testing.T,
	db *gorm.DB,
	conversationID string,
	userMessageID string,
	assistantMessageID string,
	assistantStatus string,
) {
	t.Helper()
	createAIWritebackMessagesWithContent(
		t,
		db,
		conversationID,
		userMessageID,
		assistantMessageID,
		"请记住以后请用简洁方式回答。",
		"好的，我会记住。",
		assistantStatus,
	)
}

func createAIWritebackMessagesWithContent(
	t *testing.T,
	db *gorm.DB,
	conversationID string,
	userMessageID string,
	assistantMessageID string,
	userContent string,
	assistantContent string,
	assistantStatus string,
) {
	t.Helper()
	now := time.Now()
	messages := []*entity.AIMessage{
		{
			ID:             userMessageID,
			ConversationID: conversationID,
			Role:           aidomain.RoleUser,
			Content:        userContent,
			Status:         aiMessageStatusSuccess,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             assistantMessageID,
			ConversationID: conversationID,
			Role:           aidomain.RoleAssistant,
			Content:        assistantContent,
			Status:         assistantStatus,
			TraceItemsJSON: "[]",
			ScopeJSON:      "{}",
			CreatedAt:      now.Add(time.Millisecond),
			UpdatedAt:      now.Add(time.Millisecond),
		},
	}
	if err := db.Create(messages).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}
}

func createAIWritebackMessageRows(t *testing.T, db *gorm.DB, messages ...*entity.AIMessage) {
	t.Helper()
	if err := db.Create(messages).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}
}

func assertAIMemoryWritebackCount(t *testing.T, db *gorm.DB, model any, want int64) {
	t.Helper()
	var count int64
	if err := db.Model(model).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	if count != want {
		t.Fatalf("count %T = %d, want %d", model, count, want)
	}
}

func repeatMemoryText(value string, times int) string {
	var builder strings.Builder
	for i := 0; i < times; i++ {
		builder.WriteString(value)
	}
	return builder.String()
}

func assertAIMemoryWritebackTTLApproxDays(t *testing.T, base time.Time, expiresAt time.Time, wantDays int) {
	t.Helper()
	got := expiresAt.Sub(base)
	want := time.Duration(wantDays) * 24 * time.Hour
	if got < want-time.Minute || got > want+time.Minute {
		t.Fatalf("ttl duration = %s, want about %s", got, want)
	}
}
