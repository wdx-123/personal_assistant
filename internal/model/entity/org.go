package entity

type Org struct {
	MODEL
	Name        string `json:"name" gorm:"type:varchar(100);not null;comment:'组织名称'"`
	Description string `json:"description" gorm:"type:varchar(255);default:'';comment:'组织描述'"`
	Code        string `json:"code" gorm:"type:varchar(20);index;comment:'加入邀请码'"`
	OwnerID     uint   `json:"owner_id" gorm:"index;comment:'创建者/负责人ID'"`
}
