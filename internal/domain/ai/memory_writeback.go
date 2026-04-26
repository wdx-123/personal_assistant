package ai

import "context"

// MemoryExtractionInput is the stable input consumed by writeback extractors.
type MemoryExtractionInput struct {
	ConversationID string
	UserID         uint
	OrgID          *uint
	Principal      AIToolPrincipal

	UserMessage      Message
	AssistantMessage Message

	PreviousSummaryText string
}

// ConversationSummaryDraft is a technology-neutral summary proposal.
type ConversationSummaryDraft struct {
	ConversationID           string
	CompressedUntilMessageID string
	SummaryText              string
	KeyPointsJSON            string
	OpenLoopsJSON            string
	TokenEstimate            int
}

// MemoryExtractionResult groups all memory candidates extracted from one turn.
type MemoryExtractionResult struct {
	Summary   *ConversationSummaryDraft
	Facts     []MemoryFactCandidate
	Documents []MemoryDocumentCandidate
}

// MemoryExtractor extracts writeback candidates without knowing persistence.
type MemoryExtractor interface {
	Extract(ctx context.Context, input MemoryExtractionInput) (MemoryExtractionResult, error)
}
