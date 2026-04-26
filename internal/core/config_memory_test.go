package core

import (
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"personal_assistant/global"
)

func TestInitConfigBindsAIMemoryAndQdrantCompatibility(t *testing.T) {
	viper.Reset()
	oldLog := global.Log
	oldConfig := global.Config
	global.Log = zap.NewNop()
	t.Cleanup(func() {
		viper.Reset()
		global.Log = oldLog
		global.Config = oldConfig
	})

	t.Setenv("AI_MEMORY_ENABLED", "true")
	t.Setenv("AI_MEMORY_RECALL_TOP_K", "9")
	t.Setenv("AI_MEMORY_RECALL_MAX_CHARS", "4096")
	t.Setenv("AI_MEMORY_RECALL_MIN_SCORE", "0.42")
	t.Setenv("AI_MEMORY_RAG_MAX_CHARS", "1024")
	t.Setenv("AI_MEMORY_EMBED_DIMENSION", "1024")
	t.Setenv("AI_MEMORY_INDEX_BATCH_SIZE", "11")
	t.Setenv("QDRANT_COLLECTION_NAME", "legacy-knowledge")
	t.Setenv("QDRANT_MEMORY_COLLECTION_NAME", "memory-chunks")

	InitConfig(t.TempDir())

	if global.Config == nil {
		t.Fatal("global.Config = nil")
	}
	if !global.Config.AI.Memory.Enabled {
		t.Fatal("AI.Memory.Enabled = false, want true")
	}
	if global.Config.AI.Memory.RecallTopK != 9 {
		t.Fatalf("AI.Memory.RecallTopK = %d, want 9", global.Config.AI.Memory.RecallTopK)
	}
	if global.Config.AI.Memory.RecallMaxChars != 4096 {
		t.Fatalf("AI.Memory.RecallMaxChars = %d, want 4096", global.Config.AI.Memory.RecallMaxChars)
	}
	if global.Config.AI.Memory.RecallMinScore != 0.42 {
		t.Fatalf("AI.Memory.RecallMinScore = %f, want 0.42", global.Config.AI.Memory.RecallMinScore)
	}
	if global.Config.AI.Memory.RAGMaxChars != 1024 {
		t.Fatalf("AI.Memory.RAGMaxChars = %d, want 1024", global.Config.AI.Memory.RAGMaxChars)
	}
	if global.Config.AI.Memory.SummaryRefreshEveryTurns != 10 {
		t.Fatalf(
			"AI.Memory.SummaryRefreshEveryTurns = %d, want default 10",
			global.Config.AI.Memory.SummaryRefreshEveryTurns,
		)
	}
	if global.Config.AI.Memory.EmbedModel != "qwen3-vl-embedding" {
		t.Fatalf("AI.Memory.EmbedModel = %q, want qwen3-vl-embedding", global.Config.AI.Memory.EmbedModel)
	}
	if global.Config.AI.Memory.EmbedDimension != 1024 {
		t.Fatalf("AI.Memory.EmbedDimension = %d, want 1024", global.Config.AI.Memory.EmbedDimension)
	}
	if global.Config.AI.Memory.IndexBatchSize != 11 {
		t.Fatalf("AI.Memory.IndexBatchSize = %d, want 11", global.Config.AI.Memory.IndexBatchSize)
	}
	if global.Config.Qdrant.CollectionName != "legacy-knowledge" {
		t.Fatalf("Qdrant.CollectionName = %q, want %q", global.Config.Qdrant.CollectionName, "legacy-knowledge")
	}
	if global.Config.Qdrant.KnowledgeCollectionName != "legacy-knowledge" {
		t.Fatalf(
			"Qdrant.KnowledgeCollectionName = %q, want %q",
			global.Config.Qdrant.KnowledgeCollectionName,
			"legacy-knowledge",
		)
	}
	if global.Config.Qdrant.MemoryCollectionName != "memory-chunks" {
		t.Fatalf(
			"Qdrant.MemoryCollectionName = %q, want %q",
			global.Config.Qdrant.MemoryCollectionName,
			"memory-chunks",
		)
	}
}
