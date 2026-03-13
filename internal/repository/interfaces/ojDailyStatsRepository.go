package interfaces

import (
	"context"
	"time"

	"personal_assistant/internal/model/entity"
)

type OJDailyStatsRepository interface {
	UpsertBatch(ctx context.Context, rows []*entity.OJUserDailyStat) error
	ListRange(
		ctx context.Context,
		userID uint,
		platform string,
		fromDate time.Time,
		toDate time.Time,
	) ([]*entity.OJUserDailyStat, error)
	// DeleteByUserPlatform 删除指定用户和平台的所有统计数据，通常用于重建最近窗口数据时清理旧数据
	DeleteByUserPlatform(ctx context.Context, userID uint, platform string) error
}
