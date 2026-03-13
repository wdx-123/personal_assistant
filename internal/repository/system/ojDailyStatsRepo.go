package system

import (
	"context"
	"strings"
	"time"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ojDailyStatsRepository struct {
	db *gorm.DB
}

func NewOJDailyStatsRepository(db *gorm.DB) interfaces.OJDailyStatsRepository {
	return &ojDailyStatsRepository{db: db}
}

func (r *ojDailyStatsRepository) UpsertBatch(
	ctx context.Context,
	rows []*entity.OJUserDailyStat,
) error {
	if len(rows) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "platform"},
				{Name: "stat_date"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"solved_count",
				"solved_total",
				"source_updated_at",
				"updated_at",
			}),
		}).
		Create(&rows).Error
}

func (r *ojDailyStatsRepository) ListRange(
	ctx context.Context,
	userID uint,
	platform string,
	fromDate time.Time,
	toDate time.Time,
) ([]*entity.OJUserDailyStat, error) {
	if userID == 0 {
		return nil, nil
	}

	var rows []*entity.OJUserDailyStat
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND platform = ? AND stat_date BETWEEN ? AND ?", userID, strings.TrimSpace(platform), fromDate, toDate).
		Order("stat_date ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// DeleteByUserPlatform 删除指定用户和平台的所有统计数据，通常用于重建最近窗口数据时清理旧数据
func (r *ojDailyStatsRepository) DeleteByUserPlatform(
	ctx context.Context,
	userID uint,
	platform string,
) error {
	if userID == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, strings.TrimSpace(platform)).
		Delete(&entity.OJUserDailyStat{}).Error
}
