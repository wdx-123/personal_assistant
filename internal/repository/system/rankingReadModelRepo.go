package system

import (
	"context"

	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type rankingReadModelRepository struct {
	db *gorm.DB
}

func NewRankingReadModelRepository(db *gorm.DB) interfaces.RankingReadModelRepository {
	return &rankingReadModelRepository{db: db}
}

func (r *rankingReadModelRepository) GetByUserID(
	ctx context.Context,
	userID uint,
) (*readmodel.Ranking, error) {
	if userID == 0 {
		return nil, nil
	}

	items, err := r.GetByUserIDs(ctx, []uint{userID})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return items[0], nil
}

func (r *rankingReadModelRepository) GetByUserIDs(
	ctx context.Context,
	userIDs []uint,
) ([]*readmodel.Ranking, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	return r.query(ctx, func(db *gorm.DB) *gorm.DB {
		return db.Where("users.id IN ?", userIDs)
	})
}

// ListAll 获取所有用户的排行榜读模型数据，通常用于全量缓存重建等场景
func (r *rankingReadModelRepository) ListAll(
	ctx context.Context,
) ([]*readmodel.Ranking, error) {
	return r.query(ctx, nil)
}

// query 是一个通用的查询方法，接受一个可选的 apply 函数来定制查询条件，返回满足条件的排行榜读模型列表。
func (r *rankingReadModelRepository) query(
	ctx context.Context,
	apply func(db *gorm.DB) *gorm.DB,
) ([]*readmodel.Ranking, error) {
	query := r.db.WithContext(ctx).
		Table("users").
		Select(`
			users.id AS user_id,
			users.username,
			users.avatar,
			users.current_org_id,
			COALESCE(current_orgs.name, '') AS current_org_name,
			users.status,
			users.freeze,
			COALESCE(luogu_user_details.identification, '') AS luogu_identifier,
			COALESCE(luogu_user_details.user_avatar, '') AS luogu_avatar,
			COALESCE(luogu_user_details.passed_number, 0) AS luogu_score,
			COALESCE(leetcode_user_details.user_slug, '') AS leetcode_identifier,
			COALESCE(leetcode_user_details.user_avatar, '') AS leetcode_avatar,
			COALESCE(leetcode_user_details.total_number, 0) AS leetcode_score`).
		Joins("LEFT JOIN orgs current_orgs ON current_orgs.id = users.current_org_id").
		Joins("LEFT JOIN luogu_user_details ON luogu_user_details.user_id = users.id").
		Joins("LEFT JOIN leetcode_user_details ON leetcode_user_details.user_id = users.id").
		Where("users.deleted_at IS NULL").
		Order("users.id ASC")
	if apply != nil {
		query = apply(query)
	}

	var items []*readmodel.Ranking
	if err := query.Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
