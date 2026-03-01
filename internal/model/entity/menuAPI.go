package entity

// MenuAPI 菜单与API绑定关系（单API归属单菜单）
// 通过 api_id 唯一索引确保“一个API仅能归属一个菜单”。
type MenuAPI struct {
	MenuID uint `json:"menu_id" gorm:"primaryKey;autoIncrement:false;comment:菜单ID"`
	APIID  uint `json:"api_id" gorm:"primaryKey;autoIncrement:false;uniqueIndex:uk_menu_apis_api_id;comment:API ID"`
}

// TableName 指定中间表名称，与 many2many:menu_apis 对齐。
func (MenuAPI) TableName() string {
	return "menu_apis"
}
