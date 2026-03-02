package response

import "time"

// MenuItem 菜单项（列表/树/详情通用）
type MenuItem struct {
	ID            uint             `json:"id"`
	ParentID      uint             `json:"parent_id"`
	Name          string           `json:"name"`
	Code          string           `json:"code"`
	Type          int              `json:"type"` // 1:目录 2:菜单 3:按钮
	Icon          string           `json:"icon"`
	RouteName     string           `json:"route_name"`
	RoutePath     string           `json:"route_path"`
	RouteParam    string           `json:"route_param"`
	ComponentPath string           `json:"component_path"`
	Status        int              `json:"status"` // 1:显示 0:隐藏
	Sort          int              `json:"sort"`
	Desc          string           `json:"desc"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	Children      []*MenuItem      `json:"children,omitempty"` // 子菜单（树结构时使用）
	APIs          []*APIItemSimple `json:"apis,omitempty"`     // 关联的API列表
}

// APIItemSimple API简要信息（用于菜单关联展示）
type APIItemSimple struct {
	ID     uint   `json:"id"`
	Path   string `json:"path"`
	Method string `json:"method"`
}

// MenuListResp 菜单分页列表响应
type MenuListResp struct {
	List     []*MenuItem `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}
