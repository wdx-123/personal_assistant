package entity

// RoleCapability 角色与 capability 的关联关系。
type RoleCapability struct {
	RoleID       uint `json:"role_id" gorm:"primaryKey;autoIncrement:false;comment:角色ID"`
	CapabilityID uint `json:"capability_id" gorm:"primaryKey;autoIncrement:false;index:idx_role_capabilities_capability_id;comment:能力ID"`
}
