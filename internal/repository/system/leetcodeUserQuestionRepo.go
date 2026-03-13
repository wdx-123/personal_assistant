package system

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type leetcodeUserQuestionRepository struct {
	db *gorm.DB
}

func NewLeetcodeUserQuestionRepository(db *gorm.DB) interfaces.LeetcodeUserQuestionRepository {
	return &leetcodeUserQuestionRepository{db: db}
}

func (r *leetcodeUserQuestionRepository) GetSolvedProblemIDs(
	ctx context.Context,
	leetcodeUserDetailID uint,
) (map[uint]struct{}, error) {
	var records []entity.LeetcodeUserQuestion
	err := r.db.WithContext(ctx).
		Select("leetcode_question_id").
		Where("leetcode_user_detail_id = ?", leetcodeUserDetailID).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	idSet := make(map[uint]struct{}, len(records))
	for _, record := range records {
		idSet[record.LeetcodeQuestionID] = struct{}{}
	}
	return idSet, nil
}

func (r *leetcodeUserQuestionRepository) BatchCreate(
	ctx context.Context,
	records []*entity.LeetcodeUserQuestion,
) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(records, 100).Error
}

func (r *leetcodeUserQuestionRepository) CountSolvedByDateRange(
	ctx context.Context,
	leetcodeUserDetailID uint,
	start time.Time,
	end time.Time,
) ([]*readmodel.DateSolvedCount, error) {
	if leetcodeUserDetailID == 0 || !start.Before(end) {
		return nil, nil
	}

	var rows []*readmodel.DateSolvedCount
	err := r.db.WithContext(ctx).
		Model(&entity.LeetcodeUserQuestion{}).
		Select("DATE(created_at) AS stat_date, COUNT(1) AS solved_count").
		Where(
			"leetcode_user_detail_id = ? AND created_at >= ? AND created_at < ?",
			leetcodeUserDetailID,
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
