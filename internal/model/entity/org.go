package entity

type Org struct {
	MODEL
	Name        string  `json:"name" gorm:"type:varchar(100);not null;comment:'组织名称'"`
	Description string  `json:"description" gorm:"type:varchar(255);default:'';comment:'组织描述'"`
	Code        string  `json:"code" gorm:"type:varchar(20);index;comment:'加入邀请码'"`
	Avatar      string  `json:"avatar" gorm:"type:varchar(255);default:'';comment:'组织头像URL'"`
	AvatarID    *uint   `json:"avatar_id,omitempty" gorm:"index;comment:'组织头像图片ID（可空）'"`
	OwnerID     uint    `json:"owner_id" gorm:"index;comment:'创建者/负责人ID'"`
	IsBuiltin   bool    `json:"is_builtin" gorm:"type:boolean;not null;default:false;index;comment:'是否系统内置组织'"`
	BuiltinKey  *string `json:"builtin_key,omitempty" gorm:"type:varchar(50);uniqueIndex:uk_org_builtin_key;comment:'系统内置组织标识（可空）'"`
}
