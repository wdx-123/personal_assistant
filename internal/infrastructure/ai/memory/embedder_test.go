package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestDashScopeEmbedderSendsExpectedRequest(t *testing.T) {
	var captured dashScopeEmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"output":{"embeddings":[{"index":0,"embedding":[0.1,0.2,0.3]}]}}`))
	}))
	defer server.Close()

	embedder := NewDashScopeEmbedder(EmbedderOptions{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "qwen3-vl-embedding",
		Dimension: 3,
	})
	result, err := embedder.Embed(context.Background(), aidomain.MemoryEmbeddingInput{Texts: []string{"hello"}})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if captured.Model != "qwen3-vl-embedding" || captured.Parameters.Dimension != 3 {
		t.Fatalf("captured request = %+v", captured)
	}
	if len(captured.Input.Contents) != 1 || captured.Input.Contents[0].Text != "hello" {
		t.Fatalf("captured contents = %+v", captured.Input.Contents)
	}
	if len(result.Vectors) != 1 || len(result.Vectors[0]) != 3 {
		t.Fatalf("vectors = %+v", result.Vectors)
	}
}

func TestDashScopeEmbedderRejectsDimensionMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"output":{"embeddings":[{"index":0,"embedding":[0.1,0.2]}]}}`))
	}))
	defer server.Close()

	embedder := NewDashScopeEmbedder(EmbedderOptions{
		APIKey:    "test-key",
		Endpoint:  server.URL,
		Model:     "qwen3-vl-embedding",
		Dimension: 3,
	})
	if _, err := embedder.Embed(context.Background(), aidomain.MemoryEmbeddingInput{Texts: []string{"hello"}}); err == nil {
		t.Fatal("Embed() error = nil, want dimension mismatch")
	}
}

func TestDashScopeEmbedderRequiresAPIKeyAndModel(t *testing.T) {
	embedder := NewDashScopeEmbedder(EmbedderOptions{Dimension: 3})
	if _, err := embedder.Embed(context.Background(), aidomain.MemoryEmbeddingInput{Texts: []string{"hello"}}); err == nil {
		t.Fatal("Embed() error = nil, want missing api key")
	}

	embedder = NewDashScopeEmbedder(EmbedderOptions{APIKey: "test-key", Dimension: 3})
	if _, err := embedder.Embed(context.Background(), aidomain.MemoryEmbeddingInput{Texts: []string{"hello"}}); err == nil {
		t.Fatal("Embed() error = nil, want missing model")
	}
}
