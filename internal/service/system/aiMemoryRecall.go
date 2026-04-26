package system

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
)

const (
	defaultAIMemoryRecallTopK          = 10
	defaultAIMemoryRecallMaxChars      = 4000
	defaultAIMemoryRecentRawTurns      = 8
	aiMemoryContextMessageIDPrefix     = "memory_context"
	aiMemoryContextMessageHeader       = "以下是系统恢复的记忆上下文，仅作为背景，不代表用户本轮新输入。"
	aiMemoryContextTruncationIndicator = "\n..."
)

// Recall 从已沉淀的 summary 和 self facts 中恢复本轮可读的记忆上下文。
func (s *AIMemoryService) Recall(ctx context.Context, input aiMemoryRecallInput) (aiMemoryRecallResult, error) {
	if !aiMemoryEnabled() || s == nil || s.repo == nil || input.UserID == 0 {
		return aiMemoryRecallResult{}, nil
	}

	summary, err := s.recallConversationSummary(ctx, input)
	if err != nil {
		return aiMemoryRecallResult{}, err
	}
	facts, err := s.recallSelfFacts(ctx, input.UserID)
	if err != nil {
		return aiMemoryRecallResult{}, err
	}

	content := buildAIMemoryContextContent(summary, facts, input.Query, aiMemoryRecallMaxChars())
	if strings.TrimSpace(content) == "" {
		return aiMemoryRecallResult{}, nil
	}
	message := aidomain.Message{
		ID:      buildAIMemoryContextMessageID(input.ConversationID),
		Role:    aidomain.RoleAssistant,
		Content: content,
	}
	return aiMemoryRecallResult{
		PromptBlocks: []string{content},
		Messages:     []aidomain.Message{message},
	}, nil
}

// RecallMessages 满足 aiContextAssembler 的记忆扩展点。
func (s *AIMemoryService) RecallMessages(ctx context.Context, input aiMemoryRecallInput) ([]aidomain.Message, error) {
	result, err := s.Recall(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// CompressMessages 在进入 runtime 前把上下文压成 memory + recent turns。
func (s *AIMemoryService) CompressMessages(ctx context.Context, input aiContextCompressionInput) ([]aidomain.Message, error) {
	_ = ctx
	if !aiMemoryEnabled() || len(input.Messages) == 0 {
		return input.Messages, nil
	}

	memoryMessages, rawMessages := splitAIMemoryContextMessages(input.Messages)
	reordered := joinAIMemoryFirst(memoryMessages, rawMessages)
	threshold := aiMemoryCompressThresholdTokens()
	if threshold <= 0 || estimateAIMemoryTokens(reordered) <= threshold {
		return reordered, nil
	}

	recent := selectRecentAIMemoryRawMessages(rawMessages, aiMemoryRecentRawMessageLimit())
	return joinAIMemoryFirst(memoryMessages, recent), nil
}

func (s *AIMemoryService) recallConversationSummary(
	ctx context.Context,
	input aiMemoryRecallInput,
) (*entity.AIConversationSummary, error) {
	conversationID := strings.TrimSpace(input.ConversationID)
	if conversationID == "" {
		return nil, nil
	}
	orgID := cloneMemoryUintPtr(input.ToolCallCtx.Principal.CurrentOrgID)
	scopeKey := aidomain.BuildConversationMemoryScopeKey(input.UserID, orgID)
	return s.repo.GetConversationSummary(ctx, aidomain.MemoryConversationSummaryQuery{
		ConversationID: conversationID,
		UserID:         input.UserID,
		OrgID:          orgID,
		ScopeKey:       scopeKey,
	})
}

func (s *AIMemoryService) recallSelfFacts(ctx context.Context, userID uint) ([]*entity.AIMemoryFact, error) {
	if !aiMemoryEntityEnabled() || userID == 0 {
		return nil, nil
	}
	return s.repo.ListFacts(ctx, aidomain.MemoryFactQuery{
		ScopeKeys:           []string{aidomain.BuildSelfMemoryScopeKey(userID)},
		AllowedVisibilities: []aidomain.MemoryVisibility{aidomain.MemoryVisibilitySelf},
		Limit:               aiMemoryRecallTopK(),
	})
}

func buildAIMemoryContextContent(
	summary *entity.AIConversationSummary,
	facts []*entity.AIMemoryFact,
	query string,
	maxChars int,
) string {
	summaryText := ""
	if summary != nil {
		summaryText = normalizeAIMemoryContextLine(summary.SummaryText)
	}
	factLines := renderAIMemoryFactLines(facts)
	currentQuery := normalizeAIMemoryContextLine(query)
	if summaryText == "" && len(factLines) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(aiMemoryContextMessageHeader)
	builder.WriteString("\n\n## Conversation Summary\n")
	if summaryText == "" {
		builder.WriteString("- 无")
	} else {
		builder.WriteString(summaryText)
	}
	if len(factLines) > 0 {
		builder.WriteString("\n\n## Stable Facts\n")
		for _, line := range factLines {
			builder.WriteString("- ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	if currentQuery != "" {
		builder.WriteString("\n## Current Query\n")
		builder.WriteString(currentQuery)
	}
	return truncateAIMemoryContext(builder.String(), maxChars)
}

func renderAIMemoryFactLines(facts []*entity.AIMemoryFact) []string {
	if len(facts) == 0 {
		return nil
	}
	lines := make([]string, 0, len(facts))
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		namespace := normalizeAIMemoryContextLine(fact.Namespace)
		factKey := normalizeAIMemoryContextLine(fact.FactKey)
		value := normalizeAIMemoryContextLine(fact.Summary)
		if value == "" {
			value = normalizeAIMemoryContextLine(fact.FactValueJSON)
		}
		if namespace == "" || factKey == "" || value == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s/%s: %s", namespace, factKey, value))
	}
	return lines
}

func splitAIMemoryContextMessages(messages []aidomain.Message) ([]aidomain.Message, []aidomain.Message) {
	memoryMessages := make([]aidomain.Message, 0, 1)
	rawMessages := make([]aidomain.Message, 0, len(messages))
	for _, message := range messages {
		if isAIMemoryContextMessage(message) {
			memoryMessages = append(memoryMessages, message)
			continue
		}
		rawMessages = append(rawMessages, message)
	}
	return memoryMessages, rawMessages
}

func joinAIMemoryFirst(memoryMessages []aidomain.Message, rawMessages []aidomain.Message) []aidomain.Message {
	if len(memoryMessages) == 0 {
		return append([]aidomain.Message(nil), rawMessages...)
	}
	items := make([]aidomain.Message, 0, len(memoryMessages)+len(rawMessages))
	items = append(items, memoryMessages...)
	items = append(items, rawMessages...)
	return items
}

func selectRecentAIMemoryRawMessages(messages []aidomain.Message, limit int) []aidomain.Message {
	if limit <= 0 || len(messages) <= limit {
		return append([]aidomain.Message(nil), messages...)
	}
	return append([]aidomain.Message(nil), messages[len(messages)-limit:]...)
}

func isAIMemoryContextMessage(message aidomain.Message) bool {
	id := strings.TrimSpace(message.ID)
	if strings.HasPrefix(id, aiMemoryContextMessageIDPrefix) {
		return true
	}
	return strings.Contains(message.Content, aiMemoryContextMessageHeader)
}

func buildAIMemoryContextMessageID(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return aiMemoryContextMessageIDPrefix
	}
	return aiMemoryContextMessageIDPrefix + "_" + conversationID
}

func normalizeAIMemoryContextLine(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncateAIMemoryContext(value string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(value) <= maxChars {
		return value
	}
	indicatorRunes := utf8.RuneCountInString(aiMemoryContextTruncationIndicator)
	limit := maxChars - indicatorRunes
	if limit <= 0 {
		limit = maxChars
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + aiMemoryContextTruncationIndicator
}

func estimateAIMemoryTokens(messages []aidomain.Message) int {
	totalRunes := 0
	for _, message := range messages {
		totalRunes += utf8.RuneCountInString(message.Content)
	}
	if totalRunes == 0 {
		return 0
	}
	return (totalRunes + 3) / 4
}

func aiMemoryRecallTopK() int {
	if global.Config == nil || global.Config.AI.Memory.RecallTopK <= 0 {
		return defaultAIMemoryRecallTopK
	}
	return global.Config.AI.Memory.RecallTopK
}

func aiMemoryRecallMaxChars() int {
	if global.Config == nil || global.Config.AI.Memory.RecallMaxChars <= 0 {
		return defaultAIMemoryRecallMaxChars
	}
	return global.Config.AI.Memory.RecallMaxChars
}

func aiMemoryRecentRawMessageLimit() int {
	turns := defaultAIMemoryRecentRawTurns
	if global.Config != nil && global.Config.AI.Memory.RecentRawTurns > 0 {
		turns = global.Config.AI.Memory.RecentRawTurns
	}
	return turns * 2
}

func aiMemoryCompressThresholdTokens() int {
	if global.Config == nil {
		return 0
	}
	return global.Config.AI.Memory.CompressThresholdTokens
}
