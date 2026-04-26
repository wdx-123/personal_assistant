package system

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
)

func TestAIMemoryModelsAutoMigrateAddsExpectedTablesAndIndexes(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)

	if !db.Migrator().HasTable(&entity.AIMemoryFact{}) {
		t.Fatal("ai_memory_facts table missing")
	}
	if !db.Migrator().HasTable(&entity.AIMemoryDocument{}) {
		t.Fatal("ai_memory_documents table missing")
	}
	if !db.Migrator().HasTable(&entity.AIMemoryDocumentChunk{}) {
		t.Fatal("ai_memory_document_chunks table missing")
	}
	if !db.Migrator().HasTable(&entity.AIConversationSummary{}) {
		t.Fatal("ai_conversation_summaries table missing")
	}

	assertHasIndex(t, db, &entity.AIMemoryFact{}, "uk_ai_memory_facts_scope_key_namespace_fact_key")
	assertHasIndex(t, db, &entity.AIMemoryFact{}, "idx_ai_memory_facts_scope_key_updated_at")
	assertHasIndex(t, db, &entity.AIMemoryFact{}, "idx_ai_memory_facts_expires_at")
	assertHasIndex(t, db, &entity.AIMemoryDocument{}, "idx_ai_memory_documents_scope_key_memory_type_updated_at")
	assertHasIndex(t, db, &entity.AIMemoryDocument{}, "idx_ai_memory_documents_topic_updated_at")
	assertHasIndex(t, db, &entity.AIMemoryDocument{}, "idx_ai_memory_documents_expires_at")
	assertHasIndex(t, db, &entity.AIMemoryDocument{}, "uk_ai_memory_documents_scope_key_dedup_key")
	assertHasIndex(t, db, &entity.AIMemoryDocumentChunk{}, "idx_ai_memory_document_chunks_document_id")
	assertHasIndex(t, db, &entity.AIMemoryDocumentChunk{}, "idx_ai_memory_document_chunks_document_index")
	assertHasIndex(t, db, &entity.AIMemoryDocumentChunk{}, "uk_ai_memory_document_chunks_qdrant_point_id")
	assertHasIndex(t, db, &entity.AIConversationSummary{}, "idx_ai_conversation_summaries_scope_key")
	assertHasIndex(t, db, &entity.AIConversationSummary{}, "idx_ai_conversation_summaries_user_id")
	assertHasIndex(t, db, &entity.AIConversationSummary{}, "idx_ai_conversation_summaries_org_id")
	assertHasIndex(t, db, &entity.AIConversationSummary{}, "idx_ai_conversation_summaries_updated_at")
}

func TestAIMemoryRepositoryUpsertFactOverwritesByUniqueKey(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(101)
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)

	fact := &entity.AIMemoryFact{
		ScopeKey:      scopeKey,
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilitySelf),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       "answer_style",
		FactValueJSON: `{"style":"brief"}`,
		Summary:       "prefer brief answer",
		Confidence:    0.82,
		SourceKind:    "conversation",
		SourceID:      "msg-1",
	}
	if err := repo.UpsertFact(ctx, fact); err != nil {
		t.Fatalf("UpsertFact(create) error = %v", err)
	}

	updated := &entity.AIMemoryFact{
		ScopeKey:      scopeKey,
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilitySelf),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       "answer_style",
		FactValueJSON: `{"style":"step_by_step"}`,
		Summary:       "prefer step-by-step answer",
		Confidence:    0.95,
		SourceKind:    "conversation",
		SourceID:      "msg-2",
	}
	if err := repo.UpsertFact(ctx, updated); err != nil {
		t.Fatalf("UpsertFact(update) error = %v", err)
	}

	var rows []entity.AIMemoryFact
	if err := db.Model(&entity.AIMemoryFact{}).Find(&rows).Error; err != nil {
		t.Fatalf("load facts: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("facts row count = %d, want 1", len(rows))
	}
	if rows[0].Summary != updated.Summary {
		t.Fatalf("fact summary = %q, want %q", rows[0].Summary, updated.Summary)
	}
	if rows[0].FactValueJSON != updated.FactValueJSON {
		t.Fatalf("fact value = %q, want %q", rows[0].FactValueJSON, updated.FactValueJSON)
	}
	if rows[0].SourceID != updated.SourceID {
		t.Fatalf("fact source_id = %q, want %q", rows[0].SourceID, updated.SourceID)
	}
}

func TestAIMemoryRepositoryListFactsFiltersExpiredAndVisibility(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(102)
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)
	expiredAt := time.Now().Add(-time.Hour)

	mustUpsertFact(t, repo, ctx, &entity.AIMemoryFact{
		ScopeKey:      scopeKey,
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilitySelf),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       "language",
		FactValueJSON: `{"value":"go"}`,
		Summary:       "prefer go",
	})
	mustUpsertFact(t, repo, ctx, &entity.AIMemoryFact{
		ScopeKey:      scopeKey,
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilitySelf),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       "legacy_goal",
		FactValueJSON: `{"value":"expired"}`,
		Summary:       "expired",
		ExpiresAt:     &expiredAt,
	})
	mustUpsertFact(t, repo, ctx, &entity.AIMemoryFact{
		ScopeKey:      scopeKey,
		ScopeType:     string(aidomain.MemoryScopeSelf),
		Visibility:    string(aidomain.MemoryVisibilityOrg),
		UserID:        &userID,
		Namespace:     aidomain.MemoryNamespaceUserPreference,
		FactKey:       "org_only",
		FactValueJSON: `{"value":"hidden"}`,
		Summary:       "hidden by visibility",
	})

	rows, err := repo.ListFacts(ctx, aidomain.MemoryFactQuery{
		ScopeKeys:           []string{scopeKey},
		AllowedVisibilities: []aidomain.MemoryVisibility{aidomain.MemoryVisibilitySelf},
		Namespace:           aidomain.MemoryNamespaceUserPreference,
	})
	if err != nil {
		t.Fatalf("ListFacts() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListFacts() count = %d, want 1", len(rows))
	}
	if rows[0].FactKey != "language" {
		t.Fatalf("ListFacts()[0].FactKey = %q, want %q", rows[0].FactKey, "language")
	}
}

func TestAIMemoryRepositoryListDocumentsFiltersExpiredAndSoftDeleted(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(103)
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)
	expiredAt := time.Now().Add(-time.Hour)

	docs := []*entity.AIMemoryDocument{
		{
			ID:          "doc-active",
			ScopeKey:    scopeKey,
			ScopeType:   string(aidomain.MemoryScopeSelf),
			Visibility:  string(aidomain.MemoryVisibilitySelf),
			UserID:      &userID,
			MemoryType:  string(aidomain.MemoryTypeSemantic),
			Topic:       "crawler",
			Title:       "active",
			Summary:     "active",
			ContentText: "active content",
		},
		{
			ID:          "doc-expired",
			ScopeKey:    scopeKey,
			ScopeType:   string(aidomain.MemoryScopeSelf),
			Visibility:  string(aidomain.MemoryVisibilitySelf),
			UserID:      &userID,
			MemoryType:  string(aidomain.MemoryTypeSemantic),
			Topic:       "crawler",
			Title:       "expired",
			Summary:     "expired",
			ContentText: "expired content",
			ExpiresAt:   &expiredAt,
		},
		{
			ID:          "doc-deleted",
			ScopeKey:    scopeKey,
			ScopeType:   string(aidomain.MemoryScopeSelf),
			Visibility:  string(aidomain.MemoryVisibilitySelf),
			UserID:      &userID,
			MemoryType:  string(aidomain.MemoryTypeSemantic),
			Topic:       "crawler",
			Title:       "deleted",
			Summary:     "deleted",
			ContentText: "deleted content",
		},
	}
	if err := repo.BatchUpsertDocuments(ctx, docs); err != nil {
		t.Fatalf("BatchUpsertDocuments() error = %v", err)
	}
	if err := db.Delete(&entity.AIMemoryDocument{}, "id = ?", "doc-deleted").Error; err != nil {
		t.Fatalf("soft delete document: %v", err)
	}

	rows, err := repo.ListDocuments(ctx, aidomain.MemoryDocumentQuery{
		ScopeKeys:           []string{scopeKey},
		AllowedVisibilities: []aidomain.MemoryVisibility{aidomain.MemoryVisibilitySelf},
		MemoryTypes:         []aidomain.MemoryType{aidomain.MemoryTypeSemantic},
		Topic:               "crawler",
	})
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListDocuments() count = %d, want 1", len(rows))
	}
	if rows[0].ID != "doc-active" {
		t.Fatalf("ListDocuments()[0].ID = %q, want %q", rows[0].ID, "doc-active")
	}
}

func TestAIMemoryRepositoryBatchUpsertDocumentsDedupsByScopeAndDedupKey(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(104)
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)

	first := &entity.AIMemoryDocument{
		ID:          "doc-faq-1",
		ScopeKey:    scopeKey,
		ScopeType:   string(aidomain.MemoryScopeSelf),
		Visibility:  string(aidomain.MemoryVisibilitySelf),
		UserID:      &userID,
		MemoryType:  string(aidomain.MemoryTypeFAQ),
		Topic:       "deploy",
		Title:       "faq",
		Summary:     "How to deploy",
		ContentText: "Use docker compose for local deployment",
		SourceKind:  "faq_import",
		SourceID:    "faq-001",
	}
	second := &entity.AIMemoryDocument{
		ID:          "doc-faq-2",
		ScopeKey:    scopeKey,
		ScopeType:   string(aidomain.MemoryScopeSelf),
		Visibility:  string(aidomain.MemoryVisibilitySelf),
		UserID:      &userID,
		MemoryType:  string(aidomain.MemoryTypeFAQ),
		Topic:       "deploy",
		Title:       "faq updated",
		Summary:     "How to deploy safely",
		ContentText: "Use docker compose and run migrations before release",
		SourceKind:  "faq_import",
		SourceID:    "faq-001",
	}

	if err := repo.BatchUpsertDocuments(ctx, []*entity.AIMemoryDocument{first}); err != nil {
		t.Fatalf("BatchUpsertDocuments(first) error = %v", err)
	}
	if err := repo.BatchUpsertDocuments(ctx, []*entity.AIMemoryDocument{second}); err != nil {
		t.Fatalf("BatchUpsertDocuments(second) error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.AIMemoryDocument{}).Count(&count).Error; err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("document row count = %d, want 1", count)
	}

	row := &entity.AIMemoryDocument{}
	if err := db.First(row).Error; err != nil {
		t.Fatalf("load deduped document: %v", err)
	}
	if row.ID != first.ID {
		t.Fatalf("deduped document id = %q, want preserved existing id %q", row.ID, first.ID)
	}
	if row.Summary != second.Summary {
		t.Fatalf("deduped document summary = %q, want %q", row.Summary, second.Summary)
	}
	if row.ContentHash == "" || row.SummaryHash == "" || row.DedupKey == "" {
		t.Fatalf("dedup metadata not generated: %+v", row)
	}
	wantDedupKey := aidomain.BuildMemoryDocumentDedupKey(
		aidomain.MemorySourceKind(second.SourceKind),
		second.SourceID,
		second.Topic,
		second.Summary,
		second.ContentText,
	)
	if row.DedupKey != wantDedupKey {
		t.Fatalf("dedup_key = %q, want %q", row.DedupKey, wantDedupKey)
	}
}

func TestAIMemoryRepositoryUpsertConversationSummaryOverwritesByConversationID(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(105)
	orgID := uint(205)

	first := &entity.AIConversationSummary{
		ConversationID:           "conv-1",
		UserID:                   userID,
		ScopeKey:                 aidomain.BuildConversationMemoryScopeKey(userID, nil),
		CompressedUntilMessageID: "msg-1",
		SummaryText:              "first summary",
		KeyPointsJSON:            `["a"]`,
		OpenLoopsJSON:            `[]`,
		TokenEstimate:            120,
	}
	if err := repo.UpsertConversationSummary(ctx, first); err != nil {
		t.Fatalf("UpsertConversationSummary(create) error = %v", err)
	}

	second := &entity.AIConversationSummary{
		ConversationID:           "conv-1",
		UserID:                   userID,
		OrgID:                    &orgID,
		ScopeKey:                 aidomain.BuildConversationMemoryScopeKey(userID, &orgID),
		CompressedUntilMessageID: "msg-9",
		SummaryText:              "updated summary",
		KeyPointsJSON:            `["a","b"]`,
		OpenLoopsJSON:            `["follow-up"]`,
		TokenEstimate:            256,
	}
	if err := repo.UpsertConversationSummary(ctx, second); err != nil {
		t.Fatalf("UpsertConversationSummary(update) error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.AIConversationSummary{}).Count(&count).Error; err != nil {
		t.Fatalf("count summaries: %v", err)
	}
	if count != 1 {
		t.Fatalf("summary row count = %d, want 1", count)
	}

	row, err := repo.GetConversationSummary(ctx, aidomain.MemoryConversationSummaryQuery{
		ConversationID: "conv-1",
		UserID:         userID,
		OrgID:          &orgID,
		ScopeKey:       second.ScopeKey,
	})
	if err != nil {
		t.Fatalf("GetConversationSummary() error = %v", err)
	}
	if row == nil {
		t.Fatal("GetConversationSummary() returned nil row")
	}
	if row.ScopeKey != second.ScopeKey {
		t.Fatalf("summary scope_key = %q, want %q", row.ScopeKey, second.ScopeKey)
	}
	if row.CompressedUntilMessageID != second.CompressedUntilMessageID {
		t.Fatalf(
			"summary compressed_until_message_id = %q, want %q",
			row.CompressedUntilMessageID,
			second.CompressedUntilMessageID,
		)
	}
	if row.OrgID == nil || *row.OrgID != orgID {
		t.Fatalf("summary org_id = %v, want %d", row.OrgID, orgID)
	}
}

func TestAIMemoryRepositoryGetConversationSummaryRejectsMismatchedIdentity(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(106)
	orgID := uint(206)

	summary := &entity.AIConversationSummary{
		ConversationID:           "conv-2",
		UserID:                   userID,
		OrgID:                    &orgID,
		ScopeKey:                 aidomain.BuildConversationMemoryScopeKey(userID, &orgID),
		CompressedUntilMessageID: "msg-3",
		SummaryText:              "summary",
		KeyPointsJSON:            `["a"]`,
		OpenLoopsJSON:            `[]`,
		TokenEstimate:            42,
	}
	if err := repo.UpsertConversationSummary(ctx, summary); err != nil {
		t.Fatalf("UpsertConversationSummary() error = %v", err)
	}

	mismatchCases := []aidomain.MemoryConversationSummaryQuery{
		{
			ConversationID: "conv-2",
			UserID:         999,
			OrgID:          &orgID,
			ScopeKey:       summary.ScopeKey,
		},
		{
			ConversationID: "conv-2",
			UserID:         userID,
			OrgID:          nil,
			ScopeKey:       summary.ScopeKey,
		},
		{
			ConversationID: "conv-2",
			UserID:         userID,
			OrgID:          &orgID,
			ScopeKey:       aidomain.BuildSelfMemoryScopeKey(userID),
		},
	}

	for _, query := range mismatchCases {
		row, err := repo.GetConversationSummary(ctx, query)
		if err != nil {
			t.Fatalf("GetConversationSummary(%+v) error = %v", query, err)
		}
		if row != nil {
			t.Fatalf("GetConversationSummary(%+v) = %+v, want nil", query, row)
		}
	}
}

func TestAIMemoryRepositoryReplaceDocumentChunksOverwrites(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	now := time.Now()

	first := []*entity.AIMemoryDocumentChunk{
		{
			ID:                 "chunk-1",
			DocumentID:         "doc-chunks",
			ScopeKey:           "self:user:1",
			ScopeType:          string(aidomain.MemoryScopeSelf),
			Visibility:         string(aidomain.MemoryVisibilitySelf),
			MemoryType:         string(aidomain.MemoryTypeSemantic),
			ChunkIndex:         0,
			ContentText:        "first",
			ContentHash:        "hash-first",
			EmbeddingModel:     "qwen3-vl-embedding",
			EmbeddingDimension: 1024,
			QdrantPointID:      "11111111-1111-1111-1111-111111111111",
			IndexedAt:          &now,
		},
	}
	if err := repo.ReplaceDocumentChunks(ctx, "doc-chunks", first); err != nil {
		t.Fatalf("ReplaceDocumentChunks(first) error = %v", err)
	}
	second := []*entity.AIMemoryDocumentChunk{
		{
			ID:                 "chunk-2",
			DocumentID:         "doc-chunks",
			ScopeKey:           "self:user:1",
			ScopeType:          string(aidomain.MemoryScopeSelf),
			Visibility:         string(aidomain.MemoryVisibilitySelf),
			MemoryType:         string(aidomain.MemoryTypeSemantic),
			ChunkIndex:         0,
			ContentText:        "second",
			ContentHash:        "hash-second",
			EmbeddingModel:     "qwen3-vl-embedding",
			EmbeddingDimension: 1024,
			QdrantPointID:      "22222222-2222-2222-2222-222222222222",
			IndexedAt:          &now,
		},
	}
	if err := repo.ReplaceDocumentChunks(ctx, "doc-chunks", second); err != nil {
		t.Fatalf("ReplaceDocumentChunks(second) error = %v", err)
	}
	rows, err := repo.ListDocumentChunks(ctx, "doc-chunks")
	if err != nil {
		t.Fatalf("ListDocumentChunks() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "chunk-2" {
		t.Fatalf("chunks = %+v, want only chunk-2", rows)
	}
}

func TestAIMemoryRepositoryListDocumentsNeedingIndex(t *testing.T) {
	db := newAIMemoryRepositoryTestDB(t)
	repo := NewAIMemoryRepository(db)
	ctx := context.Background()
	userID := uint(107)
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)
	doc := &entity.AIMemoryDocument{
		ID:          "doc-index-needed",
		ScopeKey:    scopeKey,
		ScopeType:   string(aidomain.MemoryScopeSelf),
		Visibility:  string(aidomain.MemoryVisibilitySelf),
		UserID:      &userID,
		MemoryType:  string(aidomain.MemoryTypeSemantic),
		Topic:       "design",
		Title:       "design",
		Summary:     "summary",
		ContentText: "content",
		SourceKind:  string(aidomain.MemorySourceModelInferred),
		SourceID:    "msg-1",
	}
	if err := repo.BatchUpsertDocuments(ctx, []*entity.AIMemoryDocument{doc}); err != nil {
		t.Fatalf("BatchUpsertDocuments() error = %v", err)
	}
	rows, err := repo.ListDocumentsNeedingIndex(ctx, 10, "qwen3-vl-embedding", 1024)
	if err != nil {
		t.Fatalf("ListDocumentsNeedingIndex() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != doc.ID {
		t.Fatalf("documents needing index = %+v, want %s", rows, doc.ID)
	}

	indexedAt := time.Now().Add(time.Hour)
	if err := repo.ReplaceDocumentChunks(ctx, doc.ID, []*entity.AIMemoryDocumentChunk{
		{
			ID:                 "chunk-indexed",
			DocumentID:         doc.ID,
			ScopeKey:           doc.ScopeKey,
			ScopeType:          doc.ScopeType,
			Visibility:         doc.Visibility,
			MemoryType:         doc.MemoryType,
			ChunkIndex:         0,
			ContentText:        doc.ContentText,
			ContentHash:        doc.ContentHash,
			EmbeddingModel:     "qwen3-vl-embedding",
			EmbeddingDimension: 1024,
			QdrantPointID:      "33333333-3333-3333-3333-333333333333",
			IndexedAt:          &indexedAt,
		},
	}); err != nil {
		t.Fatalf("ReplaceDocumentChunks() error = %v", err)
	}
	rows, err = repo.ListDocumentsNeedingIndex(ctx, 10, "qwen3-vl-embedding", 1024)
	if err != nil {
		t.Fatalf("ListDocumentsNeedingIndex(indexed) error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("documents needing index after indexed = %+v, want empty", rows)
	}

	if err := db.Model(&entity.AIMemoryDocument{}).
		Where("id = ?", doc.ID).
		Updates(map[string]any{
			"content_text": "updated content",
			"updated_at":   time.Now().Add(2 * time.Hour),
		}).Error; err != nil {
		t.Fatalf("update document content: %v", err)
	}
	rows, err = repo.ListDocumentsNeedingIndex(ctx, 10, "qwen3-vl-embedding", 1024)
	if err != nil {
		t.Fatalf("ListDocumentsNeedingIndex(updated) error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != doc.ID {
		t.Fatalf("documents needing index after update = %+v, want %s", rows, doc.ID)
	}
}

func newAIMemoryRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.AIMemoryFact{},
		&entity.AIMemoryDocument{},
		&entity.AIMemoryDocumentChunk{},
		&entity.AIConversationSummary{},
	); err != nil {
		t.Fatalf("auto migrate ai memory models: %v", err)
	}
	return db
}

func mustUpsertFact(
	t *testing.T,
	repo interfaces.AIMemoryRepository,
	ctx context.Context,
	fact *entity.AIMemoryFact,
) {
	t.Helper()
	if err := repo.UpsertFact(ctx, fact); err != nil {
		t.Fatalf("UpsertFact() error = %v", err)
	}
}

func assertHasIndex(t *testing.T, db *gorm.DB, model any, name string) {
	t.Helper()
	if !db.Migrator().HasIndex(model, name) {
		t.Fatalf("index %s missing", name)
	}
}
