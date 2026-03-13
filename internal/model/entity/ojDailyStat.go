package entity

import "time"

// OJUserDailyStat 是按用户/平台/日期聚合后的刷题读模型表。
// 它只服务查询侧，不作为 OJ 原始事实来源。
type OJUserDailyStat struct {
	MODEL
	UserID          uint      `json:"user_id" gorm:"not null;index:idx_user_platform_date,priority:1;comment:'业务用户ID'"`
	Platform        string    `json:"platform" gorm:"type:varchar(16);not null;index:idx_user_platform_date,priority:2;comment:'OJ平台'"`
	StatDate        time.Time `json:"stat_date" gorm:"type:date;not null;index:idx_user_platform_date,priority:3;comment:'统计日期'"`
	SolvedCount     int       `json:"solved_count" gorm:"not null;default:0;comment:'当天新增做题数'"`
	SolvedTotal     int       `json:"solved_total" gorm:"not null;default:0;comment:'截止当天累计做题数'"`
	SourceUpdatedAt time.Time `json:"source_updated_at" gorm:"type:datetime;not null;comment:'本次投影使用的数据源更新时间'"`
}

func (OJUserDailyStat) TableName() string {
	return "oj_user_daily_stats"
}
