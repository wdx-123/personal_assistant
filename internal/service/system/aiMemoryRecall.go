package system

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	aimemory "personal_assistant/internal/infrastructure/ai/memory"
	"personal_assistant/internal/model/entity"

	"go.uber.org/zap"
)

const (
	defaultAIMemoryRecallTopK           = 10
	defaultAIMemoryRecallMaxChars       = 4000
	defaultAIMemoryRecallMinScore       = 0.2
	defaultAIMemoryRAGMaxChars          = 2000
	defaultAIMemoryRecentRawTurns       = 8
	defaultAIMemoryRecentRawTokenBudget = 3000
	aiMemoryContextMessageIDPrefix      = "memory_context"
	aiMemoryContextMessageHeader        = "以下是系统恢复的记忆上下文，仅作为背景，不代表用户本轮新输入。"
	aiMemoryContextTruncationIndicator  = "\n..."
)

type aiMemoryRAGRecallItem struct {
	Score          float64
	Chunk          *entity.AIMemoryDocumentChunk
	ExpandedChunks []*entity.AIMemoryDocumentChunk
	ExpandedText   string
}

type aiRecentMessageSelection struct {
	Messages []aidomain.Message
	Tokens   int
}

// Recall 从已沉淀的 summary 和 self facts 中恢复本轮可读的记忆上下文。
func (s *AIMemoryService) Recall(ctx context.Context, input aiMemoryRecallInput) (aiMemoryRecallResult, error) {
	if !aiMemoryEnabled() || s == nil || s.repo == nil || input.UserID == 0 {
		return aiMemoryRecallResult{}, nil
	}

	summary, err := s.recallConversationSummary(ctx, input)
	if err != nil {
		return aiMemoryRecallResult{}, err
	}
	facts, err := s.recallSelfFacts(ctx, input.UserID, input.Query)
	if err != nil {
		return aiMemoryRecallResult{}, err
	}
	ragItems := s.recallLongTermDocumentsFailOpen(ctx, input)

	content, diagnostics := buildAIMemoryContextContent(summary, facts, ragItems, aiMemoryRecallMaxChars())
	if strings.TrimSpace(content) == "" {
		return aiMemoryRecallResult{
			Summary:     summary,
			Facts:       facts,
			RAGItems:    ragItems,
			Diagnostics: diagnostics,
		}, nil
	}
	message := aidomain.Message{
		ID:      buildAIMemoryContextMessageID(input.ConversationID),
		Role:    aidomain.RoleAssistant,
		Content: content,
	}
	return aiMemoryRecallResult{
		PromptBlocks: []string{content},
		Messages:     []aidomain.Message{message},
		Summary:      summary,
		Facts:        facts,
		RAGItems:     ragItems,
		Diagnostics:  diagnostics,
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

	recentSelection := selectRecentAIMemoryRawMessagesByBudget(
		rawMessages,
		aiMemoryRecentRawMessageLimit(),
		aiMemoryRecentRawTokenBudget(),
	)
	return joinAIMemoryFirst(memoryMessages, recentSelection.Messages), nil
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

func (s *AIMemoryService) recallSelfFacts(ctx context.Context, userID uint, query string) ([]*entity.AIMemoryFact, error) {
	if !aiMemoryEntityEnabled() || userID == 0 {
		return nil, nil
	}
	namespaces, namespaceFiltered := aiMemoryFactNamespacesForQuery(query)
	rows, err := s.repo.ListFacts(ctx, aidomain.MemoryFactQuery{
		ScopeKeys:           []string{aidomain.BuildSelfMemoryScopeKey(userID)},
		AllowedVisibilities: []aidomain.MemoryVisibility{aidomain.MemoryVisibilitySelf},
		Namespaces:          namespaces,
		Limit:               aiMemoryFactScanLimit(),
	})
	if err != nil {
		return nil, err
	}
	sortAIMemoryFacts(rows)
	if namespaceFiltered {
		rows = filterAIMemoryFactsByNamespaces(rows, namespaces)
	}
	limit := aiMemoryRecallTopK()
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *AIMemoryService) recallLongTermDocumentsFailOpen(
	ctx context.Context,
	input aiMemoryRecallInput,
) []aiMemoryRAGRecallItem {
	items, err := s.recallLongTermDocuments(ctx, input)
	if err == nil {
		return items
	}
	if global.Log != nil {
		global.Log.Warn(
			"AI memory RAG recall failed",
			zap.String("conversation_id", input.ConversationID),
			zap.Uint("user_id", input.UserID),
			zap.Error(err),
		)
	}
	return nil
}

func (s *AIMemoryService) recallLongTermDocuments(
	ctx context.Context,
	input aiMemoryRecallInput,
) ([]aiMemoryRAGRecallItem, error) {
	query := strings.TrimSpace(input.Query)
	if !aiMemoryLongTermEnabled() ||
		query == "" ||
		s == nil ||
		s.repo == nil ||
		s.embedder == nil ||
		s.vectorSearcher == nil ||
		input.UserID == 0 {
		return nil, nil
	}

	embedding, err := s.embedder.Embed(ctx, aidomain.MemoryEmbeddingInput{Texts: []string{query}})
	if err != nil {
		return nil, err
	}
	if len(embedding.Vectors) != 1 {
		return nil, fmt.Errorf("memory query embedding count = %d, want 1", len(embedding.Vectors))
	}
	vector := embedding.Vectors[0]
	if len(vector) != aiMemoryEmbedDimension() {
		return nil, fmt.Errorf("memory query embedding dimension = %d, want %d", len(vector), aiMemoryEmbedDimension())
	}

	minScore := aiMemoryRecallMinScore()
	results, err := s.vectorSearcher.SearchChunks(ctx, aidomain.MemoryVectorSearchInput{
		Vector:     vector,
		ScopeKey:   aidomain.BuildSelfMemoryScopeKey(input.UserID),
		Visibility: string(aidomain.MemoryVisibilitySelf),
		UserID:     input.UserID,
		Limit:      aiMemoryRecallTopK(),
		MinScore:   minScore,
	})
	if err != nil {
		return nil, err
	}
	pointIDs := make([]string, 0, len(results))
	for _, result := range results {
		if result.Score < minScore {
			continue
		}
		if pointID := strings.TrimSpace(result.QdrantPointID); pointID != "" {
			pointIDs = append(pointIDs, pointID)
		}
	}
	if len(pointIDs) == 0 {
		return nil, nil
	}

	chunks, err := s.repo.ListDocumentChunksByPointIDs(ctx, pointIDs)
	if err != nil {
		return nil, err
	}
	chunkByPointID := make(map[string]*entity.AIMemoryDocumentChunk, len(chunks))
	scopeKey := aidomain.BuildSelfMemoryScopeKey(input.UserID)
	for _, chunk := range chunks {
		if !isValidAIMemoryRAGChunk(chunk, scopeKey, input.UserID) {
			continue
		}
		chunkByPointID[strings.TrimSpace(chunk.QdrantPointID)] = chunk
	}

	items := make([]aiMemoryRAGRecallItem, 0, len(results))
	for _, result := range results {
		if result.Score < minScore {
			continue
		}
		chunk := chunkByPointID[strings.TrimSpace(result.QdrantPointID)]
		if chunk == nil {
			continue
		}
		items = append(items, aiMemoryRAGRecallItem{Score: result.Score, Chunk: chunk})
	}
	if len(items) == 0 {
		return nil, nil
	}
	if err := s.expandRAGItems(ctx, input.UserID, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *AIMemoryService) expandRAGItems(
	ctx context.Context,
	userID uint,
	items []aiMemoryRAGRecallItem,
) error {
	refs := make([]aidomain.MemoryDocumentChunkRef, 0, len(items)*3)
	for _, item := range items {
		if item.Chunk == nil {
			continue
		}
		for index := item.Chunk.ChunkIndex - 1; index <= item.Chunk.ChunkIndex+1; index++ {
			if index < 0 {
				continue
			}
			refs = append(refs, aidomain.MemoryDocumentChunkRef{
				DocumentID: item.Chunk.DocumentID,
				ChunkIndex: index,
			})
		}
	}
	if len(refs) == 0 {
		return nil
	}

	rows, err := s.repo.ListDocumentChunksByRefs(ctx, refs)
	if err != nil {
		return err
	}
	scopeKey := aidomain.BuildSelfMemoryScopeKey(userID)
	chunkByRef := make(map[string]*entity.AIMemoryDocumentChunk, len(rows))
	for _, row := range rows {
		if !isValidAIMemoryRAGChunk(row, scopeKey, userID) {
			continue
		}
		chunkByRef[buildAIMemoryChunkRefKey(row.DocumentID, row.ChunkIndex)] = row
	}

	for index := range items {
		primary := items[index].Chunk
		if primary == nil {
			continue
		}
		expanded := make([]*entity.AIMemoryDocumentChunk, 0, 3)
		texts := make([]string, 0, 3)
		for chunkIndex := primary.ChunkIndex - 1; chunkIndex <= primary.ChunkIndex+1; chunkIndex++ {
			if chunkIndex < 0 {
				continue
			}
			chunk := chunkByRef[buildAIMemoryChunkRefKey(primary.DocumentID, chunkIndex)]
			if chunk == nil {
				continue
			}
			expanded = append(expanded, chunk)
			texts = append(texts, chunk.ContentText)
		}
		if len(expanded) == 0 {
			expanded = []*entity.AIMemoryDocumentChunk{primary}
			texts = []string{primary.ContentText}
		}
		items[index].ExpandedChunks = expanded
		items[index].ExpandedText = aimemory.MergeChunkTextsWithOverlap(texts, aiMemoryChunkOverlapChars())
	}
	return nil
}

func isValidAIMemoryRAGChunk(chunk *entity.AIMemoryDocumentChunk, scopeKey string, userID uint) bool {
	if chunk == nil {
		return false
	}
	if chunk.EmbeddingModel != aiMemoryEmbedModel() || chunk.EmbeddingDimension != aiMemoryEmbedDimension() {
		return false
	}
	if chunk.ScopeKey != scopeKey || chunk.Visibility != string(aidomain.MemoryVisibilitySelf) {
		return false
	}
	if chunk.UserID == nil || *chunk.UserID != userID {
		return false
	}
	return true
}

func buildAIMemoryChunkRefKey(documentID string, chunkIndex int) string {
	return strings.TrimSpace(documentID) + "#" + fmt.Sprintf("%d", chunkIndex)
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

func selectRecentAIMemoryRawMessagesByBudget(
	messages []aidomain.Message,
	limit int,
	tokenBudget int,
) aiRecentMessageSelection {
	if len(messages) == 0 {
		return aiRecentMessageSelection{}
	}
	if limit <= 0 && tokenBudget <= 0 {
		return aiRecentMessageSelection{
			Messages: append([]aidomain.Message(nil), messages...),
			Tokens:   estimateAIMemoryTokens(messages),
		}
	}

	selected := make(map[int]struct{}, len(messages))
	tokens := 0
	add := func(index int) {
		if index < 0 || index >= len(messages) {
			return
		}
		if _, exists := selected[index]; exists {
			return
		}
		selected[index] = struct{}{}
		tokens += estimateAIMemoryTokens([]aidomain.Message{messages[index]})
	}

	lastUserIndex := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == aidomain.RoleUser {
			lastUserIndex = i
			break
		}
	}
	if lastUserIndex >= 0 {
		add(lastUserIndex)
		if next := lastUserIndex + 1; next < len(messages) && messages[next].Role == aidomain.RoleAssistant {
			add(next)
		}
	} else {
		add(len(messages) - 1)
	}

	for i := len(messages) - 1; i >= 0; i-- {
		if _, exists := selected[i]; exists {
			continue
		}
		if limit > 0 && len(selected) >= limit {
			break
		}
		messageTokens := estimateAIMemoryTokens([]aidomain.Message{messages[i]})
		if tokenBudget > 0 && tokens+messageTokens > tokenBudget {
			break
		}
		add(i)
	}

	ordered := make([]aidomain.Message, 0, len(selected))
	for i := 0; i < len(messages); i++ {
		if _, exists := selected[i]; exists {
			ordered = append(ordered, messages[i])
		}
	}
	return aiRecentMessageSelection{Messages: ordered, Tokens: tokens}
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

func aiMemoryRecallMinScore() float64 {
	if global.Config == nil || global.Config.AI.Memory.RecallMinScore <= 0 {
		return defaultAIMemoryRecallMinScore
	}
	return global.Config.AI.Memory.RecallMinScore
}

func aiMemoryRAGMaxChars() int {
	if global.Config == nil || global.Config.AI.Memory.RAGMaxChars <= 0 {
		return defaultAIMemoryRAGMaxChars
	}
	return global.Config.AI.Memory.RAGMaxChars
}

func aiMemoryRecentRawMessageLimit() int {
	turns := defaultAIMemoryRecentRawTurns
	if global.Config != nil && global.Config.AI.Memory.RecentRawTurns > 0 {
		turns = global.Config.AI.Memory.RecentRawTurns
	}
	return turns * 2
}

func aiMemoryRecentRawTokenBudget() int {
	if global.Config == nil || global.Config.AI.Memory.RecentRawTokenBudget <= 0 {
		return defaultAIMemoryRecentRawTokenBudget
	}
	return global.Config.AI.Memory.RecentRawTokenBudget
}

func aiMemoryCompressThresholdTokens() int {
	if global.Config == nil {
		return 0
	}
	return global.Config.AI.Memory.CompressThresholdTokens
}

func aiMemorySummaryRefreshEveryTurns() int {
	if global.Config == nil || global.Config.AI.Memory.SummaryRefreshEveryTurns <= 0 {
		return 10
	}
	return global.Config.AI.Memory.SummaryRefreshEveryTurns
}

func aiMemoryFactScanLimit() int {
	limit := aiMemoryRecallTopK() * 4
	if limit < 20 {
		limit = 20
	}
	return limit
}

func aiMemoryFactNamespacesForQuery(query string) ([]string, bool) {
	namespaces := []string{aidomain.MemoryNamespaceUserPreference}
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return nil, false
	}
	keywords := []string{
		"目标", "计划", "刷题", "oj", "leetcode", "codeforces", "面试", "学习", "luogu", "lanqiao",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			namespaces = append(namespaces, aidomain.MemoryNamespaceOJGoal, aidomain.MemoryNamespaceOJProfile)
			return uniqueAIMemoryNamespaces(namespaces), true
		}
	}
	return nil, false
}

func uniqueAIMemoryNamespaces(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func filterAIMemoryFactsByNamespaces(
	facts []*entity.AIMemoryFact,
	namespaces []string,
) []*entity.AIMemoryFact {
	if len(namespaces) == 0 || len(facts) == 0 {
		return facts
	}
	allowed := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		allowed[strings.TrimSpace(namespace)] = struct{}{}
	}
	filtered := make([]*entity.AIMemoryFact, 0, len(facts))
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		if _, ok := allowed[strings.TrimSpace(fact.Namespace)]; ok {
			filtered = append(filtered, fact)
		}
	}
	return filtered
}

func sortAIMemoryFacts(facts []*entity.AIMemoryFact) {
	sort.SliceStable(facts, func(i, j int) bool {
		return compareAIMemoryFacts(facts[i], facts[j])
	})
}
