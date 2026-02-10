package entity

import "gorm.io/gorm"

// API 接口表
// 通过 uniqueIndex:idx_path_method 确保 path+method 组合唯一
// API 与菜单的关联通过 menu_apis 中间表实现（多对多），不在此处设外键
type API struct {
	gorm.Model
	// Path + Method 结合成联合唯一索引
	Path   string `json:"path" gorm:"size:200;not null;uniqueIndex:idx_path_method;comment:API路径"`
	Method string `json:"method" gorm:"size:10;not null;uniqueIndex:idx_path_method;comment:请求方法"`
	Detail string `json:"detail" gorm:"size:100;comment:API描述"`
	Status int    `json:"status" gorm:"default:1;comment:状态(1:启用 0:禁用)"`
}
