package entity

// RoleAPI 角色与API直绑关系（用于角色直绑API权限）
type RoleAPI struct {
	RoleID uint `json:"role_id" gorm:"primaryKey;autoIncrement:false;comment:角色ID"`
	APIID  uint `json:"api_id" gorm:"primaryKey;autoIncrement:false;index:idx_role_apis_api_id;comment:API ID"`
}
