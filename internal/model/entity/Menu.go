package entity

import "gorm.io/gorm"

// Menu 菜单表
type Menu struct {
	gorm.Model
	ParentID      uint   `json:"parent_id" gorm:"default:0;comment:父菜单ID(0表示顶级菜单)"`
	Name          string `json:"name" gorm:"size:50;not null;comment:菜单名称"`
	Code          string `json:"code" gorm:"size:50;not null;comment:权限标识"`
	Icon          string `json:"icon" gorm:"size:50;comment:菜单图标"`
	Type          int    `json:"type" gorm:"default:1;comment:菜单类型(1:目录 2:菜单 3:按钮)"`
	RouteName     string `json:"route_name" gorm:"size:100;comment:路由名称"`
	RoutePath     string `json:"route_path" gorm:"size:200;comment:路由路径"`
	RouteParam    string `json:"route_param" gorm:"size:200;comment:路由参数"`
	ComponentPath string `json:"component_path" gorm:"size:200;comment:组件路径"`
	Status        int    `json:"status" gorm:"default:1;comment:状态(1:显示 0:隐藏)"`
	Sort          int    `json:"sort" gorm:"default:0;comment:排序"`
	Desc          string `json:"desc" gorm:"size:200;comment:菜单描述"`
	APIs          []API  `json:"-" gorm:"many2many:menu_apis;"`
}
