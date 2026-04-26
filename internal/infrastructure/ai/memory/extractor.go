package memory

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	aidomain "personal_assistant/internal/domain/ai"
)

const (
	defaultSummaryMaxRunes         = 1800
	defaultDocumentMinRunes        = 240
	defaultDocumentSummaryMaxRunes = 240
)

// RuleExtractor is a conservative deterministic extractor for memory writeback.
type RuleExtractor struct {
	summaryMaxRunes         int
	documentMinRunes        int
	documentSummaryMaxRunes int
}

// Options configures RuleExtractor without coupling it to global config.
type Options struct {
	SummaryMaxRunes         int
	DocumentMinRunes        int
	DocumentSummaryMaxRunes int
}

// NewRuleExtractor creates the v1 rule-based memory extractor.
func NewRuleExtractor(opts Options) *RuleExtractor {
	if opts.SummaryMaxRunes <= 0 {
		opts.SummaryMaxRunes = defaultSummaryMaxRunes
	}
	if opts.DocumentMinRunes <= 0 {
		opts.DocumentMinRunes = defaultDocumentMinRunes
	}
	if opts.DocumentSummaryMaxRunes <= 0 {
		opts.DocumentSummaryMaxRunes = defaultDocumentSummaryMaxRunes
	}
	return &RuleExtractor{
		summaryMaxRunes:         opts.SummaryMaxRunes,
		documentMinRunes:        opts.DocumentMinRunes,
		documentSummaryMaxRunes: opts.DocumentSummaryMaxRunes,
	}
}

// Extract produces summary/fact/document candidates from the completed turn.
func (e *RuleExtractor) Extract(
	ctx context.Context,
	input aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	select {
	case <-ctx.Done():
		return aidomain.MemoryExtractionResult{}, ctx.Err()
	default:
	}

	result := aidomain.MemoryExtractionResult{
		Summary: e.buildSummary(input),
		Facts:   e.extractFacts(input),
	}
	if doc := e.extractDocument(input); doc != nil {
		result.Documents = append(result.Documents, *doc)
	}
	return result, nil
}

func (e *RuleExtractor) buildSummary(input aidomain.MemoryExtractionInput) *aidomain.ConversationSummaryDraft {
	userText := normalizeText(input.UserMessage.Content)
	assistantText := normalizeText(input.AssistantMessage.Content)
	if userText == "" && assistantText == "" {
		return nil
	}

	parts := make([]string, 0, 3)
	if previous := normalizeText(input.PreviousSummaryText); previous != "" {
		parts = append(parts, previous)
	}
	turn := strings.TrimSpace("用户: " + truncateRunes(userText, 320) + "\n助手: " + truncateRunes(assistantText, 900))
	if turn != "" {
		parts = append(parts, "最近一轮: "+turn)
	}
	summaryText := truncateRunes(strings.Join(parts, "\n\n"), e.summaryMaxRunes)
	if summaryText == "" {
		return nil
	}

	keyPoints, _ := json.Marshal([]string{truncateRunes(userText, 160)})
	return &aidomain.ConversationSummaryDraft{
		ConversationID:           input.ConversationID,
		CompressedUntilMessageID: input.AssistantMessage.ID,
		SummaryText:              summaryText,
		KeyPointsJSON:            string(keyPoints),
		OpenLoopsJSON:            "[]",
		TokenEstimate:            estimateTokens(summaryText),
	}
}

func (e *RuleExtractor) extractFacts(input aidomain.MemoryExtractionInput) []aidomain.MemoryFactCandidate {
	userText := normalizeText(input.UserMessage.Content)
	if userText == "" || input.UserID == 0 {
		return nil
	}

	facts := make([]aidomain.MemoryFactCandidate, 0, 2)
	if value := captureAfterAny(userText, []string{"我的目标是", "目标是"}); value != "" {
		facts = append(facts, newSelfFactCandidate(
			input.UserID,
			aidomain.MemoryNamespaceOJGoal,
			"current_goal",
			value,
			input.UserMessage.ID,
		))
	}
	if value := captureAfterAny(userText, []string{"以后请", "请以后", "以后都", "记住", "请记住"}); value != "" {
		facts = append(facts, newSelfFactCandidate(
			input.UserID,
			aidomain.MemoryNamespaceUserPreference,
			"answer_preference",
			value,
			input.UserMessage.ID,
		))
	}
	return facts
}

func (e *RuleExtractor) extractDocument(input aidomain.MemoryExtractionInput) *aidomain.MemoryDocumentCandidate {
	query := normalizeText(input.UserMessage.Content)
	content := normalizeText(input.AssistantMessage.Content)
	if utf8.RuneCountInString(content) < e.documentMinRunes || !isKnowledgeQuery(query) || input.UserID == 0 {
		return nil
	}

	summary := truncateRunes(firstParagraph(content), e.documentSummaryMaxRunes)
	if summary == "" {
		summary = truncateRunes(content, e.documentSummaryMaxRunes)
	}
	userID := input.UserID
	return &aidomain.MemoryDocumentCandidate{
		ScopeType:   aidomain.MemoryScopeSelf,
		UserID:      &userID,
		MemoryType:  aidomain.MemoryTypeSemantic,
		Topic:       inferTopic(query),
		Title:       truncateRunes(query, 120),
		Summary:     summary,
		ContentText: content,
		SourceKind:  aidomain.MemorySourceModelInferred,
		SourceID:    input.AssistantMessage.ID,
	}
}

func newSelfFactCandidate(
	userID uint,
	namespace string,
	factKey string,
	value string,
	sourceID string,
) aidomain.MemoryFactCandidate {
	value = truncateRunes(normalizeText(value), 240)
	payload, _ := json.Marshal(map[string]string{"value": value})
	return aidomain.MemoryFactCandidate{
		ScopeType:     aidomain.MemoryScopeSelf,
		UserID:        &userID,
		Namespace:     namespace,
		FactKey:       factKey,
		FactValueJSON: string(payload),
		Summary:       value,
		SourceKind:    aidomain.MemorySourceExplicitUserStatement,
		SourceID:      sourceID,
		LowValue:      utf8.RuneCountInString(value) < 2,
	}
}

func captureAfterAny(text string, markers []string) string {
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		value := strings.TrimSpace(text[idx+len(marker):])
		value = strings.Trim(value, " ：:，,。.;；")
		return truncateAtAny(value, []string{"。", "\n", "；", ";"})
	}
	return ""
}

func isKnowledgeQuery(query string) bool {
	keywords := []string{
		"方案", "步骤", "设计", "总结", "排障", "复盘", "如何做", "怎么做", "架构", "实现",
		"faq", "runbook", "troubleshoot", "design", "steps", "summary", "architecture",
	}
	lower := strings.ToLower(query)
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func inferTopic(query string) string {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "runbook"):
		return "runbook"
	case strings.Contains(lower, "faq"):
		return "faq"
	case strings.Contains(query, "排障"), strings.Contains(lower, "troubleshoot"):
		return "troubleshooting"
	case strings.Contains(query, "设计"), strings.Contains(query, "架构"), strings.Contains(lower, "design"), strings.Contains(lower, "architecture"):
		return "design"
	default:
		return "conversation_knowledge"
	}
}

func firstParagraph(value string) string {
	for _, sep := range []string{"\n\n", "\r\n\r\n"} {
		if idx := strings.Index(value, sep); idx >= 0 {
			return strings.TrimSpace(value[:idx])
		}
	}
	return value
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncateAtAny(value string, seps []string) string {
	for _, sep := range seps {
		if idx := strings.Index(value, sep); idx >= 0 {
			return strings.TrimSpace(value[:idx])
		}
	}
	return strings.TrimSpace(value)
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:limit]))
}

func estimateTokens(value string) int {
	runes := utf8.RuneCountInString(value)
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}
