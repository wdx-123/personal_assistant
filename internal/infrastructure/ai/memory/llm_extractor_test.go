package memory

import (
	"context"
	"errors"
	"strings"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	aidomain "personal_assistant/internal/domain/ai"
)

type fakeLLMExtractorChatModel struct {
	generateMsg    *schema.Message
	generateErr    error
	generateInputs [][]*schema.Message
}

func (m *fakeLLMExtractorChatModel) Generate(
	ctx context.Context,
	input []*schema.Message,
	opts ...einomodel.Option,
) (*schema.Message, error) {
	_ = opts
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	cloned := make([]*schema.Message, len(input))
	copy(cloned, input)
	m.generateInputs = append(m.generateInputs, cloned)
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	return m.generateMsg, nil
}

func (m *fakeLLMExtractorChatModel) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	_ = ctx
	_ = input
	_ = opts
	return schema.StreamReaderFromArray([]*schema.Message{}), nil
}

func TestLLMExtractorExtractsStructuredCandidates(t *testing.T) {
	model := &fakeLLMExtractorChatModel{
		generateMsg: schema.AssistantMessage(`{
			"summary": {
				"summary_text": "用户希望回答更简洁，并询问记忆写回设计。",
				"key_points": ["偏好简洁回答"],
				"open_loops": ["继续完善 writeback"]
			},
			"facts": [
				{
					"namespace": "user_preference",
					"fact_key": "answer_style",
					"value": "更简洁",
					"summary": "用户偏好简洁回答",
					"confidence": 0.92,
					"ttl_hint": {
						"kind": "persistent",
						"reason": "用户表达的是长期回答偏好",
						"confidence": 0.9
					}
				}
			],
			"documents": [
				{
					"memory_type": "semantic",
					"topic": "memory_writeback",
					"title": "记忆写回治理",
					"summary": "LLM 提议候选，policy 裁决。",
					"content": "Prompt 负责治理意图，Policy 负责治理裁决。",
					"confidence": 0.88,
					"ttl_hint": {
						"kind": "duration",
						"value": 30,
						"unit": "day",
						"reason": "示例知识阶段性有效",
						"confidence": 0.81
					}
				}
			]
		}`, nil),
	}
	extractor := newTestLLMExtractor(t, model)

	result, err := extractor.Extract(context.Background(), buildLLMExtractorTestInput())
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if result.Summary == nil || !strings.Contains(result.Summary.SummaryText, "更简洁") {
		t.Fatalf("summary = %#v, want concise preference", result.Summary)
	}
	if len(result.Facts) != 1 {
		t.Fatalf("facts len = %d, want 1", len(result.Facts))
	}
	fact := result.Facts[0]
	if fact.ScopeType != aidomain.MemoryScopeSelf || fact.SourceKind != aidomain.MemorySourceModelInferred {
		t.Fatalf("fact scope/source = %s/%s, want self/model_inferred", fact.ScopeType, fact.SourceKind)
	}
	if fact.Namespace != aidomain.MemoryNamespaceUserPreference || fact.FactKey != "answer_style" {
		t.Fatalf("fact = %#v, want user_preference answer_style", fact)
	}
	if fact.Confidence != 0.92 || fact.TTLHint == nil || fact.TTLHint.Kind != aidomain.MemoryTTLHintPersistent {
		t.Fatalf("fact confidence/ttl_hint = %v/%#v, want persistent hint", fact.Confidence, fact.TTLHint)
	}
	if len(result.Documents) != 1 {
		t.Fatalf("documents len = %d, want 1", len(result.Documents))
	}
	doc := result.Documents[0]
	if doc.ScopeType != aidomain.MemoryScopeSelf || doc.SourceKind != aidomain.MemorySourceModelInferred {
		t.Fatalf("doc scope/source = %s/%s, want self/model_inferred", doc.ScopeType, doc.SourceKind)
	}
	if doc.MemoryType != aidomain.MemoryTypeSemantic {
		t.Fatalf("doc memory_type = %q, want semantic", doc.MemoryType)
	}
	if doc.Confidence != 0.88 || doc.TTLHint == nil || doc.TTLHint.Value != 30 {
		t.Fatalf("doc confidence/ttl_hint = %v/%#v, want duration 30 days", doc.Confidence, doc.TTLHint)
	}
	if doc.ContentText != "Prompt 负责治理意图，Policy 负责治理裁决。" {
		t.Fatalf("doc content = %q, want condensed document content", doc.ContentText)
	}
}

func TestLLMExtractorPromptRequiresCondensedDocuments(t *testing.T) {
	extractor := &LLMExtractor{maxInputRunes: 200}

	prompt := extractor.buildPrompt(buildLLMExtractorTestInput())
	for _, want := range []string{
		"document 只用于可复用方案、设计、FAQ、runbook、排障步骤、项目经验",
		"不要复制整段 assistant answer",
		"如果没有长期可复用知识，documents 必须输出空数组 []",
		"清洗浓缩后的可语义召回正文",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
}

func TestLLMDocumentCandidatesTruncateLongContent(t *testing.T) {
	longContent := strings.Repeat("知", defaultLLMDocumentContentMaxRunes+20)

	docs := buildLLMDocumentCandidates(buildLLMExtractorTestInput(), []llmDocumentCandidate{
		{
			MemoryType: "semantic",
			Topic:      "memory_writeback",
			Title:      "记忆写回治理",
			Summary:    "长期知识摘要",
			Content:    longContent,
			Confidence: 0.9,
		},
	})

	if len(docs) != 1 {
		t.Fatalf("documents len = %d, want 1", len(docs))
	}
	if got := len([]rune(docs[0].ContentText)); got != defaultLLMDocumentContentMaxRunes {
		t.Fatalf("content runes = %d, want %d", got, defaultLLMDocumentContentMaxRunes)
	}
}

func TestLLMDocumentCandidatesMarkCopiedAssistantAnswerLowValue(t *testing.T) {
	input := buildLLMExtractorTestInput()
	input.AssistantMessage.Content = strings.Repeat("完整回答", 90)

	docs := buildLLMDocumentCandidates(input, []llmDocumentCandidate{
		{
			MemoryType: "semantic",
			Topic:      "memory_writeback",
			Title:      "记忆写回治理",
			Summary:    "长期知识摘要",
			Content:    input.AssistantMessage.Content,
			Confidence: 0.9,
		},
	})

	if len(docs) != 1 {
		t.Fatalf("documents len = %d, want 1", len(docs))
	}
	if !docs[0].LowValue {
		t.Fatal("document LowValue = false, want true for copied assistant answer")
	}
}

func TestLLMDocumentCandidatesUseSummaryFallbackAndSkipEmpty(t *testing.T) {
	docs := buildLLMDocumentCandidates(buildLLMExtractorTestInput(), []llmDocumentCandidate{
		{
			MemoryType: "semantic",
			Topic:      "memory_writeback",
			Title:      "记忆写回治理",
			Summary:    "摘要可作为兜底正文",
			Confidence: 0.9,
		},
		{
			MemoryType: "semantic",
			Topic:      "empty",
			Title:      "空文档",
			Summary:    " ",
			Content:    " ",
			Confidence: 0.9,
		},
	})

	if len(docs) != 1 {
		t.Fatalf("documents len = %d, want 1", len(docs))
	}
	if docs[0].ContentText != "摘要可作为兜底正文" {
		t.Fatalf("content fallback = %q, want summary", docs[0].ContentText)
	}
}

func TestLLMExtractorParsesFencedJSON(t *testing.T) {
	model := &fakeLLMExtractorChatModel{
		generateMsg: schema.AssistantMessage("```json\n{\"facts\":[],\"documents\":[]}\n```", nil),
	}
	extractor := newTestLLMExtractor(t, model)

	if _, err := extractor.Extract(context.Background(), buildLLMExtractorTestInput()); err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
}

func TestLLMExtractorRejectsInvalidJSON(t *testing.T) {
	model := &fakeLLMExtractorChatModel{
		generateMsg: schema.AssistantMessage("not-json", nil),
	}
	extractor := newTestLLMExtractor(t, model)

	if _, err := extractor.Extract(context.Background(), buildLLMExtractorTestInput()); err == nil {
		t.Fatal("Extract() error = nil, want invalid json error")
	}
}

func TestLLMExtractorRejectsEmptyOutput(t *testing.T) {
	model := &fakeLLMExtractorChatModel{
		generateMsg: schema.AssistantMessage("", nil),
	}
	extractor := newTestLLMExtractor(t, model)

	if _, err := extractor.Extract(context.Background(), buildLLMExtractorTestInput()); err == nil {
		t.Fatal("Extract() error = nil, want empty output error")
	}
}

func TestLLMExtractorRespectsCanceledContext(t *testing.T) {
	model := &fakeLLMExtractorChatModel{
		generateMsg: schema.AssistantMessage(`{"facts":[],"documents":[]}`, nil),
	}
	extractor := newTestLLMExtractor(t, model)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := extractor.Extract(ctx, buildLLMExtractorTestInput())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Extract() error = %v, want context.Canceled", err)
	}
	if len(model.generateInputs) != 0 {
		t.Fatalf("generateInputs len = %d, want 0", len(model.generateInputs))
	}
}

func newTestLLMExtractor(t *testing.T, model *fakeLLMExtractorChatModel) *LLMExtractor {
	t.Helper()
	extractor, err := NewLLMExtractor(context.Background(), LLMExtractorOptions{
		ChatModel:     model,
		MaxInputChars: 200,
	})
	if err != nil {
		t.Fatalf("NewLLMExtractor() error = %v", err)
	}
	return extractor
}

func buildLLMExtractorTestInput() aidomain.MemoryExtractionInput {
	return aidomain.MemoryExtractionInput{
		ConversationID: "conv-llm",
		UserID:         7,
		UserMessage: aidomain.Message{
			ID:      "msg-user-llm",
			Role:    aidomain.RoleUser,
			Content: "请记住以后回答更简洁，并解释 memory writeback hook。",
		},
		AssistantMessage: aidomain.Message{
			ID:      "msg-ai-llm",
			Role:    aidomain.RoleAssistant,
			Content: "Prompt 是治理意图，Policy 是治理裁决。",
		},
	}
}
