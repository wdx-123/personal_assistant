package system

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	aimemory "personal_assistant/internal/infrastructure/ai/memory"
	"personal_assistant/internal/model/config"
	"personal_assistant/internal/model/entity"
)

func TestAIMemoryIndexDocumentsPersistsChunksAndUpsertsVectors(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	service.chunker = aimemory.NewParagraphChunker(aimemory.ChunkerOptions{MaxChars: 100, OverlapChars: 0})
	service.embedder = &fakeMemoryEmbedder{vectors: [][]float32{{0.1, 0.2, 0.3}}}
	vectorStore := &fakeMemoryVectorStore{}
	service.vectorStore = vectorStore
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableLongTermMemory: true,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	doc := createMemoryIndexDocument(t, service, "doc-index-ok", "可索引内容")
	if err := service.IndexDocuments(context.Background(), []string{doc.ID}); err != nil {
		t.Fatalf("IndexDocuments() error = %v", err)
	}
	chunks, err := service.repo.ListDocumentChunks(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("ListDocumentChunks() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks len = %d, want 1", len(chunks))
	}
	if chunks[0].EmbeddingModel != "qwen3-vl-embedding" || chunks[0].EmbeddingDimension != 3 {
		t.Fatalf("chunk embedding metadata = %+v", chunks[0])
	}
	if vectorStore.deletedDocumentID != doc.ID || len(vectorStore.upserted) != 1 {
		t.Fatalf("vector store deleted=%q upserted=%+v", vectorStore.deletedDocumentID, vectorStore.upserted)
	}
}

func TestAIMemoryIndexDocumentsDoesNotPersistChunksWhenEmbeddingFails(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	service.chunker = aimemory.NewParagraphChunker(aimemory.ChunkerOptions{MaxChars: 100, OverlapChars: 0})
	service.embedder = &fakeMemoryEmbedder{err: stderrors.New("embedding failed")}
	service.vectorStore = &fakeMemoryVectorStore{}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableLongTermMemory: true,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	doc := createMemoryIndexDocument(t, service, "doc-index-embed-fail", "可索引内容")
	if err := service.IndexDocuments(context.Background(), []string{doc.ID}); err == nil {
		t.Fatal("IndexDocuments() error = nil, want embedding error")
	}
	chunks, err := service.repo.ListDocumentChunks(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("ListDocumentChunks() error = %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks len = %d, want 0", len(chunks))
	}
}

func TestAIMemoryIndexDocumentsDoesNotPersistChunksWhenVectorStoreFails(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, nil)
	service.chunker = aimemory.NewParagraphChunker(aimemory.ChunkerOptions{MaxChars: 100, OverlapChars: 0})
	service.embedder = &fakeMemoryEmbedder{vectors: [][]float32{{0.1, 0.2, 0.3}}}
	service.vectorStore = &fakeMemoryVectorStore{err: stderrors.New("qdrant failed")}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableLongTermMemory: true,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	doc := createMemoryIndexDocument(t, service, "doc-index-vector-fail", "可索引内容")
	if err := service.IndexDocuments(context.Background(), []string{doc.ID}); err == nil {
		t.Fatal("IndexDocuments() error = nil, want vector store error")
	}
	chunks, err := service.repo.ListDocumentChunks(context.Background(), doc.ID)
	if err != nil {
		t.Fatalf("ListDocumentChunks() error = %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks len = %d, want 0", len(chunks))
	}
}

func TestAIMemoryWritebackIndexFailureDoesNotFailTurnCompleted(t *testing.T) {
	db := newAIMemoryWritebackTestDB(t)
	service := newAIMemoryWritebackTestService(db, aimemory.NewRuleExtractor(aimemory.Options{DocumentMinRunes: 40}))
	service.chunker = aimemory.NewParagraphChunker(aimemory.ChunkerOptions{MaxChars: 100, OverlapChars: 0})
	embedder := &fakeMemoryEmbedder{err: stderrors.New("embedding failed"), called: make(chan struct{}, 1)}
	service.embedder = embedder
	service.vectorStore = &fakeMemoryVectorStore{}
	restore := setAIMemoryTestConfig(t, config.AIMemory{
		Enabled:              true,
		EnableEntityMemory:   true,
		EnableLongTermMemory: true,
		EmbedModel:           "qwen3-vl-embedding",
		EmbedDimension:       3,
	})
	defer restore()

	createAIWritebackMessagesWithContent(
		t,
		db,
		"conv-index-fail",
		"msg-user-index-fail",
		"msg-ai-index-fail",
		"请给我一个 RAG 切分入库的实现方案",
		repeatMemoryText("实现方案包括切块、embedding、写入 Qdrant 和保存 chunk 映射。", 6),
		aiMessageStatusSuccess,
	)
	err := service.OnTurnCompleted(context.Background(), aiMemoryWritebackInput{
		ConversationID:     "conv-index-fail",
		UserID:             22,
		UserMessageID:      "msg-user-index-fail",
		AssistantMessageID: "msg-ai-index-fail",
		Principal:          aidomain.AIToolPrincipal{UserID: 22},
	})
	if err != nil {
		t.Fatalf("OnTurnCompleted() error = %v", err)
	}
	assertAIMemoryWritebackCount(t, db, &entity.AIMemoryDocument{}, 1)
	select {
	case <-embedder.called:
	case <-time.After(time.Second):
		t.Fatal("embedding was not called by async indexer")
	}
}

func createMemoryIndexDocument(t *testing.T, service *AIMemoryService, id string, content string) *entity.AIMemoryDocument {
	t.Helper()
	userID := uint(21)
	doc := &entity.AIMemoryDocument{
		ID:          id,
		ScopeKey:    aidomain.BuildSelfMemoryScopeKey(userID),
		ScopeType:   string(aidomain.MemoryScopeSelf),
		Visibility:  string(aidomain.MemoryVisibilitySelf),
		UserID:      &userID,
		MemoryType:  string(aidomain.MemoryTypeSemantic),
		Topic:       "design",
		Title:       "design",
		Summary:     "summary",
		ContentText: content,
		SourceKind:  string(aidomain.MemorySourceModelInferred),
		SourceID:    "msg-index",
	}
	if err := service.repo.BatchUpsertDocuments(context.Background(), []*entity.AIMemoryDocument{doc}); err != nil {
		t.Fatalf("BatchUpsertDocuments() error = %v", err)
	}
	return doc
}

type fakeMemoryEmbedder struct {
	vectors [][]float32
	err     error
	called  chan struct{}
}

func (f *fakeMemoryEmbedder) Embed(
	_ context.Context,
	input aidomain.MemoryEmbeddingInput,
) (aidomain.MemoryEmbeddingResult, error) {
	if f.called != nil {
		select {
		case f.called <- struct{}{}:
		default:
		}
	}
	if f.err != nil {
		return aidomain.MemoryEmbeddingResult{}, f.err
	}
	if len(f.vectors) > 0 {
		return aidomain.MemoryEmbeddingResult{Vectors: f.vectors}, nil
	}
	vectors := make([][]float32, len(input.Texts))
	for i := range vectors {
		vectors[i] = []float32{0.1, 0.2, 0.3}
	}
	return aidomain.MemoryEmbeddingResult{Vectors: vectors}, nil
}

type fakeMemoryVectorStore struct {
	deletedDocumentID string
	upserted          []aidomain.MemoryVectorChunk
	searchInput       aidomain.MemoryVectorSearchInput
	searchResults     []aidomain.MemoryVectorSearchResult
	err               error
	searchErr         error
}

func (f *fakeMemoryVectorStore) DeleteDocumentChunks(_ context.Context, documentID string) error {
	f.deletedDocumentID = documentID
	return f.err
}

func (f *fakeMemoryVectorStore) UpsertChunks(_ context.Context, chunks []aidomain.MemoryVectorChunk) error {
	f.upserted = append([]aidomain.MemoryVectorChunk(nil), chunks...)
	return f.err
}

func (f *fakeMemoryVectorStore) SearchChunks(
	_ context.Context,
	input aidomain.MemoryVectorSearchInput,
) ([]aidomain.MemoryVectorSearchResult, error) {
	f.searchInput = input
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.searchResults, nil
}
