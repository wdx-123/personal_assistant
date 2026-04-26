package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
)

const defaultDashScopeEmbeddingEndpoint = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"

// EmbedderOptions 配置 DashScope multimodal embedding 客户端。
type EmbedderOptions struct {
	APIKey    string
	Endpoint  string
	Model     string
	Dimension int
	Timeout   time.Duration
	Client    *http.Client
}

// DashScopeEmbedder 调用阿里云百炼 multimodal embedding 接口。
type DashScopeEmbedder struct {
	apiKey    string
	endpoint  string
	model     string
	dimension int
	client    *http.Client
}

// NewDashScopeEmbedder 创建 DashScope embedding 客户端。
func NewDashScopeEmbedder(opts EmbedderOptions) *DashScopeEmbedder {
	if strings.TrimSpace(opts.Endpoint) == "" {
		opts.Endpoint = defaultDashScopeEmbeddingEndpoint
	}
	if opts.Client == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		opts.Client = &http.Client{Timeout: timeout}
	}
	return &DashScopeEmbedder{
		apiKey:    strings.TrimSpace(opts.APIKey),
		endpoint:  strings.TrimSpace(opts.Endpoint),
		model:     strings.TrimSpace(opts.Model),
		dimension: opts.Dimension,
		client:    opts.Client,
	}
}

// Embed 为输入文本生成向量。
func (e *DashScopeEmbedder) Embed(
	ctx context.Context,
	input aidomain.MemoryEmbeddingInput,
) (aidomain.MemoryEmbeddingResult, error) {
	if e == nil {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedder is nil")
	}
	if e.apiKey == "" {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding api key is required")
	}
	if e.model == "" {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding model is required")
	}
	if e.dimension <= 0 {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding dimension must be positive")
	}
	texts := normalizeEmbeddingTexts(input.Texts)
	if len(texts) == 0 {
		return aidomain.MemoryEmbeddingResult{}, nil
	}

	reqBody := dashScopeEmbeddingRequest{
		Model: e.model,
		Input: dashScopeEmbeddingInput{
			Contents: make([]dashScopeEmbeddingContent, 0, len(texts)),
		},
		Parameters: dashScopeEmbeddingParameters{Dimension: e.dimension},
	}
	for _, text := range texts {
		reqBody.Input.Contents = append(reqBody.Input.Contents, dashScopeEmbeddingContent{Text: text})
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return aidomain.MemoryEmbeddingResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(payload))
	if err != nil {
		return aidomain.MemoryEmbeddingResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return aidomain.MemoryEmbeddingResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return aidomain.MemoryEmbeddingResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding failed: status=%d body=%s", resp.StatusCode, truncateEmbeddingErrorBody(string(body)))
	}

	var parsed dashScopeEmbeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return aidomain.MemoryEmbeddingResult{}, err
	}
	if len(parsed.Output.Embeddings) != len(texts) {
		return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding count = %d, want %d", len(parsed.Output.Embeddings), len(texts))
	}
	vectors := make([][]float32, len(texts))
	for _, item := range parsed.Output.Embeddings {
		if item.Index < 0 || item.Index >= len(texts) {
			return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding index out of range: %d", item.Index)
		}
		if len(item.Embedding) != e.dimension {
			return aidomain.MemoryEmbeddingResult{}, fmt.Errorf("dashscope embedding dimension = %d, want %d", len(item.Embedding), e.dimension)
		}
		vector := make([]float32, len(item.Embedding))
		for i, value := range item.Embedding {
			vector[i] = float32(value)
		}
		vectors[item.Index] = vector
	}
	return aidomain.MemoryEmbeddingResult{Vectors: vectors}, nil
}

type dashScopeEmbeddingRequest struct {
	Model      string                       `json:"model"`
	Input      dashScopeEmbeddingInput      `json:"input"`
	Parameters dashScopeEmbeddingParameters `json:"parameters"`
}

type dashScopeEmbeddingInput struct {
	Contents []dashScopeEmbeddingContent `json:"contents"`
}

type dashScopeEmbeddingContent struct {
	Text string `json:"text"`
}

type dashScopeEmbeddingParameters struct {
	Dimension int `json:"dimension"`
}

type dashScopeEmbeddingResponse struct {
	Output dashScopeEmbeddingOutput `json:"output"`
}

type dashScopeEmbeddingOutput struct {
	Embeddings []dashScopeEmbeddingItem `json:"embeddings"`
}

type dashScopeEmbeddingItem struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

func normalizeEmbeddingTexts(texts []string) []string {
	items := make([]string, 0, len(texts))
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text != "" {
			items = append(items, text)
		}
	}
	return items
}

func truncateEmbeddingErrorBody(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 500 {
		return value
	}
	return value[:500]
}
