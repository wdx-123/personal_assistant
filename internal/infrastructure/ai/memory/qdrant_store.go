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
	Query(ctx context.Context, request *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error)
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

// SearchChunks 按 query vector 检索 memory chunk 候选。
func (s *QdrantVectorStore) SearchChunks(
	ctx context.Context,
	input aidomain.MemoryVectorSearchInput,
) ([]aidomain.MemoryVectorSearchResult, error) {
	if s == nil || s.client == nil || len(input.Vector) == 0 {
		return nil, nil
	}
	if s.collectionName == "" {
		return nil, fmt.Errorf("qdrant memory collection name is required")
	}
	limit := uint64(input.Limit)
	if limit == 0 {
		return nil, nil
	}
	scoreThreshold := float32(input.MinScore)
	request := &qdrant.QueryPoints{
		CollectionName: s.collectionName,
		Query:          qdrant.NewQueryDense(input.Vector),
		Filter:         buildMemoryVectorSearchFilter(input),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayloadInclude("document_id", "chunk_id", "scope_key", "visibility", "user_id"),
	}
	if input.MinScore > 0 {
		request.ScoreThreshold = &scoreThreshold
	}
	points, err := s.client.Query(ctx, request)
	if err != nil {
		return nil, err
	}
	results := make([]aidomain.MemoryVectorSearchResult, 0, len(points))
	for _, point := range points {
		if point == nil {
			continue
		}
		pointID := memoryQdrantPointID(point.GetId())
		if pointID == "" {
			continue
		}
		payload := point.GetPayload()
		results = append(results, aidomain.MemoryVectorSearchResult{
			QdrantPointID: pointID,
			ChunkID:       memoryQdrantPayloadString(payload, "chunk_id"),
			DocumentID:    memoryQdrantPayloadString(payload, "document_id"),
			Score:         float64(point.GetScore()),
		})
	}
	return results, nil
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

func buildMemoryVectorSearchFilter(input aidomain.MemoryVectorSearchInput) *qdrant.Filter {
	must := make([]*qdrant.Condition, 0, 3)
	if scopeKey := strings.TrimSpace(input.ScopeKey); scopeKey != "" {
		must = append(must, qdrant.NewMatchKeyword("scope_key", scopeKey))
	}
	if visibility := strings.TrimSpace(input.Visibility); visibility != "" {
		must = append(must, qdrant.NewMatchKeyword("visibility", visibility))
	}
	if input.UserID > 0 {
		must = append(must, qdrant.NewMatchInt("user_id", int64(input.UserID)))
	}
	if len(must) == 0 {
		return nil
	}
	return &qdrant.Filter{Must: must}
}

func memoryQdrantPointID(pointID *qdrant.PointId) string {
	if pointID == nil {
		return ""
	}
	if uuid := strings.TrimSpace(pointID.GetUuid()); uuid != "" {
		return uuid
	}
	if num := pointID.GetNum(); num > 0 {
		return fmt.Sprintf("%d", num)
	}
	return ""
}

func memoryQdrantPayloadString(payload map[string]*qdrant.Value, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value := payload[key]
	if value == nil {
		return ""
	}
	return strings.TrimSpace(value.GetStringValue())
}
