package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

const minAIMemoryRecentRawMessageLimit = 2

type aiHybridContextPlanner interface {
	Plan(ctx context.Context, input aiHybridContextInput) (aiHybridContextResult, error)
}

type aiHybridContextInput struct {
	ConversationID string
	Query          string
	RawHistory     []aidomain.Message
	Recall         aiMemoryRecallResult
	VisibleTools   []aidomain.Tool
}

type aiHybridContextResult struct {
	History     []aidomain.Message
	Diagnostics aiHybridContextDiagnostics
}

type aiHybridContextDiagnostics struct {
	SummaryCandidates         int
	SummaryKept               int
	FactCandidates            int
	FactsKept                 int
	FactsDropped              int
	RAGCandidates             int
	RAGKept                   int
	RAGDropped                int
	RAGMinScore               float64
	RecallMaxChars            int
	RAGMaxChars               int
	MemoryChars               int
	MemoryTokens              int
	RecentMessagesTokenBudget int
	RecentMessagesTokens      int
	RawMessageCandidates      int
	RecentMessagesKept        int
	RawMessagesDropped        int
	CompressionTriggered      bool
	CompressionReason         string
	HistoryTokens             int
	CurrentQueryProvided      bool
	CurrentQueryInHistory     bool
	VisibleTools              int
	RAGRemainingChars         int
}

type defaultAIHybridContextPlanner struct{}

func newDefaultAIHybridContextPlanner() aiHybridContextPlanner {
	return &defaultAIHybridContextPlanner{}
}

func (p *defaultAIHybridContextPlanner) Plan(
	ctx context.Context,
	input aiHybridContextInput,
) (aiHybridContextResult, error) {
	_ = ctx
	rawHistory := append([]aidomain.Message(nil), input.RawHistory...)
	diagnostics := input.Recall.Diagnostics
	diagnostics.RawMessageCandidates = len(rawHistory)
	diagnostics.VisibleTools = len(input.VisibleTools)
	diagnostics.CurrentQueryProvided = strings.TrimSpace(input.Query) != ""
	diagnostics.CurrentQueryInHistory = aiHistoryContainsCurrentQuery(rawHistory, input.Query)
	diagnostics.RecentMessagesTokenBudget = aiMemoryRecentRawTokenBudget()
	diagnostics.CompressionReason = "skip"

	if !aiMemoryEnabled() {
		diagnostics.RecentMessagesKept = len(rawHistory)
		diagnostics.RecentMessagesTokens = estimateAIMemoryTokens(rawHistory)
		diagnostics.HistoryTokens = estimateAIMemoryTokens(rawHistory)
		return aiHybridContextResult{History: rawHistory, Diagnostics: diagnostics}, nil
	}

	memoryMessages := append([]aidomain.Message(nil), input.Recall.Messages...)
	reordered := joinAIMemoryFirst(memoryMessages, rawHistory)
	threshold := aiMemoryCompressThresholdTokens()
	totalTokens := estimateAIMemoryTokens(reordered)
	if threshold <= 0 || totalTokens <= threshold {
		diagnostics.RecentMessagesKept = len(rawHistory)
		diagnostics.RecentMessagesTokens = estimateAIMemoryTokens(rawHistory)
		diagnostics.HistoryTokens = totalTokens
		return aiHybridContextResult{History: reordered, Diagnostics: diagnostics}, nil
	}

	diagnostics.CompressionTriggered = true
	recentSelection := selectRecentAIMemoryRawMessagesByBudget(
		rawHistory,
		aiMemoryRecentRawMessageLimitWithMinimum(),
		aiMemoryRecentRawTokenBudget(),
	)
	recent := recentSelection.Messages
	diagnostics.RecentMessagesKept = len(recent)
	diagnostics.RecentMessagesTokens = recentSelection.Tokens
	if dropped := len(rawHistory) - len(recent); dropped > 0 {
		diagnostics.RawMessagesDropped = dropped
		diagnostics.CompressionReason = "budget"
	} else {
		diagnostics.CompressionReason = "threshold"
	}
	history := joinAIMemoryFirst(memoryMessages, recent)
	diagnostics.HistoryTokens = estimateAIMemoryTokens(history)
	return aiHybridContextResult{History: history, Diagnostics: diagnostics}, nil
}

func buildAIMemoryContextContent(
	summary *entity.AIConversationSummary,
	facts []*entity.AIMemoryFact,
	ragItems []aiMemoryRAGRecallItem,
	maxChars int,
) (string, aiHybridContextDiagnostics) {
	diagnostics := aiHybridContextDiagnostics{
		RAGMinScore:               aiMemoryRecallMinScore(),
		RecallMaxChars:            maxChars,
		RAGMaxChars:               aiMemoryRAGMaxChars(),
		RecentMessagesTokenBudget: aiMemoryRecentRawTokenBudget(),
	}
	summaryText := ""
	if summary != nil {
		summaryText = normalizeAIMemoryContextLine(summary.SummaryText)
	}
	keyPoints := decodeAIMemorySummaryLines(summaryKeyPointsJSON(summary))
	openLoops := decodeAIMemorySummaryLines(summaryOpenLoopsJSON(summary))
	if summaryText != "" || len(keyPoints) > 0 || len(openLoops) > 0 {
		diagnostics.SummaryCandidates = 1
		diagnostics.SummaryKept = 1
	}

	factLines := renderSortedAIMemoryFactLines(facts)
	diagnostics.FactCandidates = len(factLines)
	ragLines := renderSortedAIMemoryRAGLines(ragItems)
	diagnostics.RAGCandidates = len(ragLines)
	if summaryText == "" && len(keyPoints) == 0 && len(openLoops) == 0 && len(factLines) == 0 && len(ragLines) == 0 {
		return "", diagnostics
	}

	var builder strings.Builder
	appendAIMemoryContextPart(&builder, aiMemoryContextMessageHeader, maxChars)
	if len(keyPoints) > 0 {
		appendAIMemoryContextPart(&builder, "\n\n## Latest Decisions\n", maxChars)
		for _, line := range keyPoints {
			appendAIMemoryContextPart(&builder, "- "+line+"\n", maxChars)
		}
	}
	if len(openLoops) > 0 {
		appendAIMemoryContextPart(&builder, "\n\n## Open Loops\n", maxChars)
		for _, line := range openLoops {
			appendAIMemoryContextPart(&builder, "- "+line+"\n", maxChars)
		}
	}
	appendAIMemoryContextPart(&builder, "\n\n## Conversation Summary\n", maxChars)
	if summaryText == "" {
		appendAIMemoryContextPart(&builder, "- 无", maxChars)
	} else {
		appendAIMemoryContextPart(&builder, summaryText, maxChars)
	}

	for _, line := range factLines {
		prefix := "\n"
		if diagnostics.FactsKept == 0 {
			prefix = "\n\n## Stable Facts\n"
		}
		if appendAIMemoryContextPart(&builder, prefix+"- "+line+"\n", maxChars) {
			diagnostics.FactsKept++
		}
	}
	diagnostics.FactsDropped = diagnostics.FactCandidates - diagnostics.FactsKept

	ragBudget := aiMemoryRAGMaxChars()
	diagnostics.RAGRemainingChars = ragBudget
	ragUsed := 0
	for _, line := range ragLines {
		prefix := ""
		if diagnostics.RAGKept == 0 {
			prefix = "\n\n## Long-term Documents\n"
		}
		part := prefix + line + "\n"
		partRunes := utf8.RuneCountInString(part)
		if ragBudget > 0 && ragUsed+partRunes > ragBudget {
			remaining := ragBudget - ragUsed
			if remaining <= utf8.RuneCountInString(prefix)+1 {
				break
			}
			part = truncateAIMemoryContext(part, remaining)
			partRunes = utf8.RuneCountInString(part)
		}
		if appendAIMemoryContextPart(&builder, part, maxChars) {
			diagnostics.RAGKept++
			ragUsed += partRunes
			diagnostics.RAGRemainingChars = ragBudget - ragUsed
		} else {
			break
		}
	}
	diagnostics.RAGDropped = diagnostics.RAGCandidates - diagnostics.RAGKept

	content := strings.TrimRight(builder.String(), "\n")
	diagnostics.MemoryChars = utf8.RuneCountInString(content)
	diagnostics.MemoryTokens = estimateAIMemoryTokens([]aidomain.Message{{Content: content}})
	return content, diagnostics
}

func appendAIMemoryContextPart(builder *strings.Builder, part string, maxChars int) bool {
	if part == "" {
		return false
	}
	if maxChars <= 0 {
		builder.WriteString(part)
		return true
	}
	current := utf8.RuneCountInString(builder.String())
	remaining := maxChars - current
	if remaining <= 0 {
		return false
	}
	partRunes := utf8.RuneCountInString(part)
	if partRunes <= remaining {
		builder.WriteString(part)
		return true
	}
	if remaining <= utf8.RuneCountInString(aiMemoryContextTruncationIndicator) {
		return false
	}
	builder.WriteString(truncateAIMemoryContext(part, remaining))
	return true
}

func renderSortedAIMemoryFactLines(facts []*entity.AIMemoryFact) []string {
	sorted := append([]*entity.AIMemoryFact(nil), facts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return compareAIMemoryFacts(sorted[i], sorted[j])
	})

	lines := make([]string, 0, len(sorted))
	for _, fact := range sorted {
		line, ok := renderAIMemoryFactLine(fact)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func renderAIMemoryFactLine(fact *entity.AIMemoryFact) (string, bool) {
	if fact == nil {
		return "", false
	}
	namespace := normalizeAIMemoryContextLine(fact.Namespace)
	factKey := normalizeAIMemoryContextLine(fact.FactKey)
	value := normalizeAIMemoryContextLine(fact.Summary)
	if value == "" {
		value = normalizeAIMemoryContextLine(fact.FactValueJSON)
	}
	if namespace == "" || factKey == "" || value == "" {
		return "", false
	}
	return fmt.Sprintf("%s/%s: %s", namespace, factKey, value), true
}

func renderSortedAIMemoryRAGLines(items []aiMemoryRAGRecallItem) []string {
	sorted := append([]aiMemoryRAGRecallItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		leftID := ""
		rightID := ""
		if sorted[i].Chunk != nil {
			leftID = sorted[i].Chunk.ID
		}
		if sorted[j].Chunk != nil {
			rightID = sorted[j].Chunk.ID
		}
		return leftID < rightID
	})

	lines := make([]string, 0, len(sorted))
	for _, item := range sorted {
		line, ok := renderAIMemoryRAGLine(item)
		if ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func renderAIMemoryRAGLine(item aiMemoryRAGRecallItem) (string, bool) {
	if item.Chunk == nil {
		return "", false
	}
	content := strings.TrimSpace(item.ExpandedText)
	if content == "" {
		content = item.Chunk.ContentText
	}
	content = normalizeAIMemoryContextLine(content)
	if content == "" {
		return "", false
	}
	topic := normalizeAIMemoryContextLine(item.Chunk.Topic)
	memoryType := normalizeAIMemoryContextLine(item.Chunk.MemoryType)
	var builder strings.Builder
	builder.WriteString("- ")
	if topic != "" || memoryType != "" {
		builder.WriteString("[")
		if memoryType != "" {
			builder.WriteString(memoryType)
		}
		if topic != "" {
			if memoryType != "" {
				builder.WriteString("/")
			}
			builder.WriteString(topic)
		}
		builder.WriteString(fmt.Sprintf(" score=%.3f] ", item.Score))
	}
	builder.WriteString(content)
	return builder.String(), true
}

func aiMemorySourcePriority(sourceKind string) int {
	source := strings.ToLower(strings.TrimSpace(sourceKind))
	switch source {
	case string(aidomain.MemorySourceExplicitUserStatement):
		return 0
	case string(aidomain.MemorySourceToolVerifiedSummary):
		return 1
	case string(aidomain.MemorySourceAdminSet):
		return 2
	case string(aidomain.MemorySourceModelInferred):
		return 3
	}
	if strings.Contains(source, "explicit") || strings.Contains(source, "user") {
		return 0
	}
	if strings.Contains(source, "tool") || strings.Contains(source, "service") || strings.Contains(source, "realtime") {
		return 1
	}
	if strings.Contains(source, "admin") || strings.Contains(source, "manual") {
		return 2
	}
	return 4
}

func aiMemoryNamespacePriority(namespace string) int {
	switch strings.TrimSpace(namespace) {
	case aidomain.MemoryNamespaceUserPreference:
		return 0
	case aidomain.MemoryNamespaceOJGoal:
		return 1
	case aidomain.MemoryNamespaceOJProfile:
		return 2
	default:
		return 3
	}
}

func compareAIMemoryFacts(left *entity.AIMemoryFact, right *entity.AIMemoryFact) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	leftNamespacePriority := aiMemoryNamespacePriority(left.Namespace)
	rightNamespacePriority := aiMemoryNamespacePriority(right.Namespace)
	if leftNamespacePriority != rightNamespacePriority {
		return leftNamespacePriority < rightNamespacePriority
	}
	leftPriority := aiMemorySourcePriority(left.SourceKind)
	rightPriority := aiMemorySourcePriority(right.SourceKind)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if left.Confidence != right.Confidence {
		return left.Confidence > right.Confidence
	}
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	return left.FactKey < right.FactKey
}

func decodeAIMemorySummaryLines(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = normalizeAIMemoryContextLine(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func summaryKeyPointsJSON(summary *entity.AIConversationSummary) string {
	if summary == nil {
		return ""
	}
	return summary.KeyPointsJSON
}

func summaryOpenLoopsJSON(summary *entity.AIConversationSummary) string {
	if summary == nil {
		return ""
	}
	return summary.OpenLoopsJSON
}

func aiMemoryRecentRawMessageLimitWithMinimum() int {
	limit := aiMemoryRecentRawMessageLimit()
	if limit < minAIMemoryRecentRawMessageLimit {
		return minAIMemoryRecentRawMessageLimit
	}
	return limit
}

func aiHistoryContainsCurrentQuery(history []aidomain.Message, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	for _, message := range history {
		if message.Role == aidomain.RoleUser && strings.TrimSpace(message.Content) == query {
			return true
		}
	}
	return false
}
