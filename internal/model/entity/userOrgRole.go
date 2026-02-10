package entity

import "gorm.io/gorm"

// UserOrgRole 用户组织角色关联
// 多租户的核心 。以前用户和角色的关系是直接绑定的（User-Role），
// 现在变成了“用户在某个组织下是什么角色”（User-Org-Role）。
// 解决问题 ：让你能实现在 A 区是领导，在 B 区是员工。
type UserOrgRole struct {
	gorm.Model
	UserID uint `json:"user_id" gorm:"not null;index"`
	OrgID  uint `json:"org_id" gorm:"not null;index"`
	RoleID uint `json:"role_id" gorm:"not null;index"`
}
