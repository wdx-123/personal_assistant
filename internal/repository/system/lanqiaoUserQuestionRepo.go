package system

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
	readmodel "personal_assistant/internal/model/readmodel"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type lanqiaoUserQuestionRepository struct {
	db *gorm.DB
}

func NewLanqiaoUserQuestionRepository(db *gorm.DB) interfaces.LanqiaoUserQuestionRepository {
	return &lanqiaoUserQuestionRepository{db: db}
}

func (r *lanqiaoUserQuestionRepository) WithTx(tx any) interfaces.LanqiaoUserQuestionRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &lanqiaoUserQuestionRepository{db: transaction}
	}
	return r
}

func (r *lanqiaoUserQuestionRepository) GetSolvedProblemIDs(
	ctx context.Context,
	lanqiaoUserDetailID uint,
) (map[uint]struct{}, error) {
	var records []entity.LanqiaoUserQuestion
	err := r.db.WithContext(ctx).
		Select("lanqiao_question_id").
		Where("lanqiao_user_detail_id = ?", lanqiaoUserDetailID).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	idSet := make(map[uint]struct{}, len(records))
	for _, record := range records {
		idSet[record.LanqiaoQuestionID] = struct{}{}
	}
	return idSet, nil
}

func (r *lanqiaoUserQuestionRepository) GetSolvedProblemIDsByDetailIDs(
	ctx context.Context,
	lanqiaoUserDetailIDs []uint,
) (map[uint]map[uint]struct{}, error) {
	result := make(map[uint]map[uint]struct{}, len(lanqiaoUserDetailIDs))
	if len(lanqiaoUserDetailIDs) == 0 {
		return result, nil
	}
	type row struct {
		LanqiaoUserDetailID uint `gorm:"column:lanqiao_user_detail_id"`
		LanqiaoQuestionID   uint `gorm:"column:lanqiao_question_id"`
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&entity.LanqiaoUserQuestion{}).
		Select("lanqiao_user_detail_id, lanqiao_question_id").
		Where("lanqiao_user_detail_id IN ?", lanqiaoUserDetailIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, ok := result[row.LanqiaoUserDetailID]; !ok {
			result[row.LanqiaoUserDetailID] = make(map[uint]struct{})
		}
		result[row.LanqiaoUserDetailID][row.LanqiaoQuestionID] = struct{}{}
	}
	return result, nil
}

func (r *lanqiaoUserQuestionRepository) BatchCreate(
	ctx context.Context,
	records []*entity.LanqiaoUserQuestion,
) error {
	if len(records) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "lanqiao_user_detail_id"}, {Name: "lanqiao_question_id"}},
			DoNothing: true,
		}).
		CreateInBatches(records, 100).Error
}

func (r *lanqiaoUserQuestionRepository) CountSolvedByDateRange(
	ctx context.Context,
	lanqiaoUserDetailID uint,
	start time.Time,
	end time.Time,
) ([]*readmodel.DateSolvedCount, error) {
	if lanqiaoUserDetailID == 0 || !start.Before(end) {
		return nil, nil
	}

	var rows []*readmodel.DateSolvedCount
	err := r.db.WithContext(ctx).
		Model(&entity.LanqiaoUserQuestion{}).
		Select("DATE(solved_at) AS stat_date, COUNT(1) AS solved_count").
		Where(
			"lanqiao_user_detail_id = ? AND solved_at >= ? AND solved_at < ?",
			lanqiaoUserDetailID,
			start,
			end,
		).
		Group("DATE(solved_at)").
		Order("stat_date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *lanqiaoUserQuestionRepository) CountPassed(
	ctx context.Context,
	lanqiaoUserDetailID uint,
) (int64, error) {
	if lanqiaoUserDetailID == 0 {
		return 0, nil
	}
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.LanqiaoUserQuestion{}).
		Where("lanqiao_user_detail_id = ?", lanqiaoUserDetailID).
		Count(&count).Error
	return count, err
}
