package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"personal_assistant/global"
	aidomain "personal_assistant/internal/domain/ai"
	aimemory "personal_assistant/internal/infrastructure/ai/memory"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
)

var errAIMemoryPhase1NotImplemented = errors.New("ai memory phase 1 skeleton is not integrated yet")

type aiMemoryRecallResult struct {
	// PromptBlocks 预留给后续把记忆整理成可拼接到 prompt 的文本块。
	PromptBlocks []string
	// Messages 预留给后续直接注入 runtime history 的记忆消息片段。
	Messages []aidomain.Message
}

type aiMemoryWritebackInput struct {
	// ConversationID 标识当前完成写回的会话。
	ConversationID string
	// UserID 表示这次写回归属于哪个用户。
	UserID uint
	// OrgID 表示这次写回发生时的组织上下文；为空时表示个人会话。
	OrgID *uint
	// UserMessageID 是触发本轮回答的用户消息 ID。
	UserMessageID string
	// AssistantMessageID 是本轮完成写回的 assistant 消息 ID。
	AssistantMessageID string
	// Principal 是本轮 AI tool 链路已经解析出的授权事实。
	Principal aidomain.AIToolPrincipal
}

// AIMemoryService 收口记忆模块的仓储依赖和后续扩展点。
type AIMemoryService struct {
	// aiRepo 用于读取已落库的会话消息快照。
	aiRepo interfaces.AIRepository
	// repo 是记忆模块的正式持久化入口。
	repo interfaces.AIMemoryRepository
	// outboxRepo 为后续异步 document upsert 预留，本阶段不实际投递 memory outbox 事件。
	outboxRepo interfaces.OutboxRepository
	// policy 预留给下一步记忆治理规则。
	policy aiMemoryPolicy
	// extractor 负责从完成轮次中抽取候选记忆。
	extractor aidomain.MemoryExtractor
	// chunker 负责把长期记忆文档切成可向量化片段。
	chunker aidomain.MemoryChunker
	// embedder 负责为 chunk 文本生成向量。
	embedder aidomain.MemoryEmbedder
	// vectorStore 负责把 chunk 向量写入 Qdrant。
	vectorStore aidomain.MemoryVectorStore
	// vectorSearcher 负责从 Qdrant 召回 memory chunks。
	vectorSearcher aidomain.MemoryVectorSearcher
}

// NewAIMemoryService 基于正式 repository group 构造记忆服务骨架。
func NewAIMemoryService(repositoryGroup *repository.Group) *AIMemoryService {
	if repositoryGroup == nil || repositoryGroup.SystemRepositorySupplier == nil {
		// Phase 1 先保证骨架可安全构造，不把 memory 变成启动期硬依赖。
		return &AIMemoryService{}
	}
	qdrantStore := newAIMemoryQdrantStore()
	return &AIMemoryService{
		aiRepo:     repositoryGroup.SystemRepositorySupplier.GetAIRepository(),
		repo:       repositoryGroup.SystemRepositorySupplier.GetAIMemoryRepository(),
		outboxRepo: repositoryGroup.SystemRepositorySupplier.GetOutboxRepository(),
		policy:     aiMemoryPolicy{},
		extractor:  aimemory.NewRuleExtractor(aimemory.Options{}),
		chunker: aimemory.NewParagraphChunker(aimemory.ChunkerOptions{
			MaxChars:     aiMemoryChunkMaxChars(),
			OverlapChars: aiMemoryChunkOverlapChars(),
		}),
		embedder: aimemory.NewDashScopeEmbedder(aimemory.EmbedderOptions{
			APIKey:    aiMemoryAPIKey(),
			Endpoint:  aiMemoryEmbedEndpoint(),
			Model:     aiMemoryEmbedModel(),
			Dimension: aiMemoryEmbedDimension(),
			Timeout:   time.Duration(aiMemoryIndexTimeoutSeconds()) * time.Second,
		}),
		vectorStore:    qdrantStore,
		vectorSearcher: qdrantStore,
	}
}

// RefreshConversationSummary 预留给后续 summary 刷新流程。
func (s *AIMemoryService) RefreshConversationSummary(ctx context.Context, conversationID string) error {
	_ = ctx
	if strings.TrimSpace(conversationID) == "" {
		// 空会话 ID 不触发任何动作，避免后续接主链路时引入无意义写放大。
		return nil
	}
	return errAIMemoryPhase1NotImplemented
}

// UpsertFact 直接透传到仓储层，供后续流程复用。
func (s *AIMemoryService) UpsertFact(ctx context.Context, fact *entity.AIMemoryFact) error {
	if s == nil || s.repo == nil || fact == nil {
		return nil
	}
	return s.repo.UpsertFact(ctx, fact)
}

// ScheduleDocumentUpsert 当前阶段不走 outbox，直接透传到仓储层批量 upsert。
func (s *AIMemoryService) ScheduleDocumentUpsert(ctx context.Context, docs []*entity.AIMemoryDocument) error {
	if s == nil || s.repo == nil || len(docs) == 0 {
		return nil
	}
	if err := s.repo.BatchUpsertDocuments(ctx, docs); err != nil {
		return err
	}
	s.triggerDocumentIndex(ctx, docs)
	return nil
}

func newAIMemoryVectorStore() aidomain.MemoryVectorStore {
	return newAIMemoryQdrantStore()
}

func newAIMemoryQdrantStore() *aimemory.QdrantVectorStore {
	if global.QdrantClient == nil {
		return nil
	}
	return aimemory.NewQdrantVectorStore(global.QdrantClient, aimemory.VectorStoreOptions{
		CollectionName: aiMemoryCollectionName(),
	})
}
