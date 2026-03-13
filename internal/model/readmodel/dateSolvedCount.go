package readmodel

import "time"

// DateSolvedCount 承载按天聚合后的 solved_count 结果。
type DateSolvedCount struct {
	StatDate    time.Time `gorm:"column:stat_date"`
	SolvedCount int       `gorm:"column:solved_count"`
}
