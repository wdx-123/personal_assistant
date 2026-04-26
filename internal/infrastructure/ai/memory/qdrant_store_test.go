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

type fakeQdrantPointsClient struct {
	deleteRequest *qdrant.DeletePoints
	upsertRequest *qdrant.UpsertPoints
}

func (f *fakeQdrantPointsClient) Delete(_ context.Context, request *qdrant.DeletePoints) (*qdrant.UpdateResult, error) {
	f.deleteRequest = request
	return &qdrant.UpdateResult{}, nil
}

func (f *fakeQdrantPointsClient) Upsert(_ context.Context, request *qdrant.UpsertPoints) (*qdrant.UpdateResult, error) {
	f.upsertRequest = request
	return &qdrant.UpdateResult{}, nil
}
