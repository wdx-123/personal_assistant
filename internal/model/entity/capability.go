package entity

// Capability 组织内业务动作能力定义。
type Capability struct {
	MODEL
	Code      string `json:"code" gorm:"type:varchar(100);not null;uniqueIndex:uk_capability_code;comment:'能力代码'"`
	Name      string `json:"name" gorm:"type:varchar(50);not null;comment:'能力名称'"`
	Domain    string `json:"domain" gorm:"type:varchar(50);not null;index:idx_capability_domain_group;comment:'所属领域'"`
	GroupCode string `json:"group_code" gorm:"type:varchar(50);not null;index:idx_capability_domain_group;comment:'能力分组代码'"`
	GroupName string `json:"group_name" gorm:"type:varchar(50);not null;comment:'能力分组名称'"`
	Desc      string `json:"desc" gorm:"type:varchar(255);default:'';comment:'能力描述'"`
	Status    int    `json:"status" gorm:"default:1;comment:'状态(1:启用 0:禁用)'"`
}
