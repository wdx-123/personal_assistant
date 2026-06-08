package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
	infraeino "personal_assistant/internal/infrastructure/ai/eino"
)

const (
	defaultLLMExtractorTimeout        = 20 * time.Second
	defaultLLMExtractorMaxInputRunes  = 6000
	defaultLLMDocumentContentMaxRunes = 1200
	copiedAssistantDocumentMinRunes   = 240
)

// LLMExtractorOptions configures the memory candidate extractor without reading global config.
type LLMExtractorOptions struct {
	Provider            string
	APIKey              string
	BaseURL             string
	Model               string
	ByAzure             bool
	APIVersion          string
	SystemPrompt        string
	Temperature         float64
	MaxCompletionTokens int
	Timeout             time.Duration
	MaxInputChars       int
	ChatModel           einomodel.BaseChatModel
}

// LLMExtractor asks a chat model to propose memory candidates; policy code still makes final decisions.
type LLMExtractor struct {
	model         einomodel.BaseChatModel
	systemPrompt  string
	timeout       time.Duration
	maxInputRunes int
}

// NewLLMExtractor creates a real LLM-backed memory extractor.
func NewLLMExtractor(ctx context.Context, opts LLMExtractorOptions) (*LLMExtractor, error) {
	model := opts.ChatModel
	if model == nil {
		created, err := infraeino.NewChatModel(ctx, infraeino.Options{
			Provider:            opts.Provider,
			APIKey:              opts.APIKey,
			BaseURL:             opts.BaseURL,
			Model:               opts.Model,
			ByAzure:             opts.ByAzure,
			APIVersion:          opts.APIVersion,
			Temperature:         opts.Temperature,
			MaxCompletionTokens: opts.MaxCompletionTokens,
		})
		if err != nil {
			return nil, err
		}
		model = created
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultLLMExtractorTimeout
	}
	maxInputRunes := opts.MaxInputChars
	if maxInputRunes <= 0 {
		maxInputRunes = defaultLLMExtractorMaxInputRunes
	}
	systemPrompt := strings.TrimSpace(opts.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "你是 personal_assistant 的记忆候选提议器。你只输出 JSON，不输出解释。"
	}

	return &LLMExtractor{
		model:         model,
		systemPrompt:  systemPrompt,
		timeout:       timeout,
		maxInputRunes: maxInputRunes,
	}, nil
}

// Extract asks the model for memory candidates and converts them to domain proposals.
func (e *LLMExtractor) Extract(
	ctx context.Context,
	input aidomain.MemoryExtractionInput,
) (aidomain.MemoryExtractionResult, error) {
	if e == nil || e.model == nil {
		return aidomain.MemoryExtractionResult{}, fmt.Errorf("llm memory extractor model is nil")
	}
	select {
	case <-ctx.Done():
		return aidomain.MemoryExtractionResult{}, ctx.Err()
	default:
	}

	callCtx := ctx
	cancel := func() {}
	if e.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, e.timeout)
	}
	defer cancel()

	msg, err := e.model.Generate(callCtx, []*schema.Message{
		schema.SystemMessage(e.systemPrompt),
		schema.UserMessage(e.buildPrompt(input)),
	})
	if err != nil {
		return aidomain.MemoryExtractionResult{}, err
	}
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return aidomain.MemoryExtractionResult{}, fmt.Errorf("llm memory extractor returned empty content")
	}

	var output llmMemoryExtractionOutput
	if err := unmarshalLLMExtractorJSON(msg.Content, &output); err != nil {
		return aidomain.MemoryExtractionResult{}, err
	}
	return e.toExtractionResult(input, output), nil
}

func (e *LLMExtractor) buildPrompt(input aidomain.MemoryExtractionInput) string {
	payload := llmMemoryPromptPayload{
		ConversationID:      input.ConversationID,
		UserID:              input.UserID,
		PreviousSummaryText: truncateRunes(input.PreviousSummaryText, e.maxInputRunes),
		SummaryRefreshMode:  string(input.SummaryRefreshMode),
		UserMessage: llmMemoryPromptMessage{
			ID:      input.UserMessage.ID,
			Role:    input.UserMessage.Role,
			Content: truncateRunes(input.UserMessage.Content, e.maxInputRunes),
		},
		AssistantMessage: llmMemoryPromptMessage{
			ID:      input.AssistantMessage.ID,
			Role:    input.AssistantMessage.Role,
			Content: truncateRunes(input.AssistantMessage.Content, e.maxInputRunes),
		},
		RecentMessages: buildLLMRecentPromptMessages(input.RecentMessages, e.maxInputRunes),
	}
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	return strings.TrimSpace(fmt.Sprintf(`
你需要从一次已完成的用户/助手对话中提议候选记忆。

治理边界：
1. 你只负责提议候选，不决定最终 scope、visibility、TTL、dedup key、覆盖和落库。
2. 第一版只允许提议当前用户自己的 self 记忆，不要提议 org 或 platform_ops。
3. 不要记录原始 trace、完整工具输出、完整日志、中间推理、闲聊和瞬时状态数字。
4. 只把稳定偏好、阶段目标、用户画像、可复用知识摘要提议为记忆。
5. ttl_hint 只能输出受控时间语义，不要输出最终 expires_at。
6. 只输出 JSON 对象，不要输出 markdown、不要输出解释。
7. summary_refresh_mode=head_update 时，重点产出最新 decisions 和 open loops，不要重写整段长期背景。
8. summary_refresh_mode=full_refresh 时，允许基于 previous_summary_text + recent_messages 重建完整摘要。

document 提取规则：
1. document 只用于可复用方案、设计、FAQ、runbook、排障步骤、项目经验。
2. 不要把用户偏好、阶段目标、会话主线、open loops 放进 document；这些分别属于 facts 或 summary。
3. content 必须是清洗浓缩后的可语义召回正文，需要去掉寒暄、过渡句、本轮上下文依赖和临场解释。
4. 不要复制整段 assistant answer；只保留可以脱离本轮对话独立理解的长期知识。
5. 如果没有长期可复用知识，documents 必须输出空数组 []。

输出 JSON schema：
{
  "summary": {
    "summary_text": "可选，会话摘要",
    "key_points": ["可选，关键点"],
    "open_loops": ["可选，待跟进事项"]
  },
  "facts": [
    {
      "namespace": "user_preference | oj_profile | oj_goal",
      "fact_key": "稳定的 snake_case key",
      "value": "简洁事实值",
      "summary": "可选，人类可读摘要",
      "confidence": 0.0,
      "ttl_hint": {
        "kind": "default | persistent | duration | until_date | session_only",
        "value": 30,
        "unit": "day",
        "until_date": "YYYY-MM-DD，仅 kind=until_date 时填写",
        "reason": "可选，简短解释时间语义来源",
        "confidence": 0.0
      }
    }
  ],
  "documents": [
    {
      "memory_type": "semantic | procedural | faq | episodic",
      "topic": "简短主题",
      "title": "标题",
      "summary": "摘要",
      "content": "清洗浓缩后的可语义召回正文，建议 300-800 字，最长 1200 字符",
      "confidence": 0.0,
      "ttl_hint": {
        "kind": "default | persistent | duration | until_date | session_only",
        "value": 30,
        "unit": "day",
        "until_date": "YYYY-MM-DD，仅 kind=until_date 时填写",
        "reason": "可选，简短解释时间语义来源",
        "confidence": 0.0
      }
    }
  ]
}

待分析输入：
%s`, string(payloadJSON)))
}

func (e *LLMExtractor) toExtractionResult(
	input aidomain.MemoryExtractionInput,
	output llmMemoryExtractionOutput,
) aidomain.MemoryExtractionResult {
	result := aidomain.MemoryExtractionResult{
		Summary:   buildLLMSummaryDraft(input, output.Summary),
		Facts:     buildLLMFactCandidates(input, output.Facts),
		Documents: buildLLMDocumentCandidates(input, output.Documents),
	}
	return result
}

type llmMemoryPromptPayload struct {
	ConversationID      string                   `json:"conversation_id"`
	UserID              uint                     `json:"user_id"`
	PreviousSummaryText string                   `json:"previous_summary_text"`
	SummaryRefreshMode  string                   `json:"summary_refresh_mode"`
	UserMessage         llmMemoryPromptMessage   `json:"user_message"`
	AssistantMessage    llmMemoryPromptMessage   `json:"assistant_message"`
	RecentMessages      []llmMemoryPromptMessage `json:"recent_messages,omitempty"`
}

type llmMemoryPromptMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmMemoryExtractionOutput struct {
	Summary   *llmSummaryCandidate   `json:"summary"`
	Facts     []llmFactCandidate     `json:"facts"`
	Documents []llmDocumentCandidate `json:"documents"`
}

type llmSummaryCandidate struct {
	SummaryText string   `json:"summary_text"`
	KeyPoints   []string `json:"key_points"`
	OpenLoops   []string `json:"open_loops"`
}

type llmFactCandidate struct {
	Namespace  string      `json:"namespace"`
	FactKey    string      `json:"fact_key"`
	Value      string      `json:"value"`
	Summary    string      `json:"summary"`
	Confidence float64     `json:"confidence"`
	TTLHint    *llmTTLHint `json:"ttl_hint"`
}

type llmDocumentCandidate struct {
	MemoryType string      `json:"memory_type"`
	Topic      string      `json:"topic"`
	Title      string      `json:"title"`
	Summary    string      `json:"summary"`
	Content    string      `json:"content"`
	Confidence float64     `json:"confidence"`
	TTLHint    *llmTTLHint `json:"ttl_hint"`
}

type llmTTLHint struct {
	Kind       string  `json:"kind"`
	Value      int     `json:"value"`
	Unit       string  `json:"unit"`
	UntilDate  string  `json:"until_date"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

func buildLLMSummaryDraft(
	input aidomain.MemoryExtractionInput,
	candidate *llmSummaryCandidate,
) *aidomain.ConversationSummaryDraft {
	if candidate == nil || strings.TrimSpace(candidate.SummaryText) == "" {
		return nil
	}
	keyPoints, _ := json.Marshal(normalizeLLMStringList(candidate.KeyPoints))
	openLoops, _ := json.Marshal(normalizeLLMStringList(candidate.OpenLoops))
	summaryText := normalizeText(candidate.SummaryText)
	return &aidomain.ConversationSummaryDraft{
		ConversationID:           input.ConversationID,
		CompressedUntilMessageID: input.AssistantMessage.ID,
		SummaryText:              summaryText,
		KeyPointsJSON:            string(keyPoints),
		OpenLoopsJSON:            string(openLoops),
		TokenEstimate:            estimateTokens(summaryText),
	}
}

func buildLLMFactCandidates(
	input aidomain.MemoryExtractionInput,
	candidates []llmFactCandidate,
) []aidomain.MemoryFactCandidate {
	if input.UserID == 0 || len(candidates) == 0 {
		return nil
	}
	userID := input.UserID
	facts := make([]aidomain.MemoryFactCandidate, 0, len(candidates))
	for _, item := range candidates {
		namespace := normalizeLLMFactNamespace(item.Namespace)
		factKey := normalizeLLMFactKey(item.FactKey)
		value := truncateRunes(normalizeText(item.Value), 500)
		if namespace == "" || factKey == "" || value == "" {
			continue
		}
		payload, _ := json.Marshal(map[string]string{"value": value})
		summary := truncateRunes(normalizeText(firstNonEmptyMemoryString(item.Summary, value)), 500)
		facts = append(facts, aidomain.MemoryFactCandidate{
			ScopeType:     aidomain.MemoryScopeSelf,
			UserID:        &userID,
			Namespace:     namespace,
			FactKey:       factKey,
			FactValueJSON: string(payload),
			Summary:       summary,
			Confidence:    item.Confidence,
			TTLHint:       buildLLMTTLHint(item.TTLHint),
			SourceKind:    aidomain.MemorySourceModelInferred,
			SourceID:      input.AssistantMessage.ID,
			LowValue:      len([]rune(value)) < 2 || isLowConfidenceLLMValue(item.Confidence) || isSessionOnlyLLMHint(item.TTLHint),
		})
	}
	return facts
}

func buildLLMDocumentCandidates(
	input aidomain.MemoryExtractionInput,
	candidates []llmDocumentCandidate,
) []aidomain.MemoryDocumentCandidate {
	if input.UserID == 0 || len(candidates) == 0 {
		return nil
	}
	userID := input.UserID
	docs := make([]aidomain.MemoryDocumentCandidate, 0, len(candidates))
	for _, item := range candidates {
		rawContent := normalizeText(item.Content)
		summary := truncateRunes(normalizeText(item.Summary), defaultDocumentSummaryMaxRunes)
		content := truncateRunes(rawContent, defaultLLMDocumentContentMaxRunes)
		if content == "" {
			content = summary
		}
		if content == "" {
			continue
		}
		copiedAssistant := isCopiedAssistantDocumentContent(rawContent, input.AssistantMessage.Content)
		docs = append(docs, aidomain.MemoryDocumentCandidate{
			ScopeType:   aidomain.MemoryScopeSelf,
			UserID:      &userID,
			MemoryType:  normalizeLLMMemoryType(item.MemoryType),
			Topic:       truncateRunes(normalizeText(firstNonEmptyMemoryString(item.Topic, "conversation_knowledge")), 120),
			Title:       truncateRunes(normalizeText(firstNonEmptyMemoryString(item.Title, item.Topic)), 120),
			Summary:     summary,
			ContentText: content,
			Confidence:  item.Confidence,
			TTLHint:     buildLLMTTLHint(item.TTLHint),
			SourceKind:  aidomain.MemorySourceModelInferred,
			SourceID:    input.AssistantMessage.ID,
			LowValue:    (len([]rune(content)) < 20 && len([]rune(summary)) < 10) || copiedAssistant || isLowConfidenceLLMValue(item.Confidence) || isSessionOnlyLLMHint(item.TTLHint),
		})
	}
	return docs
}

func unmarshalLLMExtractorJSON(raw string, target any) error {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(strings.TrimSpace(trimmed), "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	if trimmed == "" {
		return fmt.Errorf("llm memory extractor output is empty")
	}
	if err := json.Unmarshal([]byte(trimmed), target); err != nil {
		return fmt.Errorf("invalid llm memory extractor json: %w", err)
	}
	return nil
}

func normalizeLLMFactNamespace(namespace string) string {
	switch strings.TrimSpace(namespace) {
	case aidomain.MemoryNamespaceUserPreference:
		return aidomain.MemoryNamespaceUserPreference
	case aidomain.MemoryNamespaceOJProfile:
		return aidomain.MemoryNamespaceOJProfile
	case aidomain.MemoryNamespaceOJGoal:
		return aidomain.MemoryNamespaceOJGoal
	default:
		return ""
	}
}

func normalizeLLMMemoryType(memoryType string) aidomain.MemoryType {
	switch aidomain.MemoryType(strings.TrimSpace(memoryType)) {
	case aidomain.MemoryTypeSemantic:
		return aidomain.MemoryTypeSemantic
	case aidomain.MemoryTypeProcedural:
		return aidomain.MemoryTypeProcedural
	case aidomain.MemoryTypeFAQ:
		return aidomain.MemoryTypeFAQ
	case aidomain.MemoryTypeEpisodic:
		return aidomain.MemoryTypeEpisodic
	default:
		return aidomain.MemoryTypeSemantic
	}
}

func buildLLMTTLHint(input *llmTTLHint) *aidomain.MemoryTTLHint {
	if input == nil {
		return nil
	}
	kind := aidomain.MemoryTTLHintKind(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = aidomain.MemoryTTLHintDefault
	}
	return &aidomain.MemoryTTLHint{
		Kind:       kind,
		Value:      input.Value,
		Unit:       strings.TrimSpace(input.Unit),
		UntilDate:  strings.TrimSpace(input.UntilDate),
		Reason:     truncateRunes(normalizeText(input.Reason), 240),
		Confidence: input.Confidence,
	}
}

func isLowConfidenceLLMValue(confidence float64) bool {
	return confidence > 0 && confidence < 0.5
}

func isSessionOnlyLLMHint(input *llmTTLHint) bool {
	return input != nil && strings.TrimSpace(input.Kind) == string(aidomain.MemoryTTLHintSessionOnly)
}

func isCopiedAssistantDocumentContent(content string, assistantContent string) bool {
	content = normalizeText(content)
	if len([]rune(content)) <= copiedAssistantDocumentMinRunes {
		return false
	}
	return content == normalizeText(assistantContent)
}

func normalizeLLMFactKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), "_")
}

func normalizeLLMStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeText(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmptyMemoryString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildLLMRecentPromptMessages(messages []aidomain.Message, maxInputRunes int) []llmMemoryPromptMessage {
	if len(messages) == 0 {
		return nil
	}
	items := make([]llmMemoryPromptMessage, 0, len(messages))
	for _, message := range messages {
		content := truncateRunes(message.Content, maxInputRunes)
		if strings.TrimSpace(content) == "" {
			continue
		}
		items = append(items, llmMemoryPromptMessage{
			ID:      message.ID,
			Role:    message.Role,
			Content: content,
		})
	}
	return items
}
