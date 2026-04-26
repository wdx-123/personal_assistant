package ai

import "context"

// MemoryDocumentForIndex 是 RAG 索引器消费的长期记忆文档快照。
type MemoryDocumentForIndex struct {
	ID         string
	ScopeKey   string
	ScopeType  string
	Visibility string
	UserID     *uint
	OrgID      *uint
	MemoryType string
	Topic      string
	Title      string
	Summary    string
	Content    string
	SourceKind string
	SourceID   string
}

// MemoryDocumentChunk 表示切分后可索引的记忆片段。
type MemoryDocumentChunk struct {
	ID                 string
	DocumentID         string
	ScopeKey           string
	ScopeType          string
	Visibility         string
	UserID             *uint
	OrgID              *uint
	MemoryType         string
	Topic              string
	SourceKind         string
	SourceID           string
	ChunkIndex         int
	ContentText        string
	ContentHash        string
	TokenEstimate      int
	EmbeddingModel     string
	EmbeddingDimension int
	QdrantPointID      string
}

// MemoryEmbeddingInput 描述一次 embedding 请求。
type MemoryEmbeddingInput struct {
	Texts []string
}

// MemoryEmbeddingResult 描述 embedding 输出。
type MemoryEmbeddingResult struct {
	Vectors [][]float32
}

// MemoryVectorChunk 表示准备写入向量库的 chunk。
type MemoryVectorChunk struct {
	Chunk  MemoryDocumentChunk
	Vector []float32
}

// MemoryVectorSearchInput 描述一次 memory vector 检索请求。
type MemoryVectorSearchInput struct {
	Vector     []float32
	ScopeKey   string
	Visibility string
	UserID     uint
	Limit      int
	MinScore   float64
}

// MemoryVectorSearchResult 描述 Qdrant 返回的候选 chunk。
type MemoryVectorSearchResult struct {
	QdrantPointID string
	ChunkID       string
	DocumentID    string
	Score         float64
}

// MemoryChunker 负责把长期记忆文档切分成可 embedding 的 chunks。
type MemoryChunker interface {
	Chunk(ctx context.Context, doc MemoryDocumentForIndex) ([]MemoryDocumentChunk, error)
}

// MemoryEmbedder 负责为 chunk 文本生成向量。
type MemoryEmbedder interface {
	Embed(ctx context.Context, input MemoryEmbeddingInput) (MemoryEmbeddingResult, error)
}

// MemoryVectorStore 负责把 memory chunks 写入向量库。
type MemoryVectorStore interface {
	DeleteDocumentChunks(ctx context.Context, documentID string) error
	UpsertChunks(ctx context.Context, chunks []MemoryVectorChunk) error
}

// MemoryVectorSearcher 负责按 query vector 从向量库召回候选 chunks。
type MemoryVectorSearcher interface {
	SearchChunks(ctx context.Context, input MemoryVectorSearchInput) ([]MemoryVectorSearchResult, error)
}

// MemoryDocumentIndexer 定义 memory documents 的索引建设能力。
type MemoryDocumentIndexer interface {
	IndexDocuments(ctx context.Context, documentIDs []string) error
	IndexPendingDocuments(ctx context.Context, limit int) error
}
