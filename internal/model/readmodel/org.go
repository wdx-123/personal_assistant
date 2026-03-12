package readmodel

import "time"

// OrgWithMemberCount 是组织列表/详情接口使用的只读聚合视图。
type OrgWithMemberCount struct {
	ID          uint
	Name        string
	Description string
	Code        string
	Avatar      string
	AvatarID    *uint
	OwnerID     uint
	MemberCount int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
