package system

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type luoguUserQuestionRepository struct {
	db *gorm.DB
}

func NewLuoguUserQuestionRepository(db *gorm.DB) interfaces.LuoguUserQuestionRepository {
	return &luoguUserQuestionRepository{db: db}
}

func (r *luoguUserQuestionRepository) GetSolvedProblemIDs(
	ctx context.Context,
	luoguUserDetailID uint,
) (map[uint]struct{}, error) {
	var records []entity.LuoguUserQuestion
	err := r.db.WithContext(ctx).
		Select("luogu_question_id").
		Where("luogu_user_detail_id = ?", luoguUserDetailID).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	idSet := make(map[uint]struct{}, len(records))
	for _, record := range records {
		idSet[record.LuoguQuestionID] = struct{}{}
	}
	return idSet, nil
}

func (r *luoguUserQuestionRepository) BatchCreate(
	ctx context.Context,
	records []*entity.LuoguUserQuestion,
) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(records, 100).Error
}

func (r *luoguUserQuestionRepository) CountSolvedByDateRange(
	ctx context.Context,
	luoguUserDetailID uint,
	start time.Time,
	end time.Time,
) ([]*readmodel.DateSolvedCount, error) {
	if luoguUserDetailID == 0 || !start.Before(end) {
		return nil, nil
	}

	var rows []*readmodel.DateSolvedCount
	err := r.db.WithContext(ctx).
		Model(&entity.LuoguUserQuestion{}).
		Select("DATE(created_at) AS stat_date, COUNT(1) AS solved_count").
		Where(
			"luogu_user_detail_id = ? AND created_at >= ? AND created_at < ?",
			luoguUserDetailID,
			start,
			end,
		).
		Group("DATE(created_at)").
		Order("stat_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
