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

func TestParagraphChunkerProsePrefersStrongSentenceBoundary(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 18, OverlapChars: 0})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-prose-strong",
		Content: "第一句很短，包含逗号。第二句也很短。第三句继续。",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	if chunks[0].ContentText != "第一句很短，包含逗号。" {
		t.Fatalf("first chunk = %q, want strong sentence split", chunks[0].ContentText)
	}
}

func TestParagraphChunkerFallsBackToSoftBoundaryForLongSentence(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 12, OverlapChars: 0})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-prose-soft",
		Content: "这是一个很长的句子，需要在逗号这里切开，因为整句放不下。",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	if chunks[0].ContentText != "这是一个很长的句子，" {
		t.Fatalf("first chunk = %q, want soft boundary split", chunks[0].ContentText)
	}
}

func TestParagraphChunkerKeepsCodeFenceMultiline(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 40, OverlapChars: 0})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-code",
		Content: "```go\nfunc alpha() {\n    println(\"a\")\n}\n\nfunc beta() {\n    println(\"b\")\n}\n```",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	if !strings.Contains(chunks[0].ContentText, "```go\nfunc alpha()") {
		t.Fatalf("first code chunk lost multiline code fence:\n%s", chunks[0].ContentText)
	}
	if !strings.Contains(chunks[1].ContentText, "```go") {
		t.Fatalf("second code chunk should repeat fence:\n%s", chunks[1].ContentText)
	}
}

func TestParagraphChunkerRepeatsTableHeaderWhenSplit(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 48, OverlapChars: 0})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-table",
		Content: "| 列1 | 列2 |\n| --- | --- |\n| 行1 | 数据1 |\n| 行2 | 数据2 |\n| 行3 | 数据3 |",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	for _, chunk := range chunks {
		if !strings.Contains(chunk.ContentText, "| 列1 | 列2 |") || !strings.Contains(chunk.ContentText, "| --- | --- |") {
			t.Fatalf("table chunk missing repeated header:\n%s", chunk.ContentText)
		}
	}
}

func TestParagraphChunkerKeepsListItemsAtomic(t *testing.T) {
	chunker := NewParagraphChunker(ChunkerOptions{MaxChars: 18, OverlapChars: 0})
	chunks, err := chunker.Chunk(context.Background(), aidomain.MemoryDocumentForIndex{
		ID:      "doc-list",
		Content: "- 第一项说明\n- 第二项说明\n- 第三项说明",
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks len = %d, want at least 2", len(chunks))
	}
	for _, chunk := range chunks {
		lines := strings.Split(chunk.ContentText, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "- ") {
				t.Fatalf("list item was split without bullet marker: %q", line)
			}
		}
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
