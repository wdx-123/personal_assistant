package memory

import (
	"context"
	"strings"
	"testing"

	aidomain "personal_assistant/internal/domain/ai"
)

func TestParagraphChunkerShortText(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 100, OverlapChars: 10})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:         "doc-1",
		ScopeKey:   "self:user:1",
		ScopeType:  string(aidomain.MemoryScopeSelf),
		Visibility: string(aidomain.MemoryVisibilitySelf),
		Content:    "短文本内容",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks len = %d, want 1", len(chunks))
	}
	if chunks[0].ContentText != "短文本内容" || chunks[0].QdrantPointID == "" || chunks[0].ContentHash == "" {
		t.Fatalf("unexpected chunk: %+v", chunks[0])
	}
}

func TestParagraphChunkerLongTextWithOverlap(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 20, OverlapChars: 5})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-2",
		Content: strings.Repeat("a", 45),
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks len = %d, want 3: %+v", len(chunks), chunks)
	}
	if !strings.HasPrefix(chunks[1].ContentText, strings.Repeat("a", 5)) {
		t.Fatalf("chunk overlap missing: %+v", chunks[1])
	}
}

func TestParagraphChunkerEmptyText(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{ID: "doc-empty", Content: "   "})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("chunks len = %d, want 0", len(chunks))
	}
}
