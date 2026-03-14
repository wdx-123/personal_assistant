package readmodel

import "personal_assistant/internal/model/consts"

// Ranking 是排行榜/详情缓存投影所需的只读聚合视图。
type Ranking struct {
	UserID         uint              `gorm:"column:user_id"`
	Username       string            `gorm:"column:username"`
	Avatar         string            `gorm:"column:avatar"`
	CurrentOrgID   *uint             `gorm:"column:current_org_id"`
	CurrentOrgName string            `gorm:"column:current_org_name"`
	Status         consts.UserStatus `gorm:"column:status"`
	Freeze         bool              `gorm:"column:freeze"`

	LuoguIdentifier string `gorm:"column:luogu_identifier"`
	LuoguAvatar     string `gorm:"column:luogu_avatar"`
	LuoguScore      int    `gorm:"column:luogu_score"`

	LeetcodeIdentifier string `gorm:"column:leetcode_identifier"`
	LeetcodeAvatar     string `gorm:"column:leetcode_avatar"`
	LeetcodeScore      int    `gorm:"column:leetcode_score"`

	LanqiaoIdentifier string `gorm:"column:lanqiao_identifier"`
	LanqiaoAvatar     string `gorm:"column:lanqiao_avatar"`
	LanqiaoScore      int    `gorm:"column:lanqiao_score"`
}

func (m *Ranking) IsActive() bool {
	return m != nil && !m.Freeze && m.Status == consts.UserStatusActive
}
