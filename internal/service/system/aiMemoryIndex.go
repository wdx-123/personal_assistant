package system

import (
	"context"
	"fmt"
	"time"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"

	"go.uber.org/zap"
)

// IndexDocuments 为指定长期记忆 documents 建立 Qdrant 向量索引。
func (s *AIMemoryService) IndexDocuments(ctx context.Context, documentIDs []string) error {
	if !s.memoryIndexingReady() || len(documentIDs) == 0 {
		return nil
	}
	docs, err := s.repo.ListDocumentsByIDs(ctx, documentIDs)
	if err != nil {
		return err
	}
	return s.indexDocumentRows(ctx, docs)
}

// IndexPendingDocuments 扫描并补偿尚未完成索引的 documents。
func (s *AIMemoryService) IndexPendingDocuments(ctx context.Context, limit int) error {
	if !s.memoryIndexingReady() {
		return nil
	}
	if limit <= 0 {
		limit = aiMemoryIndexBatchSize()
	}
	docs, err := s.repo.ListDocumentsNeedingIndex(ctx, limit, aiMemoryEmbedModel(), aiMemoryEmbedDimension())
	if err != nil {
		return err
	}
	return s.indexDocumentRows(ctx, docs)
}

func (s *AIMemoryService) memoryIndexingReady() bool {
	return aiMemoryEnabled() &&
		aiMemoryLongTermEnabled() &&
		s != nil &&
		s.repo != nil &&
		s.chunker != nil &&
		s.embedder != nil &&
		s.vectorStore != nil
}

func (s *AIMemoryService) indexDocumentRows(ctx context.Context, docs []*entity.AIMemoryDocument) error {
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if err := s.indexOneDocument(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (s *AIMemoryService) indexOneDocument(ctx context.Context, doc *entity.AIMemoryDocument) error {
	chunks, err := s.chunker.Chunk(ctx, memoryDocumentEntityToIndex(doc))
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return s.repo.ReplaceDocumentChunks(ctx, doc.ID, nil)
	}
	texts := make([]string, 0, len(chunks))
	for i := range chunks {
		chunks[i].EmbeddingModel = aiMemoryEmbedModel()
		chunks[i].EmbeddingDimension = aiMemoryEmbedDimension()
		texts = append(texts, chunks[i].ContentText)
	}
	embeddings, err := s.embedder.Embed(ctx, aidomain.MemoryEmbeddingInput{Texts: texts})
	if err != nil {
		return err
	}
	if len(embeddings.Vectors) != len(chunks) {
		return fmt.Errorf("memory embedding count = %d, want %d", len(embeddings.Vectors), len(chunks))
	}

	vectorChunks := make([]aidomain.MemoryVectorChunk, 0, len(chunks))
	entities := make([]*entity.AIMemoryDocumentChunk, 0, len(chunks))
	indexedAt := time.Now()
	for i, chunk := range chunks {
		vectorChunks = append(vectorChunks, aidomain.MemoryVectorChunk{
			Chunk:  chunk,
			Vector: embeddings.Vectors[i],
		})
		entities = append(entities, memoryChunkToEntity(chunk, indexedAt))
	}
	if err := s.vectorStore.DeleteDocumentChunks(ctx, doc.ID); err != nil {
		return err
	}
	if err := s.vectorStore.UpsertChunks(ctx, vectorChunks); err != nil {
		return err
	}
	return s.repo.ReplaceDocumentChunks(ctx, doc.ID, entities)
}

func (s *AIMemoryService) triggerDocumentIndex(ctx context.Context, docs []*entity.AIMemoryDocument) {
	if !s.memoryIndexingReady() || len(docs) == 0 {
		return
	}
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc != nil && doc.ID != "" {
			ids = append(ids, doc.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	go func() {
		runCtx, cancel := context.WithTimeout(context.Background(), time.Duration(aiMemoryIndexTimeoutSeconds())*time.Second)
		defer cancel()
		if err := s.IndexDocuments(runCtx, ids); err != nil && global.Log != nil {
			global.Log.Warn("AI memory document index failed", zap.Error(err))
		}
	}()
}

func memoryDocumentEntityToIndex(doc *entity.AIMemoryDocument) aidomain.MemoryDocumentForIndex {
	if doc == nil {
		return aidomain.MemoryDocumentForIndex{}
	}
	return aidomain.MemoryDocumentForIndex{
		ID:         doc.ID,
		ScopeKey:   doc.ScopeKey,
		ScopeType:  doc.ScopeType,
		Visibility: doc.Visibility,
		UserID:     cloneMemoryUintPtr(doc.UserID),
		OrgID:      cloneMemoryUintPtr(doc.OrgID),
		MemoryType: doc.MemoryType,
		Topic:      doc.Topic,
		Title:      doc.Title,
		Summary:    doc.Summary,
		Content:    doc.ContentText,
		SourceKind: doc.SourceKind,
		SourceID:   doc.SourceID,
	}
}

func memoryChunkToEntity(chunk aidomain.MemoryDocumentChunk, indexedAt time.Time) *entity.AIMemoryDocumentChunk {
	return &entity.AIMemoryDocumentChunk{
		ID:                 chunk.ID,
		DocumentID:         chunk.DocumentID,
		ScopeKey:           chunk.ScopeKey,
		ScopeType:          chunk.ScopeType,
		Visibility:         chunk.Visibility,
		UserID:             cloneMemoryUintPtr(chunk.UserID),
		OrgID:              cloneMemoryUintPtr(chunk.OrgID),
		MemoryType:         chunk.MemoryType,
		Topic:              chunk.Topic,
		ChunkIndex:         chunk.ChunkIndex,
		ContentText:        chunk.ContentText,
		ContentHash:        chunk.ContentHash,
		TokenEstimate:      chunk.TokenEstimate,
		EmbeddingModel:     chunk.EmbeddingModel,
		EmbeddingDimension: chunk.EmbeddingDimension,
		QdrantPointID:      chunk.QdrantPointID,
		IndexedAt:          &indexedAt,
		CreatedAt:          indexedAt,
		UpdatedAt:          indexedAt,
	}
}
