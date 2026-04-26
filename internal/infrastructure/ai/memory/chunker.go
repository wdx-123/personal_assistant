package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	aidomain "personal_assistant/internal/domain/ai"
)

const (
	defaultChunkMaxChars     = 1200
	defaultChunkOverlapChars = 150
)

// ChunkerOptions 配置记忆文档切分策略。
type ChunkerOptions struct {
	MaxChars     int
	OverlapChars int
}

// ParagraphChunker 按段落优先切分记忆文档，必要时回退到字符窗口。
type ParagraphChunker struct {
	maxChars     int
	overlapChars int
}

// NewParagraphChunker 创建 paragraph-aware chunker。
func NewParagraphChunker(opts ChunkerOptions) *ParagraphChunker {
	if opts.MaxChars <= 0 {
		opts.MaxChars = defaultChunkMaxChars
	}
	if opts.OverlapChars < 0 {
		opts.OverlapChars = 0
	}
	if opts.OverlapChars >= opts.MaxChars {
		opts.OverlapChars = opts.MaxChars / 4
	}
	return &ParagraphChunker{
		maxChars:     opts.MaxChars,
		overlapChars: opts.OverlapChars,
	}
}

// Chunk 把文档正文拆成稳定 chunks。
func (c *ParagraphChunker) Chunk(
	ctx context.Context,
	doc aidomain.MemoryDocumentForIndex,
) ([]aidomain.MemoryDocumentChunk, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	content := normalizeChunkText(doc.Content)
	if content == "" {
		return nil, nil
	}
	parts := c.splitText(content)
	chunks := make([]aidomain.MemoryDocumentChunk, 0, len(parts))
	for idx, part := range parts {
		hash := aidomain.BuildMemoryDocumentContentHash(part)
		chunkID := buildMemoryChunkID(doc.ID, idx, hash)
		pointID := buildMemoryChunkPointID(doc.ID, idx, hash)
		chunks = append(chunks, aidomain.MemoryDocumentChunk{
			ID:            chunkID,
			DocumentID:    doc.ID,
			ScopeKey:      doc.ScopeKey,
			ScopeType:     doc.ScopeType,
			Visibility:    doc.Visibility,
			UserID:        cloneUintPtr(doc.UserID),
			OrgID:         cloneUintPtr(doc.OrgID),
			MemoryType:    doc.MemoryType,
			Topic:         doc.Topic,
			SourceKind:    doc.SourceKind,
			SourceID:      doc.SourceID,
			ChunkIndex:    idx,
			ContentText:   part,
			ContentHash:   hash,
			TokenEstimate: estimateChunkTokens(part),
			QdrantPointID: pointID,
		})
	}
	return chunks, nil
}

func (c *ParagraphChunker) splitText(content string) []string {
	paragraphs := splitParagraphs(content)
	chunks := make([]string, 0, len(paragraphs))
	current := ""
	for _, paragraph := range paragraphs {
		if utf8.RuneCountInString(paragraph) > c.maxChars {
			if current != "" {
				chunks = append(chunks, current)
				current = c.chunkOverlap(current)
			}
			for _, part := range splitByRuneWindow(paragraph, c.maxChars, c.overlapChars) {
				if part != "" {
					chunks = append(chunks, part)
				}
			}
			current = ""
			continue
		}
		candidate := paragraph
		if current != "" {
			candidate = current + "\n\n" + paragraph
		}
		if utf8.RuneCountInString(candidate) <= c.maxChars {
			current = candidate
			continue
		}
		if current != "" {
			chunks = append(chunks, current)
			current = strings.TrimSpace(c.chunkOverlap(current) + "\n\n" + paragraph)
		} else {
			current = paragraph
		}
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func (c *ParagraphChunker) chunkOverlap(value string) string {
	if c.overlapChars <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= c.overlapChars {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(string(runes[len(runes)-c.overlapChars:]))
}

func splitByRuneWindow(value string, maxChars int, overlap int) []string {
	runes := []rune(value)
	if len(runes) <= maxChars {
		return []string{strings.TrimSpace(value)}
	}
	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}
	chunks := make([]string, 0, (len(runes)/step)+1)
	for start := 0; start < len(runes); start += step {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
	}
	return chunks
}

func splitParagraphs(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	raw := strings.Split(value, "\n\n")
	paragraphs := make([]string, 0, len(raw))
	for _, item := range raw {
		item = normalizeChunkText(item)
		if item != "" {
			paragraphs = append(paragraphs, item)
		}
	}
	if len(paragraphs) == 0 {
		return []string{normalizeChunkText(value)}
	}
	return paragraphs
}

func normalizeChunkText(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func estimateChunkTokens(value string) int {
	runes := utf8.RuneCountInString(value)
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func buildMemoryChunkID(documentID string, chunkIndex int, contentHash string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", documentID, chunkIndex, contentHash)))
	return "mem_chunk_" + hex.EncodeToString(sum[:])[:32]
}

func buildMemoryChunkPointID(documentID string, chunkIndex int, contentHash string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", documentID, chunkIndex, contentHash)))
	hexValue := hex.EncodeToString(sum[:])[:32]
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:32],
	)
}

func cloneUintPtr(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
