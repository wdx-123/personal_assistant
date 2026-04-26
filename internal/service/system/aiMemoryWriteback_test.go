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
