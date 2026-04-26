package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aidomain "personal_assistant/internal/domain/ai"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIMemoryGormRepository 基于 GORM 实现记忆仓储。
type AIMemoryGormRepository struct {
	db *gorm.DB
}

// NewAIMemoryRepository 创建记忆仓储实例。
func NewAIMemoryRepository(db *gorm.DB) interfaces.AIMemoryRepository {
	return &AIMemoryGormRepository{db: db}
}

// WithTx 绑定事务上下文并返回新的仓储实例。
func (r *AIMemoryGormRepository) WithTx(tx any) interfaces.AIMemoryRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &AIMemoryGormRepository{db: transaction}
	}
	return r
}

// UpsertFact 按唯一键覆盖更新结构化事实。
func (r *AIMemoryGormRepository) UpsertFact(ctx context.Context, fact *entity.AIMemoryFact) error {
	if fact == nil {
		return nil
	}
	now := time.Now()
	// facts 的真相语义是“当前有效值”，不是历史事件流。
	// 因此这里采用唯一键覆盖，而不是先删后插或保留多版本并存。
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "scope_key"},
				{Name: "namespace"},
				{Name: "fact_key"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"scope_type":      fact.ScopeType,
				"visibility":      fact.Visibility,
				"user_id":         fact.UserID,
				"org_id":          fact.OrgID,
				"fact_value_json": fact.FactValueJSON,
				"summary":         fact.Summary,
				"confidence":      fact.Confidence,
				"source_kind":     fact.SourceKind,
				"source_id":       fact.SourceID,
				"effective_at":    fact.EffectiveAt,
				"expires_at":      fact.ExpiresAt,
				"updated_at":      now,
			}),
		}).
		Create(fact).Error
}

// ListFacts 按默认规则过滤过期 facts。
func (r *AIMemoryGormRepository) ListFacts(
	ctx context.Context,
	query aidomain.MemoryFactQuery,
) ([]*entity.AIMemoryFact, error) {
	if len(query.ScopeKeys) == 0 || len(query.AllowedVisibilities) == 0 {
		// 调用方没有给出完整授权边界时，仓储层直接返回空结果，避免误查全表。
		return []*entity.AIMemoryFact{}, nil
	}
	var rows []*entity.AIMemoryFact
	now := time.Now()
	db := r.db.WithContext(ctx).
		Model(&entity.AIMemoryFact{}).
		Where("scope_key IN ?", query.ScopeKeys).
		Where("visibility IN ?", aidomain.NormalizeMemoryVisibilities(query.AllowedVisibilities)).
		Where("(expires_at IS NULL OR expires_at > ?)", now)
	if query.Namespace != "" {
		db = db.Where("namespace = ?", query.Namespace)
	}
	if len(query.FactKeys) > 0 {
		db = db.Where("fact_key IN ?", query.FactKeys)
	}
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}
	if err := db.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// BatchUpsertDocuments 批量 upsert 文档元数据。
func (r *AIMemoryGormRepository) BatchUpsertDocuments(
	ctx context.Context,
	docs []*entity.AIMemoryDocument,
) error {
	if len(docs) == 0 {
		return nil
	}
	normalizedDocs, err := r.prepareDocumentUpserts(ctx, docs)
	if err != nil {
		return err
	}
	if len(normalizedDocs) == 0 {
		return nil
	}
	now := time.Now()
	// documents 后续会进入 embedding / vector store 流程，因此这里保留“按文档 ID 重写元数据”的能力。
	// 如果旧文档曾被软删除，再次 upsert 时会自动恢复 deleted_at。
	assignments := clause.Set{
		{Column: clause.Column{Name: "scope_key"}, Value: clause.Column{Table: "excluded", Name: "scope_key"}},
		{Column: clause.Column{Name: "scope_type"}, Value: clause.Column{Table: "excluded", Name: "scope_type"}},
		{Column: clause.Column{Name: "visibility"}, Value: clause.Column{Table: "excluded", Name: "visibility"}},
		{Column: clause.Column{Name: "user_id"}, Value: clause.Column{Table: "excluded", Name: "user_id"}},
		{Column: clause.Column{Name: "org_id"}, Value: clause.Column{Table: "excluded", Name: "org_id"}},
		{Column: clause.Column{Name: "memory_type"}, Value: clause.Column{Table: "excluded", Name: "memory_type"}},
		{Column: clause.Column{Name: "topic"}, Value: clause.Column{Table: "excluded", Name: "topic"}},
		{Column: clause.Column{Name: "title"}, Value: clause.Column{Table: "excluded", Name: "title"}},
		{Column: clause.Column{Name: "summary"}, Value: clause.Column{Table: "excluded", Name: "summary"}},
		{Column: clause.Column{Name: "content_text"}, Value: clause.Column{Table: "excluded", Name: "content_text"}},
		{Column: clause.Column{Name: "content_hash"}, Value: clause.Column{Table: "excluded", Name: "content_hash"}},
		{Column: clause.Column{Name: "summary_hash"}, Value: clause.Column{Table: "excluded", Name: "summary_hash"}},
		{Column: clause.Column{Name: "dedup_key"}, Value: clause.Column{Table: "excluded", Name: "dedup_key"}},
		{Column: clause.Column{Name: "importance"}, Value: clause.Column{Table: "excluded", Name: "importance"}},
		{Column: clause.Column{Name: "quality_score"}, Value: clause.Column{Table: "excluded", Name: "quality_score"}},
		{Column: clause.Column{Name: "embedding_model"}, Value: clause.Column{Table: "excluded", Name: "embedding_model"}},
		{Column: clause.Column{Name: "qdrant_point_id"}, Value: clause.Column{Table: "excluded", Name: "qdrant_point_id"}},
		{Column: clause.Column{Name: "source_kind"}, Value: clause.Column{Table: "excluded", Name: "source_kind"}},
		{Column: clause.Column{Name: "source_id"}, Value: clause.Column{Table: "excluded", Name: "source_id"}},
		{Column: clause.Column{Name: "effective_at"}, Value: clause.Column{Table: "excluded", Name: "effective_at"}},
		{Column: clause.Column{Name: "expires_at"}, Value: clause.Column{Table: "excluded", Name: "expires_at"}},
		{Column: clause.Column{Name: "deleted_at"}, Value: nil},
		{Column: clause.Column{Name: "updated_at"}, Value: now},
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: assignments,
		}).
		Create(normalizedDocs).Error
}

// ListDocuments 按默认规则过滤过期和已删除文档。
func (r *AIMemoryGormRepository) ListDocuments(
	ctx context.Context,
	query aidomain.MemoryDocumentQuery,
) ([]*entity.AIMemoryDocument, error) {
	if len(query.ScopeKeys) == 0 || len(query.AllowedVisibilities) == 0 {
		// 与 facts 一样，scope 和 visibility 任一缺失都不放宽查询。
		return []*entity.AIMemoryDocument{}, nil
	}
	var rows []*entity.AIMemoryDocument
	now := time.Now()
	db := r.db.WithContext(ctx).
		Model(&entity.AIMemoryDocument{}).
		Where("scope_key IN ?", query.ScopeKeys).
		Where("visibility IN ?", aidomain.NormalizeMemoryVisibilities(query.AllowedVisibilities)).
		Where("(expires_at IS NULL OR expires_at > ?)", now)
	if len(query.MemoryTypes) > 0 {
		db = db.Where("memory_type IN ?", aidomain.NormalizeMemoryTypes(query.MemoryTypes))
	}
	if query.Topic != "" {
		db = db.Where("topic = ?", query.Topic)
	}
	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	}
	if err := db.Order("updated_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDocumentsByIDs 按 document id 读取仍有效的文档。
func (r *AIMemoryGormRepository) ListDocumentsByIDs(
	ctx context.Context,
	ids []string,
) ([]*entity.AIMemoryDocument, error) {
	normalizedIDs := normalizeMemoryDocumentIDs(ids)
	if len(normalizedIDs) == 0 {
		return []*entity.AIMemoryDocument{}, nil
	}
	var rows []*entity.AIMemoryDocument
	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&entity.AIMemoryDocument{}).
		Where("id IN ?", normalizedIDs).
		Where("(expires_at IS NULL OR expires_at > ?)", now).
		Order("updated_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDocumentsNeedingIndex 扫描未索引或索引已过期的长期记忆文档。
func (r *AIMemoryGormRepository) ListDocumentsNeedingIndex(
	ctx context.Context,
	limit int,
	embedModel string,
	dimension int,
) ([]*entity.AIMemoryDocument, error) {
	embedModel = strings.TrimSpace(embedModel)
	if embedModel == "" || dimension <= 0 {
		return []*entity.AIMemoryDocument{}, nil
	}
	var rows []*entity.AIMemoryDocument
	now := time.Now()
	db := r.db.WithContext(ctx).
		Model(&entity.AIMemoryDocument{}).
		Where("content_text <> ''").
		Where("(expires_at IS NULL OR expires_at > ?)", now).
		Where(`
			NOT EXISTS (
				SELECT 1 FROM ai_memory_document_chunks c
				WHERE c.document_id = ai_memory_documents.id
			)
			OR EXISTS (
				SELECT 1 FROM ai_memory_document_chunks c
				WHERE c.document_id = ai_memory_documents.id
					AND (c.embedding_model <> ? OR c.embedding_dimension <> ?)
			)
			OR ai_memory_documents.updated_at > (
				SELECT COALESCE(MAX(c.indexed_at), '1970-01-01 00:00:00')
				FROM ai_memory_document_chunks c
				WHERE c.document_id = ai_memory_documents.id
			)
		`, embedModel, dimension)
	if limit > 0 {
		db = db.Limit(limit)
	}
	if err := db.Order("updated_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ReplaceDocumentChunks 按 document 覆盖保存最新 chunks。
func (r *AIMemoryGormRepository) ReplaceDocumentChunks(
	ctx context.Context,
	documentID string,
	chunks []*entity.AIMemoryDocumentChunk,
) error {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", documentID).Delete(&entity.AIMemoryDocumentChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}
		now := time.Now()
		normalized := make([]*entity.AIMemoryDocumentChunk, 0, len(chunks))
		for _, chunk := range chunks {
			if chunk == nil {
				continue
			}
			chunk.DocumentID = documentID
			if chunk.CreatedAt.IsZero() {
				chunk.CreatedAt = now
			}
			chunk.UpdatedAt = now
			normalized = append(normalized, chunk)
		}
		if len(normalized) == 0 {
			return nil
		}
		return tx.Create(normalized).Error
	})
}

// ListDocumentChunks 读取指定 document 的 chunks。
func (r *AIMemoryGormRepository) ListDocumentChunks(
	ctx context.Context,
	documentID string,
) ([]*entity.AIMemoryDocumentChunk, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return []*entity.AIMemoryDocumentChunk{}, nil
	}
	var rows []*entity.AIMemoryDocumentChunk
	if err := r.db.WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("chunk_index ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListDocumentChunksByPointIDs 按 Qdrant point ids 回查仍有效的 chunks。
func (r *AIMemoryGormRepository) ListDocumentChunksByPointIDs(
	ctx context.Context,
	pointIDs []string,
) ([]*entity.AIMemoryDocumentChunk, error) {
	normalizedIDs := normalizeMemoryPointIDs(pointIDs)
	if len(normalizedIDs) == 0 {
		return []*entity.AIMemoryDocumentChunk{}, nil
	}
	var rows []*entity.AIMemoryDocumentChunk
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Model(&entity.AIMemoryDocumentChunk{}).
		Joins("JOIN ai_memory_documents d ON d.id = ai_memory_document_chunks.document_id AND d.deleted_at IS NULL").
		Where("ai_memory_document_chunks.qdrant_point_id IN ?", normalizedIDs).
		Where("(d.expires_at IS NULL OR d.expires_at > ?)", now).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetConversationSummary 获取指定会话的压缩摘要。
func (r *AIMemoryGormRepository) GetConversationSummary(
	ctx context.Context,
	query aidomain.MemoryConversationSummaryQuery,
) (*entity.AIConversationSummary, error) {
	if strings.TrimSpace(query.ConversationID) == "" || query.UserID == 0 || strings.TrimSpace(query.ScopeKey) == "" {
		return nil, nil
	}
	var summary entity.AIConversationSummary
	// summary 不是历史版本集合，而是按 conversation_id 覆盖维护的当前快照。
	// 读取时必须同时校验 user_id / org_id / scope_key，避免跨主体误命中。
	db := r.db.WithContext(ctx).
		Where("conversation_id = ?", query.ConversationID).
		Where("user_id = ?", query.UserID).
		Where("scope_key = ?", query.ScopeKey)
	if query.OrgID == nil {
		db = db.Where("org_id IS NULL")
	} else {
		db = db.Where("org_id = ?", *query.OrgID)
	}
	if err := db.First(&summary).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &summary, nil
}

// UpsertConversationSummary 按主键覆盖更新会话摘要。
func (r *AIMemoryGormRepository) UpsertConversationSummary(
	ctx context.Context,
	summary *entity.AIConversationSummary,
) error {
	if summary == nil {
		return nil
	}
	now := time.Now()
	// 会话摘要始终保留“当前版本”。
	// 因此这里按 conversation_id 主键覆盖，不保留多版本并存。
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "conversation_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"user_id":                     summary.UserID,
				"org_id":                      summary.OrgID,
				"scope_key":                   summary.ScopeKey,
				"compressed_until_message_id": summary.CompressedUntilMessageID,
				"summary_text":                summary.SummaryText,
				"key_points_json":             summary.KeyPointsJSON,
				"open_loops_json":             summary.OpenLoopsJSON,
				"token_estimate":              summary.TokenEstimate,
				"updated_at":                  now,
			}),
		}).
		Create(summary).Error
}

func (r *AIMemoryGormRepository) prepareDocumentUpserts(
	ctx context.Context,
	docs []*entity.AIMemoryDocument,
) ([]*entity.AIMemoryDocument, error) {
	dedupedDocs := make([]*entity.AIMemoryDocument, 0, len(docs))
	indexByKey := make(map[string]int, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if err := normalizeMemoryDocumentMetadata(doc); err != nil {
			return nil, err
		}
		key := buildScopedDocumentDedupIdentity(doc.ScopeKey, doc.DedupKey)
		if existingIndex, ok := indexByKey[key]; ok {
			dedupedDocs[existingIndex] = doc
			continue
		}
		indexByKey[key] = len(dedupedDocs)
		dedupedDocs = append(dedupedDocs, doc)
	}

	for _, doc := range dedupedDocs {
		existingID, err := r.findExistingDocumentID(ctx, doc.ScopeKey, doc.DedupKey)
		if err != nil {
			return nil, err
		}
		if existingID != "" {
			doc.ID = existingID
		}
	}
	return dedupedDocs, nil
}

func (r *AIMemoryGormRepository) findExistingDocumentID(
	ctx context.Context,
	scopeKey string,
	dedupKey string,
) (string, error) {
	var row entity.AIMemoryDocument
	err := r.db.WithContext(ctx).
		Unscoped().
		Select("id").
		Where("scope_key = ?", scopeKey).
		Where("dedup_key = ?", dedupKey).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return row.ID, nil
}

func normalizeMemoryDocumentMetadata(doc *entity.AIMemoryDocument) error {
	if doc == nil {
		return nil
	}
	doc.ScopeKey = strings.TrimSpace(doc.ScopeKey)
	doc.ID = strings.TrimSpace(doc.ID)
	doc.Topic = strings.TrimSpace(doc.Topic)
	doc.SourceID = strings.TrimSpace(doc.SourceID)
	if doc.ScopeKey == "" {
		return fmt.Errorf("memory document scope_key is required")
	}
	if doc.ID == "" {
		return fmt.Errorf("memory document id is required")
	}

	doc.ContentHash = aidomain.BuildMemoryDocumentContentHash(doc.ContentText)
	doc.SummaryHash = aidomain.BuildMemoryDocumentSummaryHash(doc.Summary)
	doc.DedupKey = aidomain.BuildMemoryDocumentDedupKey(
		aidomain.MemorySourceKind(doc.SourceKind),
		doc.SourceID,
		doc.Topic,
		doc.Summary,
		doc.ContentText,
	)
	if doc.DedupKey == "" {
		return fmt.Errorf("memory document dedup_key is required")
	}
	return nil
}

func buildScopedDocumentDedupIdentity(scopeKey string, dedupKey string) string {
	return strings.TrimSpace(scopeKey) + "\n" + strings.TrimSpace(dedupKey)
}

func normalizeMemoryDocumentIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, id)
	}
	return items
}

func normalizeMemoryPointIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, id)
	}
	return items
}
