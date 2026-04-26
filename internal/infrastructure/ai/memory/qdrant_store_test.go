package memory

import (
	"context"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"

	"github.com/qdrant/go-client/qdrant"
)

func TestQdrantVectorStoreDeletesAndUpsertsMemoryChunks(t *testing.T) {
	client := &fakeQdrantPointsClient{}
	store := NewQdrantVectorStore(client, VectorStoreOptions{CollectionName: "ai_memory_chunks"})
	userID := uint(7)

	err := store.DeleteDocumentChunks(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("DeleteDocumentChunks() error = %v", err)
	}
	if client.deleteRequest == nil || client.deleteRequest.CollectionName != "ai_memory_chunks" {
		t.Fatalf("delete request = %+v", client.deleteRequest)
	}
	filter := client.deleteRequest.GetPoints().GetFilter()
	if filter == nil || len(filter.Must) != 1 {
		t.Fatalf("delete filter = %+v", filter)
	}

	err = store.UpsertChunks(context.Background(), []aidomain.MemoryVectorChunk{
		{
			Chunk: aidomain.MemoryDocumentChunk{
				ID:            "chunk-1",
				DocumentID:    "doc-1",
				ScopeKey:      "self:user:7",
				ScopeType:     string(aidomain.MemoryScopeSelf),
				Visibility:    string(aidomain.MemoryVisibilitySelf),
				UserID:        &userID,
				MemoryType:    string(aidomain.MemoryTypeSemantic),
				Topic:         "design",
				SourceKind:    string(aidomain.MemorySourceModelInferred),
				SourceID:      "msg-1",
				ChunkIndex:    0,
				ContentHash:   "hash-1",
				QdrantPointID: "11111111-1111-1111-1111-111111111111",
			},
			Vector: []float32{0.1, 0.2, 0.3},
		},
	})
	if err != nil {
		t.Fatalf("UpsertChunks() error = %v", err)
	}
	if client.upsertRequest == nil || client.upsertRequest.CollectionName != "ai_memory_chunks" {
		t.Fatalf("upsert request = %+v", client.upsertRequest)
	}
	if len(client.upsertRequest.Points) != 1 {
		t.Fatalf("upsert points len = %d, want 1", len(client.upsertRequest.Points))
	}
	payload := client.upsertRequest.Points[0].Payload
	if payload["document_id"].GetStringValue() != "doc-1" ||
		payload["chunk_id"].GetStringValue() != "chunk-1" ||
		payload["scope_key"].GetStringValue() != "self:user:7" ||
		payload["user_id"].GetIntegerValue() != 7 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestQdrantVectorStoreSearchesMemoryChunksWithSelfFilter(t *testing.T) {
	client := &fakeQdrantPointsClient{
		queryResponse: []*qdrant.ScoredPoint{
			{
				Id:    qdrant.NewID("11111111-1111-1111-1111-111111111111"),
				Score: 0.87,
				Payload: qdrant.NewValueMap(map[string]any{
					"document_id": "doc-1",
					"chunk_id":    "chunk-1",
				}),
			},
		},
	}
	store := NewQdrantVectorStore(client, VectorStoreOptions{CollectionName: "ai_memory_chunks"})

	results, err := store.SearchChunks(context.Background(), aidomain.MemoryVectorSearchInput{
		Vector:     []float32{0.1, 0.2, 0.3},
		ScopeKey:   aidomain.BuildSelfMemoryScopeKey(7),
		Visibility: string(aidomain.MemoryVisibilitySelf),
		UserID:     7,
		Limit:      5,
		MinScore:   0.2,
	})
	if err != nil {
		t.Fatalf("SearchChunks() error = %v", err)
	}
	if client.queryRequest == nil || client.queryRequest.CollectionName != "ai_memory_chunks" {
		t.Fatalf("query request = %+v", client.queryRequest)
	}
	if client.queryRequest.GetLimit() != 5 {
		t.Fatalf("query limit = %d, want 5", client.queryRequest.GetLimit())
	}
	if client.queryRequest.GetScoreThreshold() != float32(0.2) {
		t.Fatalf("score threshold = %f, want 0.2", client.queryRequest.GetScoreThreshold())
	}
	must := client.queryRequest.GetFilter().GetMust()
	if len(must) != 3 {
		t.Fatalf("filter must len = %d, want 3: %+v", len(must), must)
	}
	assertQdrantMatchKeyword(t, must[0], "scope_key", "self:user:7")
	assertQdrantMatchKeyword(t, must[1], "visibility", "self")
	assertQdrantMatchInt(t, must[2], "user_id", 7)
	if len(results) != 1 ||
		results[0].QdrantPointID != "11111111-1111-1111-1111-111111111111" ||
		results[0].DocumentID != "doc-1" ||
		results[0].ChunkID != "chunk-1" ||
		results[0].Score < 0.86 ||
		results[0].Score > 0.88 {
		t.Fatalf("results = %+v", results)
	}
}

type fakeQdrantPointsClient struct {
	deleteRequest *qdrant.DeletePoints
	queryRequest  *qdrant.QueryPoints
	queryResponse []*qdrant.ScoredPoint
	upsertRequest *qdrant.UpsertPoints
}

func (f *fakeQdrantPointsClient) Delete(_ context.Context, request *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
	f.deleteRequest = request
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantPointsClient) Query(_ context.Context, request *qdrant.QueryPoints) ([]*qdrant.ScoredPoint, error) {
	f.queryRequest = request
	return f.queryResponse, nil
}

func (f *fakeQdrantPointsClient) Upsert(_ context.Context, request *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
	f.upsertRequest = request
	return &qdrant.UpdateResult{}, nil
}

func assertQdrantMatchKeyword(t *testing.T, condition *qdrant.Condition, key string, value string) {
	t.Helper()
	field := condition.GetField()
	if field == nil || field.GetKey() != key || field.GetMatch().GetKeyword() != value {
		t.Fatalf("condition = %+v, want %s=%s", condition, key, value)
	}
}

func assertQdrantMatchInt(t *testing.T, condition *qdrant.Condition, key string, value int64) {
	t.Helper()
	field := condition.GetField()
	if field == nil || field.GetKey() != key || field.GetMatch().GetInteger() != value {
		t.Fatalf("condition = %+v, want %s=%d", condition, key, value)
	}
}
