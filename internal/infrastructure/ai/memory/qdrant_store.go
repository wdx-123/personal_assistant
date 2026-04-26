package memory

import (
	"context"
	"fmt"
	"strings"

	aidomain "personal_assistant/internal/domain/ai"

	"github.com/qdrant/go-client/qdrant"
)

// QdrantPointsClient 是 Qdrant client 在 memory index 阶段需要的最小能力。
type QdrantPointsClient interface {
	Delete(ctx context.Context, request *qdrant.DeletePoints) (*qdrant.UpdateResult, error)
	Upsert(ctx context.Context, request *qdrant.UpsertPoints) (*qdrant.UpdateResult, error)
}

// VectorStoreOptions 配置 Qdrant memory vector store。
type VectorStoreOptions struct {
	CollectionName string
}

// QdrantVectorStore 把 memory chunks 写入 Qdrant collection。
type QdrantVectorStore struct {
	client         QdrantPointsClient
	collectionName string
}

// NewQdrantVectorStore 创建 Qdrant vector store。
func NewQdrantVectorStore(client QdrantPointsClient, opts VectorStoreOptions) *QdrantVectorStore {
	return &QdrantVectorStore{
		client:         client,
		collectionName: strings.TrimSpace(opts.CollectionName),
	}
}

// DeleteDocumentChunks 删除指定 document 已存在的旧 points。
func (s *QdrantVectorStore) DeleteDocumentChunks(ctx context.Context, documentID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil
	}
	if s.collectionName == "" {
		return fmt.Errorf("qdrant memory collection name is required")
	}
	wait := true
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collectionName,
		Wait:           &wait,
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatchKeyword("document_id", documentID),
			},
		}),
	})
	return err
}

// UpsertChunks 写入最新 memory chunk vectors。
func (s *QdrantVectorStore) UpsertChunks(ctx context.Context, chunks []aidomain.MemoryVectorChunk) error {
	if s == nil || s.client == nil || len(chunks) == 0 {
		return nil
	}
	if s.collectionName == "" {
		return fmt.Errorf("qdrant memory collection name is required")
	}
	points := make([]*qdrant.PointStruct, 0, len(chunks))
	for _, item := range chunks {
		if len(item.Vector) == 0 || strings.TrimSpace(item.Chunk.QdrantPointID) == "" {
			continue
		}
		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewID(item.Chunk.QdrantPointID),
			Vectors: qdrant.NewVectorsDense(item.Vector),
			Payload: qdrant.NewValueMap(buildMemoryVectorPayload(item.Chunk)),
		})
	}
	if len(points) == 0 {
		return nil
	}
	wait := true
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Wait:           &wait,
		Points:         points,
	})
	return err
}

func buildMemoryVectorPayload(chunk aidomain.MemoryDocumentChunk) map[string]any {
	payload := map[string]any{
		"document_id":  chunk.DocumentID,
		"chunk_id":     chunk.ID,
		"scope_key":    chunk.ScopeKey,
		"scope_type":   chunk.ScopeType,
		"visibility":   chunk.Visibility,
		"memory_type":  chunk.MemoryType,
		"topic":        chunk.Topic,
		"source_kind":  chunk.SourceKind,
		"source_id":    chunk.SourceID,
		"chunk_index":  chunk.ChunkIndex,
		"content_hash": chunk.ContentHash,
	}
	if chunk.UserID != nil {
		payload["user_id"] = int64(*chunk.UserID)
	}
	if chunk.OrgID != nil {
		payload["org_id"] = int64(*chunk.OrgID)
	}
	return payload
}
