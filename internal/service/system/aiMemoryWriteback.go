package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

type aiMemoryWritebackSnapshot struct {
	UserMessage      aidomain.Message
	AssistantMessage aidomain.Message
	PreviousSummary  *entity.AIConversationSummary
}

// OnTurnCompleted extracts and persists memory candidates after a successful AI turn.
func (s *AIMemoryService) OnTurnCompleted(ctx context.Context, input aiMemoryWritebackInput) error {
	if !aiMemoryEnabled() || s == nil || s.repo == nil || s.aiRepo == nil || s.extractor == nil {
		return nil
	}
	if strings.TrimSpace(input.ConversationID) == "" ||
		strings.TrimSpace(input.UserMessageID) == "" ||
		strings.TrimSpace(input.AssistantMessageID) == "" ||
		input.UserID == 0 {
		return nil
	}

	snapshot, err := s.buildWritebackSnapshot(ctx, input)
	if err != nil {
		return err
	}
	if snapshot == nil || strings.TrimSpace(snapshot.AssistantMessage.Content) == "" {
		return nil
	}

	extracted, err := s.extractor.Extract(ctx, aidomain.MemoryExtractionInput{
		ConversationID:      input.ConversationID,
		UserID:              input.UserID,
		OrgID:               input.OrgID,
		Principal:           normalizeMemoryPrincipal(input),
		UserMessage:         snapshot.UserMessage,
		AssistantMessage:    snapshot.AssistantMessage,
		PreviousSummaryText: previousSummaryText(snapshot.PreviousSummary),
	})
	if err != nil {
		return err
	}

	access := s.buildMemoryAccessContext(input)
	if err := s.applyConversationSummary(ctx, input, extracted.Summary); err != nil {
		return err
	}
	if err := s.applyFactCandidates(ctx, extracted.Facts, access); err != nil {
		return err
	}
	return s.applyDocumentCandidates(ctx, extracted.Documents, access)
}

func (s *AIMemoryService) buildWritebackSnapshot(
	ctx context.Context,
	input aiMemoryWritebackInput,
) (*aiMemoryWritebackSnapshot, error) {
	messages, err := s.aiRepo.ListMessagesByConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	var userMessage *entity.AIMessage
	var assistantMessage *entity.AIMessage
	for _, item := range messages {
		if item == nil {
			continue
		}
		if item.ID == input.UserMessageID {
			userMessage = item
		}
		if item.ID == input.AssistantMessageID {
			assistantMessage = item
		}
	}
	if userMessage == nil || assistantMessage == nil || assistantMessage.Status != aiMessageStatusSuccess {
		return nil, nil
	}

	scopeKey := aidomain.BuildConversationMemoryScopeKey(input.UserID, input.OrgID)
	previous, err := s.repo.GetConversationSummary(ctx, aidomain.MemoryConversationSummaryQuery{
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		OrgID:          input.OrgID,
		ScopeKey:       scopeKey,
	})
	if err != nil {
		return nil, err
	}

	return &aiMemoryWritebackSnapshot{
		UserMessage:      aiEntityMessageToDomain(userMessage),
		AssistantMessage: aiEntityMessageToDomain(assistantMessage),
		PreviousSummary:  previous,
	}, nil
}

func (s *AIMemoryService) applyConversationSummary(
	ctx context.Context,
	input aiMemoryWritebackInput,
	draft *aidomain.ConversationSummaryDraft,
) error {
	if draft == nil || strings.TrimSpace(draft.SummaryText) == "" {
		return nil
	}
	now := time.Now()
	scopeKey := aidomain.BuildConversationMemoryScopeKey(input.UserID, input.OrgID)
	return s.repo.UpsertConversationSummary(ctx, &entity.AIConversationSummary{
		ConversationID:           input.ConversationID,
		UserID:                   input.UserID,
		OrgID:                    cloneMemoryUintPtr(input.OrgID),
		ScopeKey:                 scopeKey,
		CompressedUntilMessageID: draft.CompressedUntilMessageID,
		SummaryText:              draft.SummaryText,
		KeyPointsJSON:            defaultMemoryJSONList(draft.KeyPointsJSON),
		OpenLoopsJSON:            defaultMemoryJSONList(draft.OpenLoopsJSON),
		TokenEstimate:            draft.TokenEstimate,
		CreatedAt:                now,
		UpdatedAt:                now,
	})
}

func (s *AIMemoryService) applyFactCandidates(
	ctx context.Context,
	candidates []aidomain.MemoryFactCandidate,
	access aidomain.MemoryAccessContext,
) error {
	if !aiMemoryEntityEnabled() || len(candidates) == 0 {
		return nil
	}
	for _, candidate := range candidates {
		if candidate.ScopeType != aidomain.MemoryScopeSelf {
			continue
		}
		if decision := s.policy.ShouldStoreFact(candidate, access); !decision.Allowed {
			continue
		}
		scopeDecision := s.policy.ResolveScope(aidomain.MemoryScopeInput{
			ScopeType: candidate.ScopeType,
			UserID:    candidate.UserID,
			OrgID:     candidate.OrgID,
		}, access)
		if !scopeDecision.Allowed {
			continue
		}
		visibilityDecision := s.policy.ResolveVisibility(scopeDecision, candidate.SourceKind)
		if !visibilityDecision.Allowed {
			continue
		}
		shouldUpsert, err := s.shouldUpsertFact(ctx, candidate, scopeDecision, visibilityDecision)
		if err != nil {
			return err
		}
		if !shouldUpsert {
			continue
		}
		ttl := s.policy.ResolveTTL(candidate.Namespace, "")
		fact := buildMemoryFactEntity(candidate, scopeDecision, visibilityDecision, ttl.ExpiresAt)
		if err := s.repo.UpsertFact(ctx, fact); err != nil {
			return err
		}
	}
	return nil
}

func (s *AIMemoryService) shouldUpsertFact(
	ctx context.Context,
	candidate aidomain.MemoryFactCandidate,
	scopeDecision aidomain.MemoryScopeDecision,
	visibilityDecision aidomain.MemoryVisibilityDecision,
) (bool, error) {
	rows, err := s.repo.ListFacts(ctx, aidomain.MemoryFactQuery{
		ScopeKeys:           []string{scopeDecision.ScopeKey},
		AllowedVisibilities: []aidomain.MemoryVisibility{visibilityDecision.Visibility},
		Namespace:           candidate.Namespace,
		FactKeys:            []string{candidate.FactKey},
		Limit:               1,
	})
	if err != nil || len(rows) == 0 || rows[0] == nil {
		return err == nil, err
	}
	decision := s.policy.ShouldOverrideFact(
		aidomain.MemoryFactVersion{
			ValueJSON:  rows[0].FactValueJSON,
			SourceKind: aidomain.MemorySourceKind(rows[0].SourceKind),
		},
		aidomain.MemoryFactVersion{
			ValueJSON:  candidate.FactValueJSON,
			SourceKind: candidate.SourceKind,
		},
		scopeDecision.ScopeType,
		candidate.Namespace,
	)
	return decision.Allowed, nil
}

func (s *AIMemoryService) applyDocumentCandidates(
	ctx context.Context,
	candidates []aidomain.MemoryDocumentCandidate,
	access aidomain.MemoryAccessContext,
) error {
	if !aiMemoryLongTermEnabled() || len(candidates) == 0 {
		return nil
	}
	docs := make([]*entity.AIMemoryDocument, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ScopeType != aidomain.MemoryScopeSelf {
			continue
		}
		decision := s.policy.ShouldStoreDocument(candidate, access)
		if !decision.Allowed {
			continue
		}
		scopeDecision := s.policy.ResolveScope(aidomain.MemoryScopeInput{
			ScopeType: candidate.ScopeType,
			UserID:    candidate.UserID,
			OrgID:     candidate.OrgID,
		}, access)
		if !scopeDecision.Allowed {
			continue
		}
		visibilityDecision := s.policy.ResolveVisibility(scopeDecision, candidate.SourceKind)
		if !visibilityDecision.Allowed {
			continue
		}
		ttl := s.policy.ResolveTTL("", candidate.MemoryType)
		docs = append(docs, buildMemoryDocumentEntity(candidate, scopeDecision, visibilityDecision, decision, ttl.ExpiresAt))
	}
	if len(docs) == 0 {
		return nil
	}
	if err := s.repo.BatchUpsertDocuments(ctx, docs); err != nil {
		return err
	}
	s.triggerDocumentIndex(ctx, docs)
	return nil
}

func buildMemoryFactEntity(
	candidate aidomain.MemoryFactCandidate,
	scopeDecision aidomain.MemoryScopeDecision,
	visibilityDecision aidomain.MemoryVisibilityDecision,
	expiresAt *time.Time,
) *entity.AIMemoryFact {
	now := time.Now()
	return &entity.AIMemoryFact{
		ScopeKey:      scopeDecision.ScopeKey,
		ScopeType:     string(scopeDecision.ScopeType),
		Visibility:    string(visibilityDecision.Visibility),
		UserID:        cloneMemoryUintPtr(scopeDecision.UserID),
		OrgID:         cloneMemoryUintPtr(scopeDecision.OrgID),
		Namespace:     candidate.Namespace,
		FactKey:       candidate.FactKey,
		FactValueJSON: candidate.FactValueJSON,
		Summary:       candidate.Summary,
		Confidence:    0.9,
		SourceKind:    string(candidate.SourceKind),
		SourceID:      candidate.SourceID,
		EffectiveAt:   &now,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func buildMemoryDocumentEntity(
	candidate aidomain.MemoryDocumentCandidate,
	scopeDecision aidomain.MemoryScopeDecision,
	visibilityDecision aidomain.MemoryVisibilityDecision,
	decision aidomain.MemoryDocumentDecision,
	expiresAt *time.Time,
) *entity.AIMemoryDocument {
	now := time.Now()
	return &entity.AIMemoryDocument{
		ID:             buildMemoryDocumentID(scopeDecision.ScopeKey, decision.DedupKey),
		ScopeKey:       scopeDecision.ScopeKey,
		ScopeType:      string(scopeDecision.ScopeType),
		Visibility:     string(visibilityDecision.Visibility),
		UserID:         cloneMemoryUintPtr(scopeDecision.UserID),
		OrgID:          cloneMemoryUintPtr(scopeDecision.OrgID),
		MemoryType:     string(candidate.MemoryType),
		Topic:          candidate.Topic,
		Title:          candidate.Title,
		Summary:        candidate.Summary,
		ContentText:    candidate.ContentText,
		ContentHash:    decision.ContentHash,
		SummaryHash:    decision.SummaryHash,
		DedupKey:       decision.DedupKey,
		Importance:     0.8,
		QualityScore:   0.8,
		EmbeddingModel: aiMemoryEmbedModel(),
		SourceKind:     string(candidate.SourceKind),
		SourceID:       candidate.SourceID,
		EffectiveAt:    &now,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s *AIMemoryService) buildMemoryAccessContext(input aiMemoryWritebackInput) aidomain.MemoryAccessContext {
	return aidomain.MemoryAccessContext{
		Principal: normalizeMemoryPrincipal(input),
	}
}

func normalizeMemoryPrincipal(input aiMemoryWritebackInput) aidomain.AIToolPrincipal {
	principal := input.Principal
	if principal.UserID == 0 {
		principal.UserID = input.UserID
	}
	if principal.CurrentOrgID == nil {
		principal.CurrentOrgID = cloneMemoryUintPtr(input.OrgID)
	}
	return principal
}

func aiEntityMessageToDomain(message *entity.AIMessage) aidomain.Message {
	if message == nil {
		return aidomain.Message{}
	}
	role := strings.TrimSpace(message.Role)
	if role != aidomain.RoleAssistant {
		role = aidomain.RoleUser
	}
	return aidomain.Message{
		ID:      message.ID,
		Role:    role,
		Content: message.Content,
	}
}

func previousSummaryText(summary *entity.AIConversationSummary) string {
	if summary == nil {
		return ""
	}
	return summary.SummaryText
}

func defaultMemoryJSONList(value string) string {
	if strings.TrimSpace(value) == "" {
		return "[]"
	}
	return value
}

func buildMemoryDocumentID(scopeKey string, dedupKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(scopeKey) + "\n" + strings.TrimSpace(dedupKey)))
	return "mem_doc_" + hex.EncodeToString(sum[:])[:32]
}

func aiMemoryEnabled() bool {
	return global.Config != nil && global.Config.AI.Memory.Enabled
}

func aiMemoryEntityEnabled() bool {
	return global.Config != nil && global.Config.AI.Memory.EnableEntityMemory
}

func aiMemoryLongTermEnabled() bool {
	return global.Config != nil && global.Config.AI.Memory.EnableLongTermMemory
}

func aiMemoryEmbedModel() string {
	if global.Config == nil {
		return "qwen3-vl-embedding"
	}
	if value := strings.TrimSpace(global.Config.AI.Memory.EmbedModel); value != "" {
		return value
	}
	return "qwen3-vl-embedding"
}

func aiMemoryAPIKey() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.APIKey)
}

func aiMemoryEmbedEndpoint() string {
	if global.Config == nil {
		return ""
	}
	return strings.TrimSpace(global.Config.AI.Memory.EmbedEndpoint)
}

func aiMemoryEmbedDimension() int {
	if global.Config == nil || global.Config.AI.Memory.EmbedDimension <= 0 {
		return 1024
	}
	return global.Config.AI.Memory.EmbedDimension
}

func aiMemoryChunkMaxChars() int {
	if global.Config == nil || global.Config.AI.Memory.ChunkMaxChars <= 0 {
		return 1200
	}
	return global.Config.AI.Memory.ChunkMaxChars
}

func aiMemoryChunkOverlapChars() int {
	if global.Config == nil || global.Config.AI.Memory.ChunkOverlapChars < 0 {
		return 150
	}
	return global.Config.AI.Memory.ChunkOverlapChars
}

func aiMemoryIndexBatchSize() int {
	if global.Config == nil || global.Config.AI.Memory.IndexBatchSize <= 0 {
		return 20
	}
	return global.Config.AI.Memory.IndexBatchSize
}

func aiMemoryIndexTimeoutSeconds() int {
	if global.Config == nil || global.Config.AI.Memory.IndexTimeoutSeconds <= 0 {
		return 30
	}
	return global.Config.AI.Memory.IndexTimeoutSeconds
}

func aiMemoryCollectionName() string {
	if global.Config == nil {
		return ""
	}
	if value := strings.TrimSpace(global.Config.Qdrant.MemoryCollectionName); value != "" {
		return value
	}
	return strings.TrimSpace(global.Config.Qdrant.CollectionName)
}
