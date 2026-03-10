package entity

import (
	"time"

	"personal_assistant/internal/model/consts"
)

// OrgMember 组织成员关系（单行关系 + 状态流转，不使用软删除语义）
type OrgMember struct {
	ID uint `json:"id" gorm:"primaryKey;comment:'主键ID'"`

	OrgID  uint `json:"org_id" gorm:"not null;index:uk_org_member,unique;index;comment:'组织ID'"`
	UserID uint `json:"user_id" gorm:"not null;index:uk_org_member,unique;index;comment:'用户ID'"`

	MemberStatus consts.OrgMemberStatus `json:"member_status" gorm:"type:tinyint;not null;default:1;index;comment:'成员状态：1 active,2 left,3 removed'"`

	JoinedAt     time.Time  `json:"joined_at" gorm:"type:datetime;not null;comment:'加入时间'"`
	LeftAt       *time.Time `json:"left_at,omitempty" gorm:"type:datetime;comment:'退出时间'"`
	RemovedAt    *time.Time `json:"removed_at,omitempty" gorm:"type:datetime;comment:'踢出时间'"`
	RemovedBy    *uint      `json:"removed_by,omitempty" gorm:"index;comment:'踢出操作者ID'"`
	RemoveReason string     `json:"remove_reason" gorm:"type:varchar(200);default:'';comment:'退出/踢出原因'"`
	JoinSource   string     `json:"join_source" gorm:"type:varchar(32);not null;default:'legacy_backfill';comment:'加入来源'"`

	CreatedAt time.Time `json:"created_at" gorm:"type:datetime;not null;comment:'创建时间'"`
	UpdatedAt time.Time `json:"updated_at" gorm:"type:datetime;not null;comment:'更新时间'"`
}
